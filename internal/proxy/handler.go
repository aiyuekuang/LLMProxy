package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/auth"
	"llmproxy/internal/config"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
	"llmproxy/internal/ratelimit"
	"llmproxy/internal/routing"
)

// 全局 HTTP 客户端（复用连接池）
var proxyClient = &http.Client{
	Timeout: 0, // 流式响应不设超时
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// RequestBody 请求体结构（仅用于提取 stream 参数）
type RequestBody struct {
	Stream bool `json:"stream"`
}

// NewHandler 创建代理处理器
// 参数：
//   - cfg: 配置对象
//   - loadBalancer: 负载均衡器
//   - router: 智能路由器（可选）
//   - keyStore: Key 存储（可选）
//   - limiter: 限流器（可选）
// 返回：
//   - http.HandlerFunc: HTTP 处理函数
func NewHandler(cfg *config.Config, loadBalancer lb.LoadBalancer, router *routing.Router, keyStore auth.KeyStore, limiter ratelimit.RateLimiter) http.HandlerFunc {
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

		// 4. 解析请求体，仅提取 stream 参数
		var reqBody RequestBody
		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			log.Printf("解析请求体失败: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// 5. 选择后端并发送请求
		var resp *http.Response
		var backend *lb.Backend
		
		if router != nil {
			// 使用智能路由（带重试和故障转移）
			resp, backend, err = router.ProxyRequest(r, bodyBytes, "")
		} else {
			// 使用简单负载均衡
			backend = loadBalancer.Next()
			if backend == nil {
				log.Println("没有可用的健康后端")
				http.Error(w, "No healthy backend", http.StatusServiceUnavailable)
				return
			}
			resp, err = sendRequest(r, backend, bodyBytes)
		}

		if err != nil {
			log.Printf("后端请求失败: %v", err)
			http.Error(w, "Backend error", http.StatusBadGateway)
			if backend != nil {
				metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
			}
			return
		}
		defer resp.Body.Close()

		log.Printf("请求转发到后端: %s, stream=%v", backend.URL, reqBody.Stream)

		// 6. 读取响应体（用于用量收集）
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("读取响应体失败: %v", err)
			http.Error(w, "Backend error", http.StatusBadGateway)
			metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
			return
		}

		// 7. 透传响应到客户端
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

		// 8. 记录请求指标
		latency := float64(time.Since(start).Milliseconds())
		metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, latency, resp.StatusCode)

		log.Printf("请求完成: status=%d, latency=%dms", resp.StatusCode, int(latency))

		// 9. 异步触发用量上报和额度扣减
		go func() {
			usage := collectUsage(bodyBytes, respBody, reqBody.Stream, backend.URL, r.URL.Path, resp.StatusCode, int64(latency))
			if usage != nil {
				// 添加用户信息
				if keyStore != nil {
					apiKeyStr := extractAPIKey(r)
					if apiKeyStr != "" {
						key, err := keyStore.Get(apiKeyStr)
						if err == nil {
							usage.UserID = key.UserID
							usage.APIKey = apiKeyStr
						}
					}
				}
				
				// 记录 Token 使用量指标（如果有 usage 信息）
				if usage.Usage != nil {
					metrics.RecordUsage(usage.Usage.PromptTokens, usage.Usage.CompletionTokens)
					
					// 扣减额度（如果启用鉴权）
					if keyStore != nil && usage.APIKey != "" {
						totalTokens := int64(usage.Usage.PromptTokens + usage.Usage.CompletionTokens)
						keyStore.IncrementUsedQuota(usage.APIKey, totalTokens)
					}
				}
				
				// 发送用量数据（Webhook 或数据库）
				SendUsage(cfg.Usage, usage)
			}
		}()
	}
}

// sendRequest 发送请求到后端
// 参数：
//   - r: 原始请求
//   - backend: 后端实例
//   - bodyBytes: 请求体
// 返回：
//   - *http.Response: 响应
//   - error: 错误信息
func sendRequest(r *http.Request, backend *lb.Backend, bodyBytes []byte) (*http.Response, error) {
	proxyReq, err := http.NewRequest("POST", backend.URL+r.URL.Path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	proxyReq.Header = r.Header.Clone()
	return proxyClient.Do(proxyReq)
}

// extractAPIKey 从请求中提取 API Key
// 参数：
//   - r: HTTP 请求
// 返回：
//   - string: API Key
func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" && len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return r.Header.Get("X-API-Key")
}

// maskKey 脱敏 API Key
// 参数：
//   - key: API Key
// 返回：
//   - string: 脱敏后的 Key
func maskKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8] + "..."
}

// isLLMEndpoint 判断是否为 LLM API 端点
// 参数：
//   - path: 请求路径
// 返回：
//   - bool: 是否为 LLM 端点
func isLLMEndpoint(path string) bool {
	return path == "/v1/chat/completions" || path == "/v1/completions"
}
