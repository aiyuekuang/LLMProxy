package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ============================================================
//                    服务器配置
// ============================================================

// ServerConfig 服务器配置
type ServerConfig struct {
	Listen         string        `yaml:"listen"`           // 监听地址
	ReadTimeout    time.Duration `yaml:"read_timeout"`     // 读取超时
	WriteTimeout   time.Duration `yaml:"write_timeout"`    // 写入超时
	IdleTimeout    time.Duration `yaml:"idle_timeout"`     // 空闲超时
	MaxHeaderBytes int           `yaml:"max_header_bytes"` // 最大请求头大小
	MaxBodySize    int64         `yaml:"max_body_size"`    // 最大请求体大小
	CORS           *CORSConfig   `yaml:"cors"`             // CORS 配置
	TLS            *TLSConfig    `yaml:"tls"`              // TLS 配置
}

// CORSConfig CORS 跨域配置
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// TLSConfig TLS 配置
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	ClientCAFile string `yaml:"client_ca_file"`
	ClientAuth   string `yaml:"client_auth"`
}

// ============================================================
//                    系统日志配置
// ============================================================

// LogConfig 系统日志配置
type LogConfig struct {
	Level  string         `yaml:"level"`  // debug / info / warn / error
	Format string         `yaml:"format"` // json / text
	Output string         `yaml:"output"` // stdout / stderr / file
	File   *LogFileConfig `yaml:"file"`
}

// LogFileConfig 日志文件配置
type LogFileConfig struct {
	Path     string `yaml:"path"`
	Rotate   string `yaml:"rotate"`   // daily / hourly / size
	MaxSize  int    `yaml:"max_size"` // MB
	MaxAge   int    `yaml:"max_age"`  // 天
	Compress bool   `yaml:"compress"`
}

// ============================================================
//                    存储配置（多数据源）
// ============================================================

// StorageConfig 顶层存储配置（多数据源）
type StorageConfig struct {
	Databases []*DatabaseConnection `yaml:"databases"` // 数据库连接池
	Caches    []*CacheConnection    `yaml:"caches"`    // 缓存连接池
}

// DatabaseConnection 数据库连接配置
type DatabaseConnection struct {
	Name            string        `yaml:"name"`               // 连接名称
	Enabled         bool          `yaml:"enabled"`            // 是否启用
	Driver          string        `yaml:"driver"`             // 驱动: mysql / postgres / sqlite
	DSN             string        `yaml:"dsn"`                // 直接指定 DSN
	Host            string        `yaml:"host"`               // 主机
	Port            int           `yaml:"port"`               // 端口
	User            string        `yaml:"user"`               // 用户名
	Password        string        `yaml:"password"`           // 密码
	Database        string        `yaml:"database"`           // 数据库名
	Path            string        `yaml:"path"`               // SQLite 路径
	MaxOpenConns    int           `yaml:"max_open_conns"`     // 最大打开连接数
	MaxIdleConns    int           `yaml:"max_idle_conns"`     // 最大空闲连接数
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`  // 连接最大生命周期
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"` // 空闲连接最大时间
}

// GetDSN 生成数据库 DSN 连接字符串
func (c *DatabaseConnection) GetDSN() string {
	if c == nil {
		return ""
	}
	if c.DSN != "" {
		return c.DSN
	}
	switch c.Driver {
	case "mysql":
		port := c.Port
		if port == 0 {
			port = 3306
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			c.User, c.Password, c.Host, port, c.Database)
	case "postgres":
		port := c.Port
		if port == 0 {
			port = 5432
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Host, port, c.User, c.Password, c.Database)
	case "sqlite":
		if c.Path != "" {
			return c.Path
		}
		return "./data/llmproxy.db"
	default:
		return ""
	}
}

