package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Backend 后端服务器配置
type Backend struct {
	URL    string   `yaml:"url"`    // 后端服务器 URL
	Weight int      `yaml:"weight"` // 权重（用于负载均衡）
	Models []string `yaml:"models"` // 支持的模型列表，空表示支持所有模型
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

// RoutingConfig 路由配置
type RoutingConfig struct {
	Retry               RetryConfig       `yaml:"retry"`
	Fallback            []FallbackRule    `yaml:"fallback"`
	LoadBalanceStrategy string            `yaml:"load_balance_strategy"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	Enabled     bool          `yaml:"enabled"`
	MaxRetries  int           `yaml:"max_retries"`
	InitialWait time.Duration `yaml:"initial_wait"`
	MaxWait     time.Duration `yaml:"max_wait"`
	Multiplier  float64       `yaml:"multiplier"`
}

// FallbackRule 故障转移规则
type FallbackRule struct {
	Primary  string   `yaml:"primary"`
	Fallback []string `yaml:"fallback"`
	Models   []string `yaml:"models"`
}

// AuthConfig 鉴权配置
type AuthConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Storage  string        `yaml:"storage"`
	Defaults *DefaultConfig `yaml:"defaults"`
}

// DefaultConfig 默认配置
type DefaultConfig struct {
	QuotaResetPeriod string `yaml:"quota_reset_period"`
	TotalQuota       int64  `yaml:"total_quota"`
}

// APIKey API Key 结构
type APIKey struct {
	Key              string     `yaml:"key" json:"key"`
	Name             string     `yaml:"name" json:"name"`
	UserID           string     `yaml:"user_id" json:"user_id"`
	Status           string     `yaml:"status" json:"status"`
	TotalQuota       int64      `yaml:"total_quota" json:"total_quota"`
	UsedQuota        int64      `yaml:"used_quota" json:"used_quota"`
	QuotaResetPeriod string     `yaml:"quota_reset_period" json:"quota_reset_period"`
	LastResetAt      time.Time  `yaml:"last_reset_at" json:"last_reset_at"`
	AllowedIPs       []string   `yaml:"allowed_ips" json:"allowed_ips"`
	DeniedIPs        []string   `yaml:"denied_ips" json:"denied_ips"`
	ExpiresAt        *time.Time `yaml:"expires_at" json:"expires_at"`
	CreatedAt        time.Time  `yaml:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `yaml:"updated_at" json:"updated_at"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled  bool         `yaml:"enabled"`
	Storage  string       `yaml:"storage"`
	Global   *GlobalLimit `yaml:"global"`
	PerKey   *KeyLimit    `yaml:"per_key"`
}

// GlobalLimit 全局限流配置
type GlobalLimit struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerSecond int  `yaml:"requests_per_second"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	BurstSize         int  `yaml:"burst_size"`
}

// KeyLimit Key 级限流配置
type KeyLimit struct {
	Enabled           bool  `yaml:"enabled"`
	RequestsPerSecond int   `yaml:"requests_per_second"`
	RequestsPerMinute int   `yaml:"requests_per_minute"`
	TokensPerMinute   int64 `yaml:"tokens_per_minute"`
	MaxConcurrent     int   `yaml:"max_concurrent"`
	BurstSize         int   `yaml:"burst_size"`
}

// ScriptsConfig Lua 脚本配置
type ScriptsConfig struct {
	Routing           *ScriptConfig `yaml:"routing"`
	Auth              *ScriptConfig `yaml:"auth"`
	RequestTransform  *ScriptConfig `yaml:"request_transform"`
	ResponseTransform *ScriptConfig `yaml:"response_transform"`
	RateLimit         *ScriptConfig `yaml:"rate_limit"`
	Usage             *ScriptConfig `yaml:"usage"`
	ErrorHandler      *ScriptConfig `yaml:"error_handler"`
}

// ScriptConfig 单个脚本配置
type ScriptConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Script     string        `yaml:"script"`
	ScriptFile string        `yaml:"script_file"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxMemory  int           `yaml:"max_memory"`
}

// Config 主配置结构
type Config struct {
	Listen      string           `yaml:"listen"`
	Backends    []Backend        `yaml:"backends"`
	UsageHook   *UsageHook       `yaml:"usage_hook"`
	HealthCheck *HealthCheck     `yaml:"health_check"`
	Routing     *RoutingConfig   `yaml:"routing"`
	Auth        *AuthConfig      `yaml:"auth"`
	APIKeys     []*APIKey        `yaml:"api_keys"`
	RateLimit   *RateLimitConfig `yaml:"rate_limit"`
	Scripts     *ScriptsConfig   `yaml:"scripts"` // Lua 脚本配置
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

	// 路由配置默认值
	if cfg.Routing != nil {
		if cfg.Routing.Retry.Enabled && cfg.Routing.Retry.MaxRetries == 0 {
			cfg.Routing.Retry.MaxRetries = 3
		}
		if cfg.Routing.Retry.InitialWait == 0 {
			cfg.Routing.Retry.InitialWait = 1 * time.Second
		}
		if cfg.Routing.Retry.MaxWait == 0 {
			cfg.Routing.Retry.MaxWait = 10 * time.Second
		}
		if cfg.Routing.Retry.Multiplier == 0 {
			cfg.Routing.Retry.Multiplier = 2.0
		}
		if cfg.Routing.LoadBalanceStrategy == "" {
			cfg.Routing.LoadBalanceStrategy = "round_robin"
		}
	}

	// 鉴权配置默认值
	if cfg.Auth != nil && cfg.Auth.Enabled {
		if cfg.Auth.Storage == "" {
			cfg.Auth.Storage = "file"
		}
	}

	// 限流配置默认值
	if cfg.RateLimit != nil && cfg.RateLimit.Enabled {
		if cfg.RateLimit.Storage == "" {
			cfg.RateLimit.Storage = "memory"
		}
	}

	return &cfg, nil
}
