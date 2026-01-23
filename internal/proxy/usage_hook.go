package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/config"
	"llmproxy/internal/metrics"
)

// 全局 Webhook HTTP 客户端（复用连接池）
var webhookClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
	},
}

// UsageRecord 用量记录
type UsageRecord struct {
	RequestID string    `json:"request_id"`        // 请求 ID
	Timestamp time.Time `json:"timestamp"`         // 时间戳
	UserID    string    `json:"user_id,omitempty"` // 用户 ID
	APIKey    string    `json:"api_key,omitempty"` // API Key

	// 完整的请求参数（不解析，完整透传）
	RequestBody map[string]interface{} `json:"request_body"` // 用户的完整请求体

	// 用量信息（从响应中提取）
	Usage *UsageInfo `json:"usage,omitempty"` // 用量信息

	// 元数据
	Method     string `json:"method"`      // HTTP 方法
	Path       string `json:"path"`        // 请求路径
	BackendURL string `json:"backend_url"` // 后端 URL
	StatusCode int    `json:"status_code"` // 响应状态码
	LatencyMs  int64  `json:"latency_ms"`  // 延迟（毫秒）
}

// UsageInfo 用量信息
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`     // 输入 token 数
	CompletionTokens int `json:"completion_tokens"` // 输出 token 数
	TotalTokens      int `json:"total_tokens"`      // 总 token 数
}

// OpenAIResponse OpenAI 标准响应格式
type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// collectUsage 从响应中收集用量信息
// 参数：
//   - reqBody: 请求体（完整保存）
//   - respBody: 响应体
//   - isStream: 是否为流式请求
//   - backendURL: 后端 URL
//   - endpoint: 请求端点
//   - statusCode: 响应状态码
//   - latencyMs: 请求延迟（毫秒）
//
// 返回：
//   - *UsageRecord: 用量记录，如果无法提取则返回 nil
func collectUsage(reqBody []byte, respBody []byte, isStream bool, backendURL, endpoint string, statusCode int, latencyMs int64) *UsageRecord {
	// 解析完整的请求体
	var requestBodyMap map[string]interface{}
	if err := json.Unmarshal(reqBody, &requestBodyMap); err != nil {
		log.Printf("解析请求体失败: %v", err)
		requestBodyMap = make(map[string]interface{})
	}

	// 提取用量信息
	var usage *UsageInfo
	var requestID string

	// 非流式请求：直接解析完整响应
	if !isStream {
		var resp OpenAIResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			log.Printf("解析响应失败: %v", err)
		} else {
			requestID = resp.ID
			// 检查是否包含 usage 信息
			if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
				usage = &UsageInfo{
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      resp.Usage.TotalTokens,
				}
			}
		}
	} else {
		// 流式请求：解析 SSE 流中的最后一个 data 块
		lines := bytes.Split(respBody, []byte("\n"))
		var lastData []byte

		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if bytes.HasPrefix(line, []byte("data: ")) {
				data := bytes.TrimPrefix(line, []byte("data: "))
				if !bytes.Equal(data, []byte("[DONE]")) {
					lastData = data
				}
			}
		}

		if len(lastData) > 0 {
			var chunk struct {
				ID    string `json:"id"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}

			if err := json.Unmarshal(lastData, &chunk); err != nil {
				log.Printf("解析流式数据失败: %v", err)
			} else {
				requestID = chunk.ID
				if chunk.Usage != nil && (chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0) {
					usage = &UsageInfo{
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
						TotalTokens:      chunk.Usage.TotalTokens,
					}
				}
			}
		}
	}

	// 构造用量记录
	return &UsageRecord{
		RequestID:   requestID,
		Timestamp:   time.Now(),
		RequestBody: requestBodyMap,
		Usage:       usage,
		Method:      "POST",
		Path:        endpoint,
		BackendURL:  backendURL,
		StatusCode:  statusCode,
		LatencyMs:   latencyMs,
	}
}

// SendUsage 发送用量数据到所有配置的上报器
// 参数：
//   - cfg: 用量上报配置
//   - usage: 用量记录
func SendUsage(cfg *config.UsageConfig, usage *UsageRecord) {
	if cfg == nil || !cfg.Enabled || usage == nil {
		return
	}

	// 遍历所有上报器
	for _, reporter := range cfg.Reporters {
		if reporter == nil || !reporter.Enabled {
			continue
		}

		switch reporter.Type {
		case "database":
			SendUsageToDatabaseByName(reporter.Name, usage)
		case "webhook":
			sendUsageToWebhook(reporter, usage)
		case "builtin":
			SendUsageToBuiltin(usage)
		default:
			log.Printf("未知的用量上报类型: %s", reporter.Type)
		}
	}
}

// sendUsageToWebhook 发送用量数据到 Webhook
// 参数：
//   - reporter: 上报器配置
//   - usage: 用量记录
func sendUsageToWebhook(reporter *config.UsageReporter, usage *UsageRecord) {
	if reporter == nil || !reporter.Enabled || reporter.Webhook == nil || usage == nil {
		return
	}

	webhook := reporter.Webhook

	// 序列化用量数据
	data, err := json.Marshal(usage)
	if err != nil {
		log.Printf("序列化用量数据失败: %v", err)
		metrics.RecordWebhookFailure()
		return
	}

	// 发送 Webhook（支持重试）
	maxRetries := webhook.Retry
	if maxRetries <= 0 {
		maxRetries = 1
	}

	timeout := webhook.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[%s] Webhook 重试 %d/%d", reporter.Name, attempt+1, maxRetries)
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}

		if sendWebhookOnce(webhook.URL, timeout, data) {
			metrics.RecordWebhookSuccess()
			return
		}
	}

	log.Printf("[%s] Webhook 发送失败，已重试 %d 次", reporter.Name, maxRetries)
	metrics.RecordWebhookFailure()
}

// sendWebhookOnce 发送一次 Webhook 请求
// 参数：
//   - url: Webhook URL
//   - timeout: 超时时间
//   - data: 请求体数据
//
// 返回：
//   - bool: 是否成功
func sendWebhookOnce(url string, timeout time.Duration, data []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		log.Printf("创建 Webhook 请求失败: %v", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookClient.Do(req)
	if err != nil {
		log.Printf("Webhook 请求失败: %v", err)
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 读取响应体（用于日志）
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Webhook 返回错误状态码 %d: %s", resp.StatusCode, string(body))
		return false
	}

	return true
}