// CacheConnection 缓存连接配置
type CacheConnection struct {
	Name         string        `yaml:"name"`           // 连接名称
	Enabled      bool          `yaml:"enabled"`        // 是否启用
	Driver       string        `yaml:"driver"`         // 驱动: redis / memory
	Addr         string        `yaml:"addr"`           // Redis 地址
	Password     string        `yaml:"password"`       // Redis 密码
	DB           int           `yaml:"db"`             // Redis 数据库编号
	PoolSize     int           `yaml:"pool_size"`      // 连接池大小
	MinIdleConns int           `yaml:"min_idle_conns"` // 最小空闲连接
	DialTimeout  time.Duration `yaml:"dial_timeout"`   // 连接超时
	ReadTimeout  time.Duration `yaml:"read_timeout"`   // 读取超时
	WriteTimeout time.Duration `yaml:"write_timeout"`  // 写入超时
	MaxSize      int           `yaml:"max_size"`       // 内存缓存最大条目
	TTL          time.Duration `yaml:"ttl"`            // 内存缓存默认 TTL
}

// GetDatabase 根据名称获取数据库连接
func (s *StorageConfig) GetDatabase(name string) *DatabaseConnection {
	if s == nil {
		return nil
	}
	for _, db := range s.Databases {
		if db.Name == name {
			return db
		}
	}
	return nil
}

// GetCache 根据名称获取缓存连接
func (s *StorageConfig) GetCache(name string) *CacheConnection {
	if s == nil {
		return nil
	}
	for _, cache := range s.Caches {
		if cache.Name == name {
			return cache
		}
	}
	return nil
}

// ============================================================
//                    后端服务配置
// ============================================================

// Backend 后端服务器配置
type Backend struct {
	Name           string            `yaml:"name"`            // 后端名称
	URL            string            `yaml:"url"`             // 后端服务器 URL
	Weight         int               `yaml:"weight"`          // 权重（用于负载均衡）
	Timeout        time.Duration     `yaml:"timeout"`         // 请求超时
	ConnectTimeout time.Duration     `yaml:"connect_timeout"` // 连接超时
	MaxIdleConns   int               `yaml:"max_idle_conns"`  // 最大空闲连接
	Headers        map[string]string `yaml:"headers"`         // 自定义请求头
}

// ============================================================
//                    服务发现配置
// ============================================================

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	Enabled  bool               `yaml:"enabled"`  // 是否启用
	Mode     string             `yaml:"mode"`     // 模式: merge / first
	Interval time.Duration      `yaml:"interval"` // 全局同步间隔
	Sources  []*DiscoverySource `yaml:"sources"`  // 发现源列表
}

// DiscoverySource 发现源配置
type DiscoverySource struct {
	Name       string                   `yaml:"name"`                 // 源名称
	Type       string                   `yaml:"type"`                 // 类型: database / static / consul / kubernetes / etcd / http
	Enabled    bool                     `yaml:"enabled"`              // 是否启用
	Database   *DiscoveryDatabaseConfig `yaml:"database,omitempty"`   // 数据库配置
	Static     *DiscoveryStaticConfig   `yaml:"static,omitempty"`     // 静态配置
	Consul     *DiscoveryConsulConfig   `yaml:"consul,omitempty"`     // Consul 配置
	Kubernetes *DiscoveryK8sConfig      `yaml:"kubernetes,omitempty"` // Kubernetes 配置
	Etcd       *DiscoveryEtcdConfig     `yaml:"etcd,omitempty"`       // Etcd 配置
	HTTP       *DiscoveryHTTPConfig     `yaml:"http,omitempty"`       // HTTP 配置
	Script     *ScriptConfig            `yaml:"script,omitempty"`     // Lua 后处理脚本
}

// DiscoveryDatabaseConfig 数据库发现配置
type DiscoveryDatabaseConfig struct {
	Storage string            `yaml:"storage"` // 引用 storage.databases[name]
	Table   string            `yaml:"table"`   // 表名
	Fields  map[string]string `yaml:"fields"`  // 字段映射
}

