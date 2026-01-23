package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"llmproxy/internal/config"
)

// HTTPSource HTTP API 发现源
// 从远程 HTTP API 获取后端服务列表
type HTTPSource struct {
	BaseSource
	url        string
	method     string
	timeout    time.Duration
	headers    map[string]string
	httpClient *http.Client
}

// httpBackendResponse HTTP API 返回的后端信息
type httpBackendResponse struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Weight int    `json:"weight"`
	Status string `json:"status"`
}

// httpDiscoveryResponse HTTP API 返回的发现响应
type httpDiscoveryResponse struct {
	Backends []httpBackendResponse `json:"backends"`
	Services []httpBackendResponse `json:"services"` // 兼容不同的字段名
}

// NewHTTPSource 创建 HTTP 发现源
// 参数：
//   - name: 发现源名称
//   - cfg: HTTP 发现配置
//
// 返回：
//   - Source: 发现源实例
//   - error: 错误信息
func NewHTTPSource(name string, cfg *config.DiscoveryHTTPConfig) (Source, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("HTTP URL 为空")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	method := cfg.Method
	if method == "" {
		method = "GET"
	}

	return &HTTPSource{
		BaseSource: NewBaseSource(name, "http"),
		url:        cfg.URL,
		method:     method,
		timeout:    timeout,
		headers:    cfg.Headers,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Discover 从 HTTP API 获取后端服务列表
func (h *HTTPSource) Discover(ctx context.Context) ([]*config.Backend, error) {
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, h.method, h.url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/json")
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析 JSON
	var response httpDiscoveryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// 尝试直接解析为数组
		var backends []httpBackendResponse
		if err := json.Unmarshal(body, &backends); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}
		response.Backends = backends
	}

	// 合并 backends 和 services
	allBackends := response.Backends
	if len(response.Services) > 0 {
		allBackends = append(allBackends, response.Services...)
	}

	// 转换为 config.Backend
	var result []*config.Backend
	for _, bk := range allBackends {
		// 跳过非活跃的服务
		if bk.Status != "" && bk.Status != "enabled" && bk.Status != "active" {
			continue
		}

		weight := bk.Weight
		if weight <= 0 {
			weight = 1
		}

		result = append(result, &config.Backend{
			Name:   bk.Name,
			URL:    bk.URL,
			Weight: weight,
		})
	}

	return result, nil
}
