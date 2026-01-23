package lb

import (
	"context"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// LatencyBased 延迟优先负载均衡器
type LatencyBased struct {
	*BaseLoadBalancer                          // 嵌入基础负载均衡器
	latency           map[string]time.Duration // 每个后端的平均延迟
	mu                sync.RWMutex             // 读写锁
}

// NewLatencyBased 创建延迟优先负载均衡器
// 参数：
//   - backends: 后端配置列表
//   - healthCheck: 健康检查配置
//
// 返回：
//   - LoadBalancer: 负载均衡器实例
func NewLatencyBased(backends []*config.Backend, healthCheck *config.HealthCheckConfig) LoadBalancer {
	lb := &LatencyBased{
		BaseLoadBalancer: NewBaseLoadBalancer(backends, healthCheck),
		latency:          make(map[string]time.Duration),
	}

	// 初始化延迟统计
	for _, b := range backends {
		if b != nil {
			lb.latency[b.URL] = 100 * time.Millisecond // 默认延迟
		}
	}

	return lb
}

// Next 获取延迟最低的健康后端
// 返回：
//   - *Backend: 后端实例
func (lb *LatencyBased) Next() *Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var selected *Backend
	minLatency := time.Duration(1<<63 - 1) // 最大时间

	for _, backend := range lb.GetBackends() {
		if !backend.Healthy {
			continue
		}

		latency := lb.latency[backend.URL]
		if latency == 0 {
			latency = 100 * time.Millisecond // 默认延迟
		}

		if latency < minLatency {
			minLatency = latency
			selected = backend
		}
	}

	return selected
}

// UpdateHealth 更新后端健康状态
// 参数：
//   - backend: 后端实例
//   - healthy: 健康状态
func (lb *LatencyBased) UpdateHealth(backend *Backend, healthy bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	oldStatus := backend.Healthy
	backend.Healthy = healthy
	LogHealthChange(backend, oldStatus, healthy)
}

// RecordResult 记录请求结果，更新延迟统计
// 使用指数移动平均（EMA）算法
// 参数：
//   - backend: 后端实例
//   - latency: 请求延迟
//   - err: 错误信息
func (lb *LatencyBased) RecordResult(backend *Backend, latency time.Duration, err error) {
	if err != nil {
		// 请求失败，不更新延迟统计
		return
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	// 使用指数移动平均（EMA）更新延迟
	// 新延迟 = alpha * 当前延迟 + (1 - alpha) * 旧延迟
	alpha := 0.3 // 平滑系数
	oldLatency := lb.latency[backend.URL]
	if oldLatency == 0 {
		lb.latency[backend.URL] = latency
	} else {
		lb.latency[backend.URL] = time.Duration(
			alpha*float64(latency) + (1-alpha)*float64(oldLatency),
		)
	}
}

// Start 启动健康检查
// 参数：
//   - ctx: 上下文，用于取消健康检查
func (lb *LatencyBased) Start(ctx context.Context) {
	lb.StartHealthCheck(ctx, lb.UpdateHealth, "延迟优先")
}
