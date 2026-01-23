package discovery

import (
	"context"

	"llmproxy/internal/config"
)

// Source 服务发现源接口
type Source interface {
	// Name 返回发现源名称
	Name() string

	// Type 返回发现源类型
	Type() string

	// Discover 执行服务发现
	// 返回：
	//   - []*config.Backend: 发现的后端服务列表
	//   - error: 错误信息
	Discover(ctx context.Context) ([]*config.Backend, error)

	// Close 关闭发现源
	Close() error
}

// BaseSource 基础发现源（提供通用功能）
type BaseSource struct {
	name       string
	sourceType string
}

// NewBaseSource 创建基础发现源
func NewBaseSource(name, sourceType string) BaseSource {
	return BaseSource{
		name:       name,
		sourceType: sourceType,
	}
}

// Name 返回发现源名称
func (b *BaseSource) Name() string {
	return b.name
}

// Type 返回发现源类型
func (b *BaseSource) Type() string {
	return b.sourceType
}

// Close 默认关闭实现
func (b *BaseSource) Close() error {
	return nil
}
