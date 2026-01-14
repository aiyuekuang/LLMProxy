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

// UsageRecord 用量记录
type UsageRecord struct {
	RequestID        string    `json:"request_id"`         // 请求 ID
	UserID           string    `json:"user_id,omitempty"`  // 用户 ID
	APIKey           string    `json:"api_key,omitempty"`  // API Key
	Model            string    `json:"model"`              // 模型名称
	PromptTokens     int       `json:"prompt_tokens"`      // 输入 token 数
	CompletionTokens int       `json:"completion_tokens"`  // 输出 token 数
	TotalTokens      int       `json:"total_tokens"`       // 总 token 数
	IsStream         bool      `json:"is_stream"`          // 是否为流式请求
	Endpoint         string    `json:"endpoint"`           // 请求端点
	Timestamp        time.Time `json:"timestamp"`          // 时间戳
	BackendURL       string    `json:"backend_url"`        // 后端 URL
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
//   - reqBody: 请求体（用于提取 model 等信息）
//   - respBody: 响应体
//   - isStream: 是否为流式请求
//   - backendURL: 后端 URL
//   - endpoint: 请求端点
// 返回：
//   - *UsageRecord: 用量记录，如果无法提取则返回 nil
func collectUsage(reqBody []byte, respBody []byte, isStream bool, backendURL, endpoint string) *UsageRecord {
	// 解析请求体获取 model
	var req struct {
		Model string `json:"model"`
	}
	json.Unmarshal(reqBody, &req)

	// 非流式请求：直接解析完整响应
	if !isStream {
		var resp OpenAIResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			log.Printf("解析响应失败: %v", err)
			return nil
		}

		// 检查是否包含 usage 信息
		if resp.Usage.PromptTokens == 0 && resp.Usage.CompletionTokens == 0 {
			log.Println("响应中不包含 usage 信息，跳过计量")
			return nil
		}

		return &UsageRecord{
			RequestID:        resp.ID,
			Model:            req.Model,
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
			IsStream:         false,
			Endpoint:         endpoint,
			Timestamp:        time.Now(),
			BackendURL:       backendURL,
		}
	}

	// 流式请求：解析 SSE 流中的最后一个 data 块
	// SSE 格式：data: {...}\n\ndata: [DONE]\n\n
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

	if len(lastData) == 0 {
		log.Println("流式响应中未找到有效数据")
		return nil
	}

	// 解析最后一个 data 块
	var chunk struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Usage   *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(lastData, &chunk); err != nil {
		log.Printf("解析流式数据失败: %v", err)
		return nil
	}

	// 检查是否包含 usage 信息
	if chunk.Usage == nil || (chunk.Usage.PromptTokens == 0 && chunk.Usage.CompletionTokens == 0) {
		log.Println("流式响应中不包含 usage 信息，跳过计量")
		return nil
	}

	return &UsageRecord{
		RequestID:        chunk.ID,
		Model:            req.Model,
		PromptTokens:     chunk.Usage.PromptTokens,
		CompletionTokens: chunk.Usage.CompletionTokens,
		TotalTokens:      chunk.Usage.TotalTokens,
		IsStream:         true,
		Endpoint:         endpoint,
		Timestamp:        time.Now(),
		BackendURL:       backendURL,
	}
}

// SendUsageWebhook 发送用量数据到 Webhook
// 参数：
//   - hook: Webhook 配置
//   - usage: 用量记录
func SendUsageWebhook(hook *config.UsageHook, usage *UsageRecord) {
	if hook == nil || !hook.Enabled {
		return
	}

	if usage == nil {
		return
	}

	// 序列化用量数据
	data, err := json.Marshal(usage)
	if err != nil {
		log.Printf("序列化用量数据失败: %v", err)
		metrics.RecordWebhookFailure()
		return
	}

	// 发送 Webhook（支持重试）
	maxRetries := hook.Retry
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Webhook 重试 %d/%d", attempt+1, maxRetries)
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond) // 指数退避
		}

		if sendWebhookOnce(hook, data) {
			metrics.RecordWebhookSuccess()
			return
		}
	}

	// 所有重试都失败
	log.Printf("Webhook 发送失败，已重试 %d 次", maxRetries)
	metrics.RecordWebhookFailure()
}

// sendWebhookOnce 发送一次 Webhook 请求
// 参数：
//   - hook: Webhook 配置
//   - data: 请求体数据
// 返回：
//   - bool: 是否成功
func sendWebhookOnce(hook *config.UsageHook, data []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), hook.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", hook.URL, bytes.NewReader(data))
	if err != nil {
		log.Printf("创建 Webhook 请求失败: %v", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: hook.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Webhook 请求失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	// 读取响应体（用于日志）
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Webhook 返回错误状态码 %d: %s", resp.StatusCode, string(body))
		return false
	}

	log.Printf("Webhook 发送成功: %s", string(data))
	return true
}
