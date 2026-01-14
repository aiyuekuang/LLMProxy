package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/config"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
)

// RequestBody 请求体结构
type RequestBody struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Stream   bool                     `json:"stream"`
}

// NewHandler 创建代理处理器
// 参数：
//   - cfg: 配置对象
// 返回：
//   - http.HandlerFunc: HTTP 处理函数
func NewHandler(cfg *config.Config) http.HandlerFunc {
	// 创建 HTTP 客户端（复用连接）
	client := &http.Client{
		Timeout: 0, // 不设置超时，由后端控制
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// 创建负载均衡器
	loadBalancer := lb.NewRoundRobin(cfg.Backends, cfg.HealthCheck)

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 1. 仅处理 LLM API 路径
		if !isLLMEndpoint(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		// 2. 仅支持 POST 方法
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 3. 读取请求体
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("读取请求体失败: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// 4. 解析请求体，提取 stream 参数
		var reqBody RequestBody
		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			log.Printf("解析请求体失败: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// 5. 选择后端
		backend := loadBalancer.Next()
		if backend == nil {
			log.Println("没有可用的健康后端")
			http.Error(w, "No healthy backend", http.StatusServiceUnavailable)
			return
		}

		log.Printf("请求转发到后端: %s, stream=%v, model=%s", backend.URL, reqBody.Stream, reqBody.Model)

		// 6. 构造代理请求
		proxyReq, err := http.NewRequest("POST", backend.URL+r.URL.Path, bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("创建代理请求失败: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// 复制请求头
		proxyReq.Header = r.Header.Clone()

		// 7. 发送请求到后端
		resp, err := client.Do(proxyReq)
		if err != nil {
			log.Printf("后端请求失败: %v", err)
			http.Error(w, "Backend error", http.StatusBadGateway)
			metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// 8. 读取响应体（用于用量收集）
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("读取响应体失败: %v", err)
			http.Error(w, "Backend error", http.StatusBadGateway)
			metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
			return
		}

		// 9. 【关键】透传响应到客户端
		if reqBody.Stream {
			// 流式响应
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
		} else {
			// 非流式响应
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
		}

		// 10. 记录请求指标
		latency := float64(time.Since(start).Milliseconds())
		metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, latency, resp.StatusCode)

		log.Printf("请求完成: status=%d, latency=%dms", resp.StatusCode, latency)

		// 11. 【异步】触发用量上报
		go func() {
			usage := collectUsage(bodyBytes, respBody, reqBody.Stream, backend.URL, r.URL.Path)
			if usage != nil {
				// 记录 Token 使用量指标
				metrics.RecordUsage(usage.PromptTokens, usage.CompletionTokens)
				// 发送 Webhook
				SendUsageWebhook(cfg.UsageHook, usage)
			}
		}()
	}
}

// isLLMEndpoint 判断是否为 LLM API 端点
// 参数：
//   - path: 请求路径
// 返回：
//   - bool: 是否为 LLM 端点
func isLLMEndpoint(path string) bool {
	return path == "/v1/chat/completions" || path == "/v1/completions"
}
