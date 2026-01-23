package discovery

import (
	"context"

	"llmproxy/internal/config"
)

// StaticSource 静态配置发现源
// 从配置文件直接读取后端服务列表
type StaticSource struct {
	BaseSource
	backends []*config.Backend
}

// NewStaticSource 创建静态发现源
// 参数：
//   - name: 发现源名称
//   - backends: 后端服务列表
//
// 返回：
//   - Source: 发现源实例
func NewStaticSource(name string, backends []*config.Backend) Source {
	return &StaticSource{
		BaseSource: NewBaseSource(name, "static"),
		backends:   backends,
	}
}

// Discover 返回静态配置的后端服务列表
func (s *StaticSource) Discover(ctx context.Context) ([]*config.Backend, error) {
	return s.backends, nil
}