// DiscoveryStaticConfig 静态发现配置
type DiscoveryStaticConfig struct {
	Backends []*Backend `yaml:"backends"` // 静态后端列表
}

// DiscoveryConsulConfig Consul 发现配置
type DiscoveryConsulConfig struct {
	Addr     string        `yaml:"addr"`     // Consul 地址
	Service  string        `yaml:"service"`  // 服务名
	Tag      string        `yaml:"tag"`      // 标签过滤
	Interval time.Duration `yaml:"interval"` // 同步间隔
}

// DiscoveryK8sConfig Kubernetes 发现配置
type DiscoveryK8sConfig struct {
	Namespace     string `yaml:"namespace"`      // 命名空间
	Service       string `yaml:"service"`        // Service 名称
	Port          int    `yaml:"port"`           // 端口
	LabelSelector string `yaml:"label_selector"` // 标签选择器
}

// DiscoveryEtcdConfig Etcd 发现配置
type DiscoveryEtcdConfig struct {
	Endpoints []string `yaml:"endpoints"` // Etcd 端点
	Prefix    string   `yaml:"prefix"`    // Key 前缀
	Username  string   `yaml:"username"`  // 用户名
	Password  string   `yaml:"password"`  // 密码
}

// DiscoveryHTTPConfig HTTP 发现配置
type DiscoveryHTTPConfig struct {
	URL      string            `yaml:"url"`      // URL
	Method   string            `yaml:"method"`   // HTTP 方法
	Interval time.Duration     `yaml:"interval"` // 同步间隔
	Timeout  time.Duration     `yaml:"timeout"`  // 超时
	Headers  map[string]string `yaml:"headers"`  // 请求头
}

// UsageConfig 用量上报配置
type UsageConfig struct {
	Enabled   bool             `yaml:"enabled"`   // 是否启用
	Reporters []*UsageReporter `yaml:"reporters"` // 上报器列表（可配置多个）
}

// UsageReporter 单个用量上报器配置
type UsageReporter struct {
	Name     string               `yaml:"name"`               // 上报器名称
	Type     string               `yaml:"type"`               // 类型：webhook / database / builtin
	Enabled  bool                 `yaml:"enabled"`            // 是否启用
	Webhook  *UsageWebhookConfig  `yaml:"webhook,omitempty"`  // Webhook 配置
	Database *UsageDatabaseConfig `yaml:"database,omitempty"` // 数据库配置
	Builtin  *UsageBuiltinConfig  `yaml:"builtin,omitempty"`  // 内置 SQLite 配置
	Script   *ScriptConfig        `yaml:"script,omitempty"`   // Lua 脚本
}

// UsageWebhookConfig 用量 Webhook 配置
type UsageWebhookConfig struct {
	URL     string            `yaml:"url"`     // Webhook URL
	Method  string            `yaml:"method"`  // HTTP 方法
	Timeout time.Duration     `yaml:"timeout"` // 超时时间
	Retry   int               `yaml:"retry"`   // 重试次数
	Headers map[string]string `yaml:"headers"` // 请求头
}

// UsageDatabaseConfig 用量数据库配置
type UsageDatabaseConfig struct {
	Storage string `yaml:"storage"` // 引用 storage.databases[name]
	Table   string `yaml:"table"`   // 表名
}

// UsageBuiltinConfig 内置用量存储配置
// 使用 admin 模块的 SQLite 数据库存储用量记录
type UsageBuiltinConfig struct {
	RetentionDays int `yaml:"retention_days"` // 数据保留天数，0=永久
}

