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
	Models  []string // 支持的模型列表，空表示支持所有模型
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// Next 获取下一个后端
	// 参数：
	//   - model: 模型名称（可选，用于模型级路由）
	// 返回：
	//   - *Backend: 后端实例，如果没有健康后端则返回 nil
	Next(model string) *Backend
	
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

// MatchModel 检查后端是否支持指定模型
// 参数：
//   - backend: 后端实例
//   - model: 模型名称
// 返回：
//   - bool: 是否支持
func MatchModel(backend *Backend, model string) bool {
	// 空列表表示支持所有模型
	if len(backend.Models) == 0 {
		return true
	}
	
	// 检查模型是否在列表中
	for _, m := range backend.Models {
		if m == model {
			return true
		}
		// 支持通配符匹配（如 "llama-3*" 匹配 "llama-3-70b"）
		if len(m) > 0 && m[len(m)-1] == '*' {
			prefix := m[:len(m)-1]
			if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
				return true
			}
		}
	}
	
	return false
}
