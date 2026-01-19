package pipeline

import (
	"context"
)

// Provider 鉴权数据提供者接口
// 负责从不同数据源获取 API Key 相关信息
type Provider interface {
	// Name 返回 Provider 名称
	Name() string

	// Type 返回 Provider 类型
	Type() ProviderType

	// Query 根据 API Key 查询数据
	// 参数：
	//   - ctx: 上下文
	//   - apiKey: API Key 字符串
	// 返回：
	//   - *ProviderResult: 查询结果
	Query(ctx context.Context, apiKey string) *ProviderResult

	// Close 关闭连接（如果有）
	Close() error
}

// BaseProvider 基础 Provider 实现
type BaseProvider struct {
	name         string       // Provider 名称
	providerType ProviderType // Provider 类型
}

// Name 返回 Provider 名称
func (b *BaseProvider) Name() string {
	return b.name
}

// Type 返回 Provider 类型
func (b *BaseProvider) Type() ProviderType {
	return b.providerType
}

// Close 默认关闭实现（无操作）
func (b *BaseProvider) Close() error {
	return nil
}
