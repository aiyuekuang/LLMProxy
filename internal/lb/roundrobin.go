package lb

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// Backend 后端服务器信息
type Backend struct {
	URL     string // 后端 URL
	Weight  int    // 权重
	Healthy bool   // 健康状态
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	Next() *Backend           // 获取下一个后端
	Start(ctx context.Context) // 启动健康检查
}

// RoundRobin 轮询负载均衡器
type RoundRobin struct {
	backends      []*Backend    // 后端列表
	current       int           // 当前索引
	mu            sync.Mutex    // 互斥锁
	healthCheck   *config.HealthCheck // 健康检查配置
	httpClient    *http.Client  // HTTP 客户端
}

// NewRoundRobin 创建轮询负载均衡器
// 参数：
//   - backends: 后端配置列表
//   - healthCheck: 健康检查配置
// 返回：
//   - LoadBalancer: 负载均衡器实例
func NewRoundRobin(backends []config.Backend, healthCheck *config.HealthCheck) LoadBalancer {
	lb := &RoundRobin{
		backends:    make([]*Backend, 0, len(backends)),
		current:     0,
		healthCheck: healthCheck,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}

	// 初始化后端列表
	for _, b := range backends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1 // 默认权重为 1
		}
		lb.backends = append(lb.backends, &Backend{
			URL:     b.URL,
			Weight:  weight,
			Healthy: true, // 初始状态为健康
		})
	}

	return lb
}

// Next 获取下一个健康的后端
// 使用加权轮询算法
// 返回：
//   - *Backend: 后端实例，如果没有健康后端则返回 nil
func (r *RoundRobin) Next() *Backend {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.backends) == 0 {
		return nil
	}

	// 尝试最多 len(backends) 次，找到健康的后端
	attempts := 0
	maxAttempts := len(r.backends)

	for attempts < maxAttempts {
		backend := r.backends[r.current]
		r.current = (r.current + 1) % len(r.backends)

		if backend.Healthy {
			return backend
		}

		attempts++
	}

	// 没有健康的后端
	return nil
}

// Start 启动健康检查
// 参数：
//   - ctx: 上下文，用于取消健康检查
func (r *RoundRobin) Start(ctx context.Context) {
	if r.healthCheck == nil {
		log.Println("健康检查未配置，跳过")
		return
	}

	ticker := time.NewTicker(r.healthCheck.Interval)
	defer ticker.Stop()

	log.Printf("健康检查已启动，间隔: %v", r.healthCheck.Interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("健康检查已停止")
			return
		case <-ticker.C:
			r.checkHealth()
		}
	}
}

// checkHealth 执行健康检查
func (r *RoundRobin) checkHealth() {
	for _, backend := range r.backends {
		go func(b *Backend) {
			healthy := r.isHealthy(b)
			
			r.mu.Lock()
			oldStatus := b.Healthy
			b.Healthy = healthy
			r.mu.Unlock()

			// 状态变化时记录日志
			if oldStatus != healthy {
				if healthy {
					log.Printf("后端 %s 恢复健康", b.URL)
				} else {
					log.Printf("后端 %s 不健康", b.URL)
				}
			}
		}(backend)
	}
}

// isHealthy 检查后端是否健康
// 参数：
//   - backend: 后端实例
// 返回：
//   - bool: 是否健康
func (r *RoundRobin) isHealthy(backend *Backend) bool {
	url := backend.URL + r.healthCheck.Path
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