// ============================================================
//                    健康检查配置
// ============================================================

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled            bool          `yaml:"enabled"`             // 是否启用
	Interval           time.Duration `yaml:"interval"`            // 检查间隔
	Timeout            time.Duration `yaml:"timeout"`             // 超时时间
	Method             string        `yaml:"method"`              // HTTP 方法
	Path               string        `yaml:"path"`                // 健康检查路径
	ExpectedStatus     int           `yaml:"expected_status"`     // 期望状态码
	UnhealthyThreshold int           `yaml:"unhealthy_threshold"` // 不健康阈值
	HealthyThreshold   int           `yaml:"healthy_threshold"`   // 健康阈值
	Script             *ScriptConfig `yaml:"script,omitempty"`    // Lua 脚本
}

// ============================================================
//                    路由配置
// ============================================================

// RoutingConfig 路由配置
type RoutingConfig struct {
	Enabled        bool           `yaml:"enabled"`         // 是否启用
	LoadBalance    string         `yaml:"load_balance"`    // 负载均衡策略
	Timeout        time.Duration  `yaml:"timeout"`         // 总请求超时
	ConnectTimeout time.Duration  `yaml:"connect_timeout"` // 连接超时
	Script         *ScriptConfig  `yaml:"script,omitempty"`
	Retry          *RetryConfig   `yaml:"retry"`
	Fallback       []FallbackRule `yaml:"fallback"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	Enabled     bool          `yaml:"enabled"`
	MaxRetries  int           `yaml:"max_retries"`
	InitialWait time.Duration `yaml:"initial_wait"`
	MaxWait     time.Duration `yaml:"max_wait"`
	Multiplier  float64       `yaml:"multiplier"`
	RetryOn     []string      `yaml:"retry_on"` // 重试条件: 5xx, connect_failure, timeout
}

// FallbackRule 故障转移规则
type FallbackRule struct {
	Models   []string `yaml:"models"`   // 适用的模型列表（空表示所有）
	Primary  string   `yaml:"primary"`  // 主后端
	Fallback []string `yaml:"fallback"` // 备用后端列表
}

// ============================================================
//                    鉴权配置
// ============================================================

// AuthConfig 鉴权配置
type AuthConfig struct {
	Enabled     bool            `yaml:"enabled"`      // 是否启用鉴权
	Mode        string          `yaml:"mode"`         // 管道模式：first_match 或 all
	SkipPaths   []string        `yaml:"skip_paths"`   // 跳过鉴权的路径
	HeaderNames []string        `yaml:"header_names"` // 自定义认证 Header 名称列表
	Pipeline    []*AuthProvider `yaml:"pipeline"`     // 鉴权管道配置
	StatusCodes *StatusCodes    `yaml:"status_codes"` // 状态码配置
}

// StatusCodeConfig 单个状态码配置
type StatusCodeConfig struct {
	Allow    bool   `yaml:"allow"`     // 是否允许通过
	HttpCode int    `yaml:"http_code"` // HTTP 状态码
	Message  string `yaml:"message"`   // 错误消息
}

// StatusCodes 状态码配置（可配置错误消息和 HTTP 状态码）
type StatusCodes struct {
	Active        *StatusCodeConfig `yaml:"active"`         // 正常状态
	Disabled      *StatusCodeConfig `yaml:"disabled"`       // 已禁用
	Expired       *StatusCodeConfig `yaml:"expired"`        // 已过期
	QuotaExceeded *StatusCodeConfig `yaml:"quota_exceeded"` // 额度耗尽
	NotFound      *StatusCodeConfig `yaml:"not_found"`      // 不存在
}

// AuthProvider 鉴权提供者配置
type AuthProvider struct {
	Name     string              `yaml:"name"`               // Provider 名称
	Type     string              `yaml:"type"`               // Provider 类型: builtin / redis / database / webhook / lua / static
	Enabled  bool                `yaml:"enabled"`            // 是否启用
	Redis    *RedisAuthConfig    `yaml:"redis,omitempty"`    // Redis 配置
	Database *DatabaseAuthConfig `yaml:"database,omitempty"` // 数据库配置
	Webhook  *WebhookAuthConfig  `yaml:"webhook,omitempty"`  // Webhook 配置
	Lua      *LuaAuthConfig      `yaml:"lua,omitempty"`      // Lua 脚本配置
	Static   *StaticAuthConfig   `yaml:"static,omitempty"`   // 静态配置
	Script   *ScriptConfig       `yaml:"script,omitempty"`   // Lua 后处理脚本
}

// RedisAuthConfig Redis 鉴权配置
type RedisAuthConfig struct {
	Storage    string `yaml:"storage"`     // 引用 storage.caches[name]
	KeyPattern string `yaml:"key_pattern"` // Key 模式
}

// DatabaseAuthConfig 数据库鉴权配置
type DatabaseAuthConfig struct {
	Storage   string   `yaml:"storage"`    // 引用 storage.databases[name]
	Table     string   `yaml:"table"`      // 表名
	KeyColumn string   `yaml:"key_column"` // API Key 列名
	Fields    []string `yaml:"fields"`     // 需要查询的字段列表
}

// WebhookAuthConfig Webhook 鉴权配置
type WebhookAuthConfig struct {
	URL     string            `yaml:"url"`     // Webhook URL
	Method  string            `yaml:"method"`  // HTTP 方法
	Timeout time.Duration     `yaml:"timeout"` // 超时时间
	Headers map[string]string `yaml:"headers"` // 自定义请求头
}

// LuaAuthConfig Lua 脚本鉴权配置
type LuaAuthConfig struct {
	Path      string        `yaml:"path"`       // 脚本文件路径
	Script    string        `yaml:"script"`     // 内联脚本
	Timeout   time.Duration `yaml:"timeout"`    // 超时时间
	MaxMemory int           `yaml:"max_memory"` // 最大内存 MB
}

// StaticAuthConfig 静态鉴权配置
type StaticAuthConfig struct {
	Keys []*APIKey `yaml:"keys"` // 静态 API Key 列表
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

// ============================================================
//                    限流配置
// ============================================================

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled bool          `yaml:"enabled"`
	Storage string        `yaml:"storage"` // memory / redis
	Redis   string        `yaml:"redis"`   // Redis 缓存名称（引用 storage.caches[name]）
	Script  *ScriptConfig `yaml:"script,omitempty"`
	Global  *GlobalLimit  `yaml:"global"`
	PerKey  *KeyLimit     `yaml:"per_key"`
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

// ============================================================
//                    请求/访问日志配置
// ============================================================

// LoggingConfig 请求日志配置
type LoggingConfig struct {
	Enabled bool                  `yaml:"enabled"`
	Request *RequestLoggingConfig `yaml:"request"`
	Access  *AccessLoggingConfig  `yaml:"access"`
}

// RequestLoggingConfig 请求日志配置
type RequestLoggingConfig struct {
	Enabled     bool           `yaml:"enabled"`
	Storage     string         `yaml:"storage"`      // 引用 storage.databases[name]
	Table       string         `yaml:"table"`        // 表名（默认 request_logs）
	IncludeBody bool           `yaml:"include_body"` // 是否记录请求/响应体
	Script      *ScriptConfig  `yaml:"script,omitempty"`
	File        *LogFileConfig `yaml:"file,omitempty"`
}

// AccessLoggingConfig 访问日志配置
type AccessLoggingConfig struct {
	Enabled bool           `yaml:"enabled"`
	Format  string         `yaml:"format"` // combined / json（默认 combined）
	Output  string         `yaml:"output"` // file / stdout
	Script  *ScriptConfig  `yaml:"script,omitempty"`
	File    *LogFileConfig `yaml:"file,omitempty"`
}

// ============================================================
//                    指标配置
// ============================================================

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled        bool      `yaml:"enabled"`
	Path           string    `yaml:"path"` // 指标端点路径
	CustomLabels   []string  `yaml:"custom_labels"`
	LatencyBuckets []float64 `yaml:"latency_buckets"`
}

// ============================================================
//                    生命周期钩子配置
// ============================================================

// HooksConfig 生命周期钩子配置
type HooksConfig struct {
	Enabled    bool          `yaml:"enabled"`
	OnRequest  *ScriptConfig `yaml:"on_request,omitempty"`
	OnAuth     *ScriptConfig `yaml:"on_auth,omitempty"`
	OnRoute    *ScriptConfig `yaml:"on_route,omitempty"`
	OnResponse *ScriptConfig `yaml:"on_response,omitempty"`
	OnError    *ScriptConfig `yaml:"on_error,omitempty"`
	OnComplete *ScriptConfig `yaml:"on_complete,omitempty"`
}

// ============================================================
//                    通用脚本配置
// ============================================================

// ScriptConfig 单个脚本配置
type ScriptConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Path      string        `yaml:"path"`       // 脚本文件路径
	Script    string        `yaml:"script"`     // 内联脚本
	Timeout   time.Duration `yaml:"timeout"`    // 超时时间
	MaxMemory int           `yaml:"max_memory"` // 最大内存 MB
}

// ============================================================
//                    主配置结构
// ============================================================

// AdminConfig Admin API 配置
type AdminConfig struct {
	Enabled bool   `yaml:"enabled"` // 是否启用 Admin API
	Token   string `yaml:"token"`   // 访问令牌
	Listen  string `yaml:"listen"`  // 监听地址（可选，默认与主服务同端口）
	DBPath  string `yaml:"db_path"` // SQLite 数据库路径（默认 ./data/keys.db）
}

// Config 主配置结构
type Config struct {
	Server      *ServerConfig      `yaml:"server"`       // 服务器配置
	Log         *LogConfig         `yaml:"log"`          // 系统日志配置
	Storage     *StorageConfig     `yaml:"storage"`      // 存储配置（多数据源）
	Backends    []*Backend         `yaml:"backends"`     // 后端服务列表
	Discovery   *DiscoveryConfig   `yaml:"discovery"`    // 服务发现配置
	Auth        *AuthConfig        `yaml:"auth"`         // 鉴权配置
	Admin       *AdminConfig       `yaml:"admin"`        // Admin API 配置
	Logging     *LoggingConfig     `yaml:"logging"`      // 请求/访问日志
	RateLimit   *RateLimitConfig   `yaml:"rate_limit"`   // 限流配置
	Routing     *RoutingConfig     `yaml:"routing"`      // 路由配置
	HealthCheck *HealthCheckConfig `yaml:"health_check"` // 健康检查配置
	Metrics     *MetricsConfig     `yaml:"metrics"`      // 指标配置
	Usage       *UsageConfig       `yaml:"usage"`        // 用量上报配置
	Hooks       *HooksConfig       `yaml:"hooks"`        // 生命周期钩子

	// 兼容旧配置（已废弃）
	Listen string `yaml:"listen"` // 已废弃，请使用 server.listen
}

// GetListen 获取监听地址（兼容旧配置）
func (c *Config) GetListen() string {
	if c.Server != nil && c.Server.Listen != "" {
		return c.Server.Listen
	}
	if c.Listen != "" {
		return c.Listen
	}
	return ":8000"
}

// Load 从文件加载配置
// 参数：
//   - path: 配置文件路径
//
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

	// 设置服务器默认值
	if cfg.Server == nil {
		cfg.Server = &ServerConfig{}
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = cfg.GetListen()
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 60 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}
	if cfg.Server.MaxHeaderBytes == 0 {
		cfg.Server.MaxHeaderBytes = 1 << 20 // 1MB
	}
	if cfg.Server.MaxBodySize == 0 {
		cfg.Server.MaxBodySize = 10 << 20 // 10MB
	}

	// 设置日志默认值
	if cfg.Log == nil {
		cfg.Log = &LogConfig{}
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Log.Output == "" {
		cfg.Log.Output = "stdout"
	}

	// 健康检查默认值
	if cfg.HealthCheck != nil {
		if cfg.HealthCheck.Interval == 0 {
			cfg.HealthCheck.Interval = 30 * time.Second
		}
		if cfg.HealthCheck.Timeout == 0 {
			cfg.HealthCheck.Timeout = 5 * time.Second
		}
		if cfg.HealthCheck.Path == "" {
			cfg.HealthCheck.Path = "/health"
		}
		if cfg.HealthCheck.Method == "" {
			cfg.HealthCheck.Method = "GET"
		}
		if cfg.HealthCheck.ExpectedStatus == 0 {
			cfg.HealthCheck.ExpectedStatus = 200
		}
		if cfg.HealthCheck.UnhealthyThreshold == 0 {
			cfg.HealthCheck.UnhealthyThreshold = 3
		}
		if cfg.HealthCheck.HealthyThreshold == 0 {
			cfg.HealthCheck.HealthyThreshold = 2
		}
	}

	// 路由配置默认值
	if cfg.Routing != nil {
		if cfg.Routing.LoadBalance == "" {
			cfg.Routing.LoadBalance = "round_robin"
		}
		if cfg.Routing.Timeout == 0 {
			cfg.Routing.Timeout = 60 * time.Second
		}
		if cfg.Routing.ConnectTimeout == 0 {
			cfg.Routing.ConnectTimeout = 5 * time.Second
		}
		if cfg.Routing.Retry != nil && cfg.Routing.Retry.Enabled {
			if cfg.Routing.Retry.MaxRetries == 0 {
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
		}
	}

	// 鉴权配置默认值
	if cfg.Auth != nil && cfg.Auth.Enabled {
		if cfg.Auth.Mode == "" {
			cfg.Auth.Mode = "first_match"
		}
		// 状态码配置默认值
		if cfg.Auth.StatusCodes == nil {
			cfg.Auth.StatusCodes = &StatusCodes{}
		}
		if cfg.Auth.StatusCodes.Active == nil {
			cfg.Auth.StatusCodes.Active = &StatusCodeConfig{Allow: true}
		}
		if cfg.Auth.StatusCodes.Disabled == nil {
			cfg.Auth.StatusCodes.Disabled = &StatusCodeConfig{Allow: false, HttpCode: 403, Message: "API Key 已被禁用"}
		}
		if cfg.Auth.StatusCodes.Expired == nil {
			cfg.Auth.StatusCodes.Expired = &StatusCodeConfig{Allow: false, HttpCode: 403, Message: "API Key 已过期"}
		}
		if cfg.Auth.StatusCodes.QuotaExceeded == nil {
			cfg.Auth.StatusCodes.QuotaExceeded = &StatusCodeConfig{Allow: false, HttpCode: 429, Message: "额度已用尽，请充值"}
		}
		if cfg.Auth.StatusCodes.NotFound == nil {
			cfg.Auth.StatusCodes.NotFound = &StatusCodeConfig{Allow: false, HttpCode: 401, Message: "无效的 API Key"}
		}
	}

	// Admin API 配置默认值
	if cfg.Admin != nil && cfg.Admin.Enabled {
		if cfg.Admin.DBPath == "" {
			cfg.Admin.DBPath = "./data/keys.db"
		}
	}

	// 限流配置默认值
	if cfg.RateLimit != nil && cfg.RateLimit.Enabled {
		if cfg.RateLimit.Storage == "" {
			cfg.RateLimit.Storage = "memory"
		}
	}

	// 指标配置默认值
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		if cfg.Metrics.Path == "" {
			cfg.Metrics.Path = "/metrics"
		}
	}

	// 服务发现默认值
	if cfg.Discovery != nil && cfg.Discovery.Enabled {
		if cfg.Discovery.Mode == "" {
			cfg.Discovery.Mode = "merge"
		}
		if cfg.Discovery.Interval == 0 {
			cfg.Discovery.Interval = 30 * time.Second
		}
	}

	return &cfg, nil
}
