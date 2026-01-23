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

// ConsulSource Consul 服务发现源
// 通过 Consul HTTP API 获取服务列表
type ConsulSource struct {
	BaseSource
	addr       string
	service    string
	tag        string
	httpClient *http.Client
}

// consulServiceEntry Consul 服务条目
type consulServiceEntry struct {
	Service consulService `json:"Service"`
}

// consulService Consul 服务信息
type consulService struct {
	ID      string   `json:"ID"`
	Service string   `json:"Service"`
	Address string   `json:"Address"`
	Port    int      `json:"Port"`
	Tags    []string `json:"Tags"`
	Meta    map[string]string `json:"Meta"`
}

// NewConsulSource 创建 Consul 发现源
// 参数：
//   - name: 发现源名称
//   - cfg: Consul 发现配置
//
// 返回：
//   - Source: 发现源实例
//   - error: 错误信息
func NewConsulSource(name string, cfg *config.DiscoveryConsulConfig) (Source, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("consul 地址为空")
	}
	if cfg.Service == "" {
		return nil, fmt.Errorf("consul 服务名为空")
	}

	return &ConsulSource{
		BaseSource: NewBaseSource(name, "consul"),
		addr:       cfg.Addr,
		service:    cfg.Service,
		tag:        cfg.Tag,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Discover 从 Consul 获取服务列表
func (c *ConsulSource) Discover(ctx context.Context) ([]*config.Backend, error) {
	// 构建 Consul API URL
	url := fmt.Sprintf("%s/v1/health/service/%s?passing=true", c.addr, c.service)
	if c.tag != "" {
		url += "&tag=" + c.tag
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Consul 失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("consul 返回状态码: %d", resp.StatusCode)
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析 JSON
	var entries []consulServiceEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为 config.Backend
	var backends []*config.Backend
	for _, entry := range entries {
		svc := entry.Service
		
		// 构建服务 URL
		address := svc.Address
		if address == "" {
			continue
		}
		
		url := fmt.Sprintf("http://%s:%d", address, svc.Port)
		
		// 从 Meta 中读取权重
		weight := 1
		if w, ok := svc.Meta["weight"]; ok {
			_, _ = fmt.Sscanf(w, "%d", &weight)
		}

		backends = append(backends, &config.Backend{
			Name:   svc.ID,
			URL:    url,
			Weight: weight,
		})
	}

	return backends, nil
}
