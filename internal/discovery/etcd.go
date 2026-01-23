package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llmproxy/internal/config"
)

// EtcdSource Etcd 服务发现源
// 通过 Etcd HTTP API (v3) 获取服务列表
type EtcdSource struct {
	BaseSource
	endpoints  []string
	prefix     string
	username   string
	password   string
	httpClient *http.Client
}

// etcdRangeResponse Etcd Range 响应
type etcdRangeResponse struct {
	Kvs []etcdKeyValue `json:"kvs"`
}

// etcdKeyValue Etcd KV
type etcdKeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// etcdServiceValue Etcd 中存储的服务值
type etcdServiceValue struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Weight int    `json:"weight"`
	Status string `json:"status"`
}

// NewEtcdSource 创建 Etcd 发现源
// 参数：
//   - name: 发现源名称
//   - cfg: Etcd 发现配置
//
// 返回：
//   - Source: 发现源实例
//   - error: 错误信息
func NewEtcdSource(name string, cfg *config.DiscoveryEtcdConfig) (Source, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd 端点为空")
	}
	if cfg.Prefix == "" {
		return nil, fmt.Errorf("etcd 前缀为空")
	}

	return &EtcdSource{
		BaseSource: NewBaseSource(name, "etcd"),
		endpoints:  cfg.Endpoints,
		prefix:     cfg.Prefix,
		username:   cfg.Username,
		password:   cfg.Password,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Discover 从 Etcd 获取服务列表
func (e *EtcdSource) Discover(ctx context.Context) ([]*config.Backend, error) {
	// 遍历端点尝试获取
	var lastErr error
	for _, endpoint := range e.endpoints {
		backends, err := e.discoverFromEndpoint(ctx, endpoint)
		if err != nil {
			lastErr = err
			continue
		}
		return backends, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("所有 Etcd 端点均不可用")
}

// discoverFromEndpoint 从单个端点获取服务列表
func (e *EtcdSource) discoverFromEndpoint(ctx context.Context, endpoint string) ([]*config.Backend, error) {
	// 构建 Etcd v3 API 请求
	// 使用 Range API 获取前缀下的所有 key
	url := fmt.Sprintf("%s/v3/kv/range", strings.TrimSuffix(endpoint, "/"))

	// 构建请求体
	// Etcd v3 API 使用 base64 编码的 key
	reqBody := fmt.Sprintf(`{"key":"%s","range_end":"%s"}`,
		base64Encode(e.prefix),
		base64Encode(prefixEnd(e.prefix)))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 设置认证
	if e.username != "" && e.password != "" {
		req.SetBasicAuth(e.username, e.password)
	}

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Etcd 失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("etcd 返回状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析 JSON
	var rangeResp etcdRangeResponse
	if err := json.Unmarshal(body, &rangeResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为 config.Backend
	var backends []*config.Backend
	for _, kv := range rangeResp.Kvs {
		// 解码 value
		value, err := base64Decode(kv.Value)
		if err != nil {
			continue
		}

		// 解析服务信息
		var svc etcdServiceValue
		if err := json.Unmarshal([]byte(value), &svc); err != nil {
			// 尝试直接使用 value 作为 URL
			key, _ := base64Decode(kv.Key)
			backends = append(backends, &config.Backend{
				Name:   strings.TrimPrefix(key, e.prefix),
				URL:    value,
				Weight: 1,
			})
			continue
		}

		// 跳过非活跃的服务
		if svc.Status != "" && svc.Status != "enabled" && svc.Status != "active" {
			continue
		}

		weight := svc.Weight
		if weight <= 0 {
			weight = 1
		}

		backends = append(backends, &config.Backend{
			Name:   svc.Name,
			URL:    svc.URL,
			Weight: weight,
		})
	}

	return backends, nil
}

// base64Encode Base64 编码
func base64Encode(s string) string {
	return fmt.Sprintf("%x", []byte(s))
}

// base64Decode Base64 解码 (实际是 hex)
func base64Decode(s string) (string, error) {
	var result []byte
	for i := 0; i < len(s); i += 2 {
		var b byte
		_, err := fmt.Sscanf(s[i:i+2], "%02x", &b)
		if err != nil {
			return "", err
		}
		result = append(result, b)
	}
	return string(result), nil
}

// prefixEnd 计算前缀范围的结束值
func prefixEnd(prefix string) string {
	if prefix == "" {
		return ""
	}
	// 增加最后一个字符
	end := []byte(prefix)
	end[len(end)-1]++
	return string(end)
}
