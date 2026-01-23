package lb

import (
	"context"
	"time"
)

// Backend 后端服务器信息
type Backend struct {
	URL     string // 后端 URL
	Weight  int    // 权重
	Healthy bool   // 健康状态
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// Next 获取下一个后端
	// 返回：
	//   - *Backend: 后端实例，如果没有健康后端则返回 nil
	Next() *Backend

	// UpdateHealth 更新后端健康状态
	// 参数：
	//   - backend: 后端实例
	//   - healthy: 健康状态
	UpdateHealth(backend *Backend, healthy bool)

	// RecordResult 记录请求结果（用于统计）
	// 参数：
	//   - backend: 后端实例
	//   - latency: 请求延迟
	//   - err: 错误信息（nil 表示成功）
	RecordResult(backend *Backend, latency time.Duration, err error)

	// Start 启动健康检查
	// 参数：
	//   - ctx: 上下文，用于取消健康检查
	Start(ctx context.Context)
}
