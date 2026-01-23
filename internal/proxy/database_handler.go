package proxy

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/auth"
	"llmproxy/internal/config"
	"llmproxy/internal/database"
	"llmproxy/internal/lb"
	"llmproxy/internal/metrics"
	"llmproxy/internal/ratelimit"
	"llmproxy/internal/routing"
)

// ModelRequest 用于提取模型名称
type ModelRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// NewDatabaseHandler 创建支持数据库集成的代理处理器
func NewDatabaseHandler(
	cfg *config.Config,
	loadBalancer lb.LoadBalancer,
	router *routing.Router,
	keyStore auth.KeyStore,
	limiter ratelimit.RateLimiter,
	dbStore *database.Store,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		if !isLLMEndpoint(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("读取请求体失败: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer func() {
			_ = r.Body.Close()
		}()

		var modelReq ModelRequest
		if err := json.Unmarshal(bodyBytes, &modelReq); err != nil {
			log.Printf("解析请求体失败: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// 选择后端并发送请求
		model := modelReq.Model
		var resp *http.Response
		var backend *lb.Backend

		if router != nil {
			resp, backend, err = router.ProxyRequest(r, bodyBytes, model)
		} else {
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
				metrics.RecordRequest(r.URL.Path, modelReq.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
			}
			// 记录失败日志
			if dbStore != nil {
				go logRequestToDatabase(dbStore, r, backend, model, 0, 0, int(time.Since(start).Milliseconds()), http.StatusBadGateway, modelReq.Stream, err.Error())
			}
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		log.Printf("请求转发到后端: %s, model=%s, stream=%v", backend.URL, model, modelReq.Stream)

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("读取响应体失败: %v", err)
			http.Error(w, "Backend error", http.StatusBadGateway)
			metrics.RecordRequest(r.URL.Path, modelReq.Stream, backend.URL, float64(time.Since(start).Milliseconds()), http.StatusBadGateway)
			return
		}

		if modelReq.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(resp.StatusCode)
			if _, err := w.Write(respBody); err != nil {
				log.Printf("写入流式响应失败: %v", err)
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			if _, err := w.Write(respBody); err != nil {
				log.Printf("写入响应失败: %v", err)
			}
		}

		latency := float64(time.Since(start).Milliseconds())
		metrics.RecordRequest(r.URL.Path, modelReq.Stream, backend.URL, latency, resp.StatusCode)

		log.Printf("请求完成: status=%d, latency=%dms", resp.StatusCode, int(latency))

		// 异步处理用量上报和日志记录
		go func() {
			usage := collectUsage(bodyBytes, respBody, modelReq.Stream, backend.URL, r.URL.Path, resp.StatusCode, int64(latency))
			if usage != nil {
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

				if usage.Usage != nil {
					metrics.RecordUsage(usage.Usage.PromptTokens, usage.Usage.CompletionTokens)

					if keyStore != nil && usage.APIKey != "" {
						totalTokens := int64(usage.Usage.PromptTokens + usage.Usage.CompletionTokens)
						if err := keyStore.IncrementUsedQuota(usage.APIKey, totalTokens); err != nil {
							log.Printf("扣减额度失败: %v", err)
						}
					}
				}

				SendUsage(cfg.Usage, usage)

				// 记录到数据库日志
				if dbStore != nil {
					promptTokens := 0
					completionTokens := 0
					if usage.Usage != nil {
						promptTokens = usage.Usage.PromptTokens
						completionTokens = usage.Usage.CompletionTokens
					}
					logRequestToDatabase(dbStore, r, backend, model, promptTokens, completionTokens, int(latency), resp.StatusCode, modelReq.Stream, "")
				}
			}
		}()
	}
}

// logRequestToDatabase 记录请求到数据库日志
func logRequestToDatabase(
	store *database.Store,
	r *http.Request,
	backend *lb.Backend,
	model string,
	promptTokens, completionTokens int,
	duration, status int,
	isStream bool,
	errorMsg string,
) {
	if store == nil {
		return
	}

	serviceID := uint(0)
	serviceName := ""
	if backend != nil {
		if svc := store.GetServiceByURL(backend.URL); svc != nil {
			serviceID = svc.ID
			serviceName = svc.Name
		}
	}

	reqLog := &database.RequestLog{
		ChannelID:        serviceID,
		ChannelName:      serviceName,
		Model:            model,
		RequestModel:     model,
		ActualModel:      model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		Duration:         duration,
		Status:           status,
		Endpoint:         r.URL.Path,
		ClientIP:         r.RemoteAddr,
		ErrorMessage:     errorMsg,
		IsStream:         isStream,
	}

	if err := store.LogRequest(reqLog); err != nil {
		log.Printf("记录请求日志失败: %v", err)
	}
}
