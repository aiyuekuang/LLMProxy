package lb

import (
	"context"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// Weighted 加权轮询负载均衡器
// 使用平滑加权轮询算法 (Smooth Weighted Round-Robin)
type Weighted struct {
	*BaseLoadBalancer
	weights []int      // 当前权重
	mu      sync.Mutex // 互斥锁
}

// NewWeighted 创建加权轮询负载均衡器
// 参数：
//   - backends: 后端配置列表
//   - healthCheck: 健康检查配置
//
// 返回：
//   - LoadBalancer: 负载均衡器实例
func NewWeighted(backends []*config.Backend, healthCheck *config.HealthCheckConfig) LoadBalancer {
	base := NewBaseLoadBalancer(backends, healthCheck)
	w := &Weighted{
		BaseLoadBalancer: base,
		weights:          make([]int, len(base.backends)),
	}
	return w
}

// Next 获取下一个健康的后端
// 使用平滑加权轮询算法：
// 1. 每次选择时，给每个后端的当前权重加上其原始权重
// 2. 选择当前权重最大的健康后端
// 3. 被选中的后端，当前权重减去所有后端的权重总和
//
// 返回：
//   - *Backend: 后端实例，如果没有健康后端则返回 nil
func (w *Weighted) Next() *Backend {
	w.mu.Lock()
	defer w.mu.Unlock()

	backends := w.GetBackends()
	if len(backends) == 0 {
		return nil
	}

	// 确保权重数组长度一致
	if len(w.weights) != len(backends) {
		w.weights = make([]int, len(backends))
	}

	// 计算总权重（仅健康后端）
	totalWeight := 0
	for i, bk := range backends {
		if bk.Healthy {
			totalWeight += bk.Weight
			w.weights[i] += bk.Weight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	// 选择当前权重最大的健康后端
	maxIdx := -1
	maxWeight := -1
	for i, bk := range backends {
		if bk.Healthy && w.weights[i] > maxWeight {
			maxWeight = w.weights[i]
			maxIdx = i
		}
	}

	if maxIdx < 0 {
		return nil
	}

	// 被选中的后端，当前权重减去总权重
	w.weights[maxIdx] -= totalWeight

	return backends[maxIdx]
}

// UpdateHealth 更新后端健康状态
// 参数：
//   - backend: 后端实例
//   - healthy: 健康状态
func (w *Weighted) UpdateHealth(backend *Backend, healthy bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	oldStatus := backend.Healthy
	backend.Healthy = healthy
	LogHealthChange(backend, oldStatus, healthy)
}

// RecordResult 记录请求结果（加权策略不需要统计）
// 参数：
//   - backend: 后端实例
//   - latency: 请求延迟
//   - err: 错误信息
func (w *Weighted) RecordResult(backend *Backend, latency time.Duration, err error) {
	// 加权轮询策略不需要记录结果
}

// Start 启动健康检查
// 参数：
//   - ctx: 上下文，用于取消健康检查
func (w *Weighted) Start(ctx context.Context) {
	w.StartHealthCheck(ctx, w.UpdateHealth, "加权轮询")
}
