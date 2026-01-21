package lb

import (
	"context"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// RoundRobin 轮询负载均衡器
type RoundRobin struct {
	*BaseLoadBalancer        // 嵌入基础负载均衡器
	current           int    // 当前索引
	mu                sync.Mutex // 互斥锁
}

// NewRoundRobin 创建轮询负载均衡器
// 参数：
//   - backends: 后端配置列表
//   - healthCheck: 健康检查配置
// 返回：
//   - LoadBalancer: 负载均衡器实例
func NewRoundRobin(backends []*config.Backend, healthCheck *config.HealthCheckConfig) LoadBalancer {
	return &RoundRobin{
		BaseLoadBalancer: NewBaseLoadBalancer(backends, healthCheck),
		current:          0,
	}
}

// Next 获取下一个健康的后端
// 使用加权轮询算法
// 返回：
//   - *Backend: 后端实例，如果没有健康后端则返回 nil
func (r *RoundRobin) Next() *Backend {
	r.mu.Lock()
	defer r.mu.Unlock()

	backends := r.GetBackends()
	if len(backends) == 0 {
		return nil
	}

	// 尝试最多 len(backends) 次，找到健康的后端
	attempts := 0
	maxAttempts := len(backends)

	for attempts < maxAttempts {
		backend := backends[r.current]
		r.current = (r.current + 1) % len(backends)

		if backend.Healthy {
			return backend
		}

		attempts++
	}

	// 没有健康的后端
	return nil
}

// UpdateHealth 更新后端健康状态
// 参数：
//   - backend: 后端实例
//   - healthy: 健康状态
func (r *RoundRobin) UpdateHealth(backend *Backend, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldStatus := backend.Healthy
	backend.Healthy = healthy
	LogHealthChange(backend, oldStatus, healthy)
}

// RecordResult 记录请求结果（轮询策略不需要统计）
// 参数：
//   - backend: 后端实例
//   - latency: 请求延迟
//   - err: 错误信息
func (r *RoundRobin) RecordResult(backend *Backend, latency time.Duration, err error) {
	// 轮询策略不需要记录结果
}

// Start 启动健康检查
// 参数：
//   - ctx: 上下文，用于取消健康检查
func (r *RoundRobin) Start(ctx context.Context) {
	r.StartHealthCheck(ctx, r.UpdateHealth, "轮询")
}
