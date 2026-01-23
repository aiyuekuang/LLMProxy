package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookProvider Webhook Provider
// 通过 HTTP 请求外部服务验证 API Key
type WebhookProvider struct {
	BaseProvider
	client  *http.Client      // HTTP 客户端
	url     string            // Webhook URL
	method  string            // HTTP 方法
	headers map[string]string // 自定义请求头
}

// NewWebhookProvider 创建 Webhook Provider
// 参数：
//   - name: Provider 名称
//   - cfg: Webhook 配置
//
// 返回：
//   - Provider: Provider 实例
//   - error: 错误信息
func NewWebhookProvider(name string, cfg *WebhookConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("webhook 配置不能为空")
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("webhook URL 不能为空")
	}

	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	return &WebhookProvider{
		BaseProvider: BaseProvider{
			name:         name,
			providerType: ProviderTypeWebhook,
		},
		client: &http.Client{
			Timeout: timeout,
		},
		url:     cfg.URL,
		method:  method,
		headers: cfg.Headers,
	}, nil
}

// WebhookRequest Webhook 请求体
type WebhookRequest struct {
	APIKey      string       `json:"api_key"`           // API Key
	Timestamp   int64        `json:"timestamp"`         // 请求时间戳
	RequestInfo *RequestInfo `json:"request,omitempty"` // 请求信息
}

// Query 调用 Webhook 验证 API Key
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
//
// 返回：
//   - *ProviderResult: 查询结果
func (w *WebhookProvider) Query(ctx context.Context, apiKey string) *ProviderResult {
	// 构建请求体
	reqBody := WebhookRequest{
		APIKey:    apiKey,
		Timestamp: time.Now().Unix(),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("请求体序列化失败: %w", err),
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, w.method, w.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("创建请求失败: %w", err),
		}
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := w.client.Do(req)
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("webhook 请求失败: %w", err),
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("读取响应失败: %w", err),
		}
	}

	// 解析响应
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("响应解析失败: %w", err),
		}
	}

	// 将 HTTP 状态码也放入数据中
	data["_http_status"] = resp.StatusCode

	return &ProviderResult{
		Found: true,
		Data:  data,
	}
}
