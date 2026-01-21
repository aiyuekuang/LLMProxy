package routing

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/lb"
)

// Router 智能路由器
type Router struct {
	config        *RoutingConfig    // 路由配置
	loadBalancer  lb.LoadBalancer   // 负载均衡器
	httpClient    *http.Client      // HTTP 客户端
	backendMap    map[string]*lb.Backend // URL -> Backend 映射
}

// NewRouter 创建路由器
// 参数：
//   - config: 路由配置
//   - loadBalancer: 负载均衡器
//   - backends: 后端列表
// 返回：
//   - *Router: 路由器实例
func NewRouter(config *RoutingConfig, loadBalancer lb.LoadBalancer, backends []*lb.Backend) *Router {
	backendMap := make(map[string]*lb.Backend)
	for _, b := range backends {
		backendMap[b.URL] = b
	}

	return &Router{
		config:       config,
		loadBalancer: loadBalancer,
		httpClient: &http.Client{
			Timeout: 0, // 不设置超时，由后端控制
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		backendMap: backendMap,
	}
}

// ProxyRequest 代理请求（带重试和故障转移）
// 参数：
//   - r: HTTP 请求
//   - bodyBytes: 请求体
//   - model: 模型名（已废弃，保留参数以兼容）
// 返回：
//   - *http.Response: 响应
//   - *lb.Backend: 使用的后端
//   - error: 错误信息
func (r *Router) ProxyRequest(req *http.Request, bodyBytes []byte, model string) (*http.Response, *lb.Backend, error) {
	// 查找 fallback 规则
	rule := r.findFallbackRule(model)
	
	if rule == nil {
		// 没有 fallback 规则，使用负载均衡器选择后端
		return r.proxyWithRetry(req, bodyBytes, model, nil)
	}
	
	// 尝试主后端
	primary := r.backendMap[rule.Primary]
	if primary != nil && primary.Healthy {
		resp, backend, err := r.proxyWithRetry(req, bodyBytes, model, primary)
		if err == nil {
			return resp, backend, nil
		}
		log.Printf("主后端 %s 失败: %v", rule.Primary, err)
	}
	
	// 尝试备用后端
	for _, fallbackURL := range rule.Fallback {
		backend := r.backendMap[fallbackURL]
		if backend == nil || !backend.Healthy {
			continue
		}
		
		log.Printf("故障转移到: %s", fallbackURL)
		resp, backend, err := r.proxyWithRetry(req, bodyBytes, model, backend)
		if err == nil {
			return resp, backend, nil
		}
		log.Printf("备用后端 %s 失败: %v", fallbackURL, err)
	}
	
	return nil, nil, fmt.Errorf("所有后端均失败，模型: %s", model)
}

// proxyWithRetry 代理请求（带重试）
// 参数：
//   - req: HTTP 请求
//   - bodyBytes: 请求体
//   - model: 模型名
//   - backend: 指定后端（nil 表示使用负载均衡器选择）
// 返回：
//   - *http.Response: 响应
//   - *lb.Backend: 使用的后端
//   - error: 错误信息
func (r *Router) proxyWithRetry(req *http.Request, bodyBytes []byte, model string, backend *lb.Backend) (*http.Response, *lb.Backend, error) {
	var resp *http.Response
	var selectedBackend *lb.Backend
	var lastErr error
	
	// 重试逻辑
	err := retryRequest(r.config.Retry, func() (int, error) {
		// 选择后端
		if backend == nil {
			selectedBackend = r.loadBalancer.Next()
			if selectedBackend == nil {
				return 503, fmt.Errorf("没有可用的健康后端")
			}
		} else {
			selectedBackend = backend
		}
		
		// 构造代理请求
		proxyReq, err := http.NewRequest(req.Method, selectedBackend.URL+req.URL.Path, bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, err
		}
		
		// 复制请求头
		proxyReq.Header = req.Header.Clone()
		
		// 发送请求
		start := time.Now()
		resp, err = r.httpClient.Do(proxyReq)
		latency := time.Since(start)
		
		// 记录结果
		r.loadBalancer.RecordResult(selectedBackend, latency, err)
		
		if err != nil {
			lastErr = err
			return 0, err
		}
		
		lastErr = nil
		return resp.StatusCode, nil
	})
	
	if err != nil {
		if lastErr != nil {
			return nil, selectedBackend, lastErr
		}
		return nil, selectedBackend, err
	}
	
	return resp, selectedBackend, nil
}

// findFallbackRule 查找适用的 fallback 规则
// 参数：
//   - model: 模型名
// 返回：
//   - *FallbackRule: fallback 规则，nil 表示没有
func (r *Router) findFallbackRule(model string) *FallbackRule {
	if r.config == nil || len(r.config.Fallback) == 0 {
		return nil
	}
	
	for _, rule := range r.config.Fallback {
		// 空列表表示适用于所有模型
		if len(rule.Models) == 0 {
			return &rule
		}
		
		// 检查模型是否匹配
		for _, m := range rule.Models {
			if m == model {
				return &rule
			}
			// 支持通配符匹配
			if len(m) > 0 && m[len(m)-1] == '*' {
				prefix := m[:len(m)-1]
				if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
					return &rule
				}
			}
		}
	}
	
	return nil
}
