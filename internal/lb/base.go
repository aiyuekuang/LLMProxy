package lb

import (
	"context"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/config"
)

// BaseLoadBalancer 基础负载均衡器（提供通用功能）
type BaseLoadBalancer struct {
	backends    []*Backend                // 后端列表
	healthCheck *config.HealthCheckConfig // 健康检查配置
	httpClient  *http.Client              // HTTP 客户端
}

// NewBaseLoadBalancer 创建基础负载均衡器
// 参数：
//   - backends: 后端配置列表
//   - healthCheck: 健康检查配置
//
// 返回：
//   - *BaseLoadBalancer: 基础负载均衡器实例
func NewBaseLoadBalancer(backends []*config.Backend, healthCheck *config.HealthCheckConfig) *BaseLoadBalancer {
	base := &BaseLoadBalancer{
		backends:    make([]*Backend, 0, len(backends)),
		healthCheck: healthCheck,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}

	// 初始化后端列表
	for _, b := range backends {
		if b == nil {
			continue
		}
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		base.backends = append(base.backends, &Backend{
			URL:     b.URL,
			Weight:  weight,
			Healthy: true,
		})
	}

	return base
}

// GetBackends 获取后端列表
// 返回：
//   - []*Backend: 后端列表
func (b *BaseLoadBalancer) GetBackends() []*Backend {
	return b.backends
}

// StartHealthCheck 启动健康检查
// 参数：
//   - ctx: 上下文，用于取消健康检查
//   - updateFunc: 更新健康状态的函数
//   - strategyName: 策略名称（用于日志）
func (b *BaseLoadBalancer) StartHealthCheck(ctx context.Context, updateFunc func(*Backend, bool), strategyName string) {
	if b.healthCheck == nil {
		log.Println("健康检查未配置，跳过")
		return
	}

	ticker := time.NewTicker(b.healthCheck.Interval)
	defer ticker.Stop()

	log.Printf("健康检查已启动（%s策略），间隔: %v", strategyName, b.healthCheck.Interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("健康检查已停止")
			return
		case <-ticker.C:
			b.checkHealth(updateFunc)
		}
	}
}

// checkHealth 执行健康检查
// 参数：
//   - updateFunc: 更新健康状态的函数
func (b *BaseLoadBalancer) checkHealth(updateFunc func(*Backend, bool)) {
	for _, backend := range b.backends {
		go func(bk *Backend) {
			healthy := b.isHealthy(bk)
			updateFunc(bk, healthy)
		}(backend)
	}
}

// isHealthy 检查后端是否健康
// 参数：
//   - backend: 后端实例
//
// 返回：
//   - bool: 是否健康
func (b *BaseLoadBalancer) isHealthy(backend *Backend) bool {
	if b.healthCheck == nil {
		return true
	}

	path := b.healthCheck.Path
	if path == "" {
		path = "/health"
	}

	url := backend.URL + path
	resp, err := b.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	expectedStatus := b.healthCheck.ExpectedStatus
	if expectedStatus == 0 {
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	}
	return resp.StatusCode == expectedStatus
}

// LogHealthChange 记录健康状态变化
// 参数：
//   - backend: 后端实例
//   - oldStatus: 旧状态
//   - newStatus: 新状态
func LogHealthChange(backend *Backend, oldStatus, newStatus bool) {
	if oldStatus != newStatus {
		if newStatus {
			log.Printf("后端 %s 恢复健康", backend.URL)
		} else {
			log.Printf("后端 %s 不健康", backend.URL)
		}
	}
}
