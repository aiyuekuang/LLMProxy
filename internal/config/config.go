package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Backend 后端服务器配置
type Backend struct {
	URL    string `yaml:"url"`    // 后端服务器 URL
	Weight int    `yaml:"weight"` // 权重（用于负载均衡）
}

// UsageHook 用量上报 Webhook 配置
type UsageHook struct {
	Enabled bool          `yaml:"enabled"` // 是否启用
	URL     string        `yaml:"url"`     // Webhook URL
	Timeout time.Duration `yaml:"timeout"` // 超时时间
	Retry   int           `yaml:"retry"`   // 重试次数
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	Interval time.Duration `yaml:"interval"` // 检查间隔
	Path     string        `yaml:"path"`     // 健康检查路径
}

// Config 主配置结构
type Config struct {
	Listen      string       `yaml:"listen"`       // 监听地址，如 ":8000"
	Backends    []Backend    `yaml:"backends"`     // 后端服务器列表
	UsageHook   *UsageHook   `yaml:"usage_hook"`   // 用量上报配置
	HealthCheck *HealthCheck `yaml:"health_check"` // 健康检查配置
}

// Load 从文件加载配置
// 参数：
//   - path: 配置文件路径
// 返回：
//   - *Config: 配置对象
//   - error: 错误信息
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.Listen == "" {
		cfg.Listen = ":8000"
	}

	if cfg.UsageHook != nil && cfg.UsageHook.Timeout == 0 {
		cfg.UsageHook.Timeout = 1 * time.Second
	}

	if cfg.HealthCheck != nil {
		if cfg.HealthCheck.Interval == 0 {
			cfg.HealthCheck.Interval = 10 * time.Second
		}
		if cfg.HealthCheck.Path == "" {
			cfg.HealthCheck.Path = "/health"
		}
	}

	return &cfg, nil
}
