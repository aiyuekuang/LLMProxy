package lb

import (
	"context"
	"sync"
	"time"

	"llmproxy/internal/config"
)

// LeastConnections 最少连接数负载均衡器
type LeastConnections struct {
	*BaseLoadBalancer        // 嵌入基础负载均衡器
	concurrent        map[string]int // 每个后端的当前并发数
	mu                sync.RWMutex   // 读写锁
}

// NewLeastConnections 创建最少连接数负载均衡器
// 参数：
//   - backends: 后端配置列表
//   - healthCheck: 健康检查配置
// 返回：
//   - LoadBalancer: 负载均衡器实例
func NewLeastConnections(backends []config.Backend, healthCheck *config.HealthCheck) LoadBalancer {
	lb := &LeastConnections{
		BaseLoadBalancer: NewBaseLoadBalancer(backends, healthCheck),
		concurrent:       make(map[string]int),
	}

	// 初始化并发计数
	for _, b := range backends {
		lb.concurrent[b.URL] = 0
	}

	return lb
}

// Next 获取并发数最少的健康后端
// 参数：
//   - model: 模型名称（可选）
// 返回：
//   - *Backend: 后端实例
func (lc *LeastConnections) Next(model string) *Backend {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	var selected *Backend
	minConnections := int(^uint(0) >> 1) // 最大整数

	for _, backend := range lc.GetBackends() {
		if !backend.Healthy {
			continue
		}

		// 检查模型匹配
		if model != "" && !MatchModel(backend, model) {
			continue
		}

		connections := lc.concurrent[backend.URL]
		if connections < minConnections {
			minConnections = connections
			selected = backend
		}
	}

	if selected != nil {
		// 增加并发计数
		lc.concurrent[selected.URL]++
	}

	return selected
}

// UpdateHealth 更新后端健康状态
// 参数：
//   - backend: 后端实例
//   - healthy: 健康状态
func (lc *LeastConnections) UpdateHealth(backend *Backend, healthy bool) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	oldStatus := backend.Healthy
	backend.Healthy = healthy
	LogHealthChange(backend, oldStatus, healthy)
}

// RecordResult 记录请求结果
// 参数：
//   - backend: 后端实例
//   - latency: 请求延迟
//   - err: 错误信息
func (lc *LeastConnections) RecordResult(backend *Backend, latency time.Duration, err error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// 减少并发计数
	if lc.concurrent[backend.URL] > 0 {
		lc.concurrent[backend.URL]--
	}
}

// Start 启动健康检查
// 参数：
//   - ctx: 上下文，用于取消健康检查
func (lc *LeastConnections) Start(ctx context.Context) {
	lc.StartHealthCheck(ctx, lc.UpdateHealth, "最少连接数")
}
