package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"llmproxy/internal/auth"
	"llmproxy/internal/config"
	"llmproxy/internal/hooks"
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

// HandlerOptions 处理器选项
type HandlerOptions struct {
	Config       *config.Config
	LoadBalancer lb.LoadBalancer
	Router       *routing.Router
	KeyStore     auth.KeyStore
	Limiter      ratelimit.RateLimiter
	Logger       *Logger
	Hooks        *hooks.Executor
}

// NewHandler 创建代理处理器
// 参数：
//   - cfg: 配置对象
//   - loadBalancer: 负载均衡器
//   - router: 智能路由器（可选）
//   - keyStore: Key 存储（可选）
//   - limiter: 限流器（可选）
//
// 返回：
//   - http.HandlerFunc: HTTP 处理函数
func NewHandler(cfg *config.Config, loadBalancer lb.LoadBalancer, router *routing.Router, keyStore auth.KeyStore, limiter ratelimit.RateLimiter) http.HandlerFunc {
	return NewHandlerWithOptions(&HandlerOptions{
		Config:       cfg,
		LoadBalancer: loadBalancer,
		Router:       router,
		KeyStore:     keyStore,
		Limiter:      limiter,
	})
}

// NewHandlerWithOptions 使用完整选项创建代理处理器
func NewHandlerWithOptions(opts *HandlerOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := generateRequestID()
		clientIP := ExtractClientIP(r)

		// 提取 API Key 和 User ID（用于日志和钩子）
		apiKey := extractAPIKey(r)
		var userID string
		if opts.KeyStore != nil && apiKey != "" {
			if key, err := opts.KeyStore.Get(apiKey); err == nil {
				userID = key.UserID
			}
		}

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
		defer func() {
			_ = r.Body.Close()
		}()

		// 4. 解析请求体，仅提取 stream 参数
		var reqBody RequestBody
		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			log.Printf("解析请求体失败: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// 4.1 执行 on_request 钩子
		if opts.Hooks != nil {
			hookCtx := &hooks.HookContext{
				Request:   hooks.ExtractRequestInfo(r, bodyBytes, clientIP, apiKey, userID),
				Metadata:  make(map[string]interface{}),
				Timestamp: start,
			}
			result := opts.Hooks.ExecuteOnRequest(hookCtx)
			if !result.Continue {
				log.Printf("on_request 钩子拒绝请求: %s", result.Error)
				http.Error(w, result.Error, http.StatusForbidden)
				return
			}
		}

		// 5. 选择后端并发送请求
		var resp *http.Response
		var backend *lb.Backend

		if opts.Router != nil {
			// 使用智能路由（带重试和故障转移）
			resp, backend, err = opts.Router.ProxyRequest(r, bodyBytes, "")
		} else {
			// 使用简单负载均衡
			backend = opts.LoadBalancer.Next()
			if backend == nil {
				log.Println("没有可用的健康后端")
				// 执行 on_error 钩子
				if opts.Hooks != nil {
					hookCtx := &hooks.HookContext{
						Request:   hooks.ExtractRequestInfo(r, bodyBytes, clientIP, apiKey, userID),
						Error:     err,
						Metadata:  make(map[string]interface{}),
						Timestamp: start,
					}
					opts.Hooks.ExecuteOnError(hookCtx)
				}
				http.Error(w, "No healthy backend", http.StatusServiceUnavailable)
				return
			}
			resp, err = sendRequest(r, backend, bodyBytes)
		}

		if err != nil {
			log.Printf("后端请求失败: %v", err)
			// 执行 on_error 钩子
			if opts.Hooks != nil {
				hookCtx := &hooks.HookContext{
					Request:   hooks.ExtractRequestInfo(r, bodyBytes, clientIP, apiKey, userID),
					Error:     err,
					Metadata:  make(map[string]interface{}),
					Timestamp: start,
				}
				opts.Hooks.ExecuteOnError(hookCtx)
			}
			http.Error(w, "Backend error", http.StatusBadGateway)
			if backend != nil {
				metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
			}
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		log.Printf("请求转发到后端: %s, stream=%v", backend.URL, reqBody.Stream)

		// 6. 处理响应
		var respBody []byte

		if reqBody.Stream {
			// 流式响应：逐块转发，实现真正的 SSE 流式传输
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲
			w.WriteHeader(resp.StatusCode)

			// 获取 Flusher 接口，用于立即刷新数据到客户端
			flusher, ok := w.(http.Flusher)
			if !ok {
				log.Println("ResponseWriter 不支持 Flusher")
			}

			// 使用缓冲区收集完整响应（用于后续用量统计）
			var buffer bytes.Buffer
			buf := make([]byte, 4096)

			for {
				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					// 写入客户端
					if _, err := w.Write(buf[:n]); err != nil {
						log.Printf("写入客户端失败: %v", err)
						break
					}
					if flusher != nil {
						flusher.Flush() // 立即刷新到客户端
					}
					// 同时收集到缓冲区
					_, _ = buffer.Write(buf[:n]) // buffer.Write 不会返回错误
				}
				if readErr != nil {
					if readErr != io.EOF {
						log.Printf("读取流式响应失败: %v", readErr)
					}
					break
				}
			}
			respBody = buffer.Bytes()
		} else {
			// 非流式响应：读取完整响应后返回
			respBody, err = io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("读取响应体失败: %v", err)
				http.Error(w, "Backend error", http.StatusBadGateway)
				metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			if _, err := w.Write(respBody); err != nil {
				log.Printf("写入响应失败: %v", err)
			}
		}

		// 7. 执行 on_response 钩子
		if opts.Hooks != nil {
			respHeaders := make(map[string]string)
			for k, v := range resp.Header {
				if len(v) > 0 {
					respHeaders[k] = v[0]
				}
			}
			hookCtx := &hooks.HookContext{
				Request: hooks.ExtractRequestInfo(r, bodyBytes, clientIP, apiKey, userID),
				Response: &hooks.ResponseInfo{
					StatusCode: resp.StatusCode,
					Headers:    respHeaders,
					Body:       respBody,
					LatencyMs:  time.Since(start).Milliseconds(),
					BackendURL: backend.URL,
				},
				Metadata:  make(map[string]interface{}),
				Timestamp: start,
			}
			opts.Hooks.ExecuteOnResponse(hookCtx)
		}

		// 8. 记录请求指标
		latency := float64(time.Since(start).Milliseconds())
		metrics.RecordRequest(r.URL.Path, reqBody.Stream, backend.URL, latency, resp.StatusCode)

		log.Printf("请求完成: status=%d, latency=%dms", resp.StatusCode, int(latency))

		// 9. 异步触发用量上报、日志记录和 on_complete 钩子
		go func() {
			usage := collectUsage(bodyBytes, respBody, reqBody.Stream, backend.URL, r.URL.Path, resp.StatusCode, int64(latency))
			if usage != nil {
				// 添加用户信息
				usage.UserID = userID
				usage.APIKey = apiKey

				// 记录 Token 使用量指标（如果有 usage 信息）
				if usage.Usage != nil {
					metrics.RecordUsage(usage.Usage.PromptTokens, usage.Usage.CompletionTokens)

					// 扣减额度（如果启用鉴权）
					if opts.KeyStore != nil && usage.APIKey != "" {
						totalTokens := int64(usage.Usage.PromptTokens + usage.Usage.CompletionTokens)
						if err := opts.KeyStore.IncrementUsedQuota(usage.APIKey, totalTokens); err != nil {
							log.Printf("扣减额度失败: %v", err)
						}
					}
				}

				// 发送用量数据（Webhook 或数据库）
				SendUsage(opts.Config.Usage, usage)
			}

			// 记录请求日志
			if opts.Logger != nil {
				model := ""
				if usage != nil && usage.RequestBody != nil {
					if m, ok := usage.RequestBody["model"].(string); ok {
						model = m
					}
				}
				reqLog := &RequestLog{
					RequestID:    requestID,
					Timestamp:    start,
					ClientIP:     clientIP,
					Method:       r.Method,
					Path:         r.URL.Path,
					Headers:      ExtractHeaders(r),
					RequestBody:  string(bodyBytes),
					ResponseBody: string(respBody),
					StatusCode:   resp.StatusCode,
					LatencyMs:    int64(latency),
					BackendURL:   backend.URL,
					APIKey:       apiKey,
					UserID:       userID,
					Model:        model,
					IsStream:     reqBody.Stream,
				}
				opts.Logger.LogRequest(reqLog)
			}

			// 执行 on_complete 钩子
			if opts.Hooks != nil {
				respHeaders := make(map[string]string)
				for k, v := range resp.Header {
					if len(v) > 0 {
						respHeaders[k] = v[0]
					}
				}
				hookCtx := &hooks.HookContext{
					Request: hooks.ExtractRequestInfo(r, bodyBytes, clientIP, apiKey, userID),
					Response: &hooks.ResponseInfo{
						StatusCode: resp.StatusCode,
						Headers:    respHeaders,
						Body:       respBody,
						LatencyMs:  int64(latency),
						BackendURL: backend.URL,
					},
					Metadata:  make(map[string]interface{}),
					Timestamp: start,
				}
				opts.Hooks.ExecuteOnComplete(hookCtx)
			}
		}()
	}
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// sendRequest 发送请求到后端
// 参数：
//   - r: 原始请求
//   - backend: 后端实例
//   - bodyBytes: 请求体
//
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
//
// 返回：
//   - string: API Key
func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" && len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return r.Header.Get("X-API-Key")
}

// isLLMEndpoint 判断是否为 LLM API 端点
// 参数：
//   - path: 请求路径
//
// 返回：
//   - bool: 是否为 LLM 端点
func isLLMEndpoint(path string) bool {
	return path == "/v1/chat/completions" || path == "/v1/completions"
}
