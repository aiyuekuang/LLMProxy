package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"llmproxy/internal/config"
)

// KubernetesSource Kubernetes 服务发现源
// 通过 Kubernetes API 获取 Endpoints
type KubernetesSource struct {
	BaseSource
	namespace     string
	service       string
	port          int
	labelSelector string
	httpClient    *http.Client
	token         string
	apiServer     string
}

// k8sEndpoints Kubernetes Endpoints 结构
type k8sEndpoints struct {
	Subsets []k8sSubset `json:"subsets"`
}

// k8sSubset Kubernetes Subset
type k8sSubset struct {
	Addresses []k8sAddress `json:"addresses"`
	Ports     []k8sPort    `json:"ports"`
}

// k8sAddress Kubernetes 地址
type k8sAddress struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// k8sPort Kubernetes 端口
type k8sPort struct {
	Port int    `json:"port"`
	Name string `json:"name"`
}

// NewKubernetesSource 创建 Kubernetes 发现源
// 参数：
//   - name: 发现源名称
//   - cfg: Kubernetes 发现配置
//
// 返回：
//   - Source: 发现源实例
//   - error: 错误信息
func NewKubernetesSource(name string, cfg *config.DiscoveryK8sConfig) (Source, error) {
	if cfg.Service == "" {
		return nil, fmt.Errorf("kubernetes 服务名为空")
	}

	namespace := cfg.Namespace
	if namespace == "" {
		// 尝试从环境变量或文件读取命名空间
		namespace = os.Getenv("KUBERNETES_NAMESPACE")
		if namespace == "" {
			data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
			if err == nil {
				namespace = string(data)
			}
		}
		if namespace == "" {
			namespace = "default"
		}
	}

	port := cfg.Port
	if port <= 0 {
		port = 80
	}

	// 读取 ServiceAccount Token
	token := ""
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err == nil {
		token = string(data)
	}

	// 获取 API Server 地址
	apiServer := os.Getenv("KUBERNETES_SERVICE_HOST")
	apiPort := os.Getenv("KUBERNETES_SERVICE_PORT")
	if apiServer == "" {
		apiServer = "kubernetes.default.svc"
		apiPort = "443"
	}

	return &KubernetesSource{
		BaseSource:    NewBaseSource(name, "kubernetes"),
		namespace:     namespace,
		service:       cfg.Service,
		port:          port,
		labelSelector: cfg.LabelSelector,
		token:         token,
		apiServer:     fmt.Sprintf("https://%s:%s", apiServer, apiPort),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			// 注意：生产环境应配置正确的 TLS
			Transport: &http.Transport{
				TLSClientConfig: nil, // 使用系统证书
			},
		},
	}, nil
}

// Discover 从 Kubernetes API 获取 Endpoints
func (k *KubernetesSource) Discover(ctx context.Context) ([]*config.Backend, error) {
	// 构建 API URL
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/endpoints/%s",
		k.apiServer, k.namespace, k.service)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置认证头
	if k.token != "" {
		req.Header.Set("Authorization", "Bearer "+k.token)
	}

	// 发送请求
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Kubernetes API 失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kubernetes API 返回状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析 JSON
	var endpoints k8sEndpoints
	if err := json.Unmarshal(body, &endpoints); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为 config.Backend
	var backends []*config.Backend
	for _, subset := range endpoints.Subsets {
		// 获取端口
		port := k.port
		for _, p := range subset.Ports {
			if p.Port > 0 {
				port = p.Port
				break
			}
		}

		// 遍历地址
		for i, addr := range subset.Addresses {
			name := addr.Hostname
			if name == "" {
				name = fmt.Sprintf("%s-%d", k.service, i)
			}

			backends = append(backends, &config.Backend{
				Name:   name,
				URL:    fmt.Sprintf("http://%s:%d", addr.IP, port),
				Weight: 1,
			})
		}
	}

	return backends, nil
}
