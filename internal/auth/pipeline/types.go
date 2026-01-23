package pipeline

import (
	"time"

	"github.com/redis/go-redis/v9"
	"llmproxy/internal/config"
	"llmproxy/internal/types"
)

// RedisClient Redis 客户端类型别名
type RedisClient = *redis.Client

// ProviderType 鉴权提供者类型
type ProviderType string

const (
	ProviderTypeFile     ProviderType = "file"     // 配置文件
	ProviderTypeRedis    ProviderType = "redis"    // Redis
	ProviderTypeDatabase ProviderType = "database" // 数据库
	ProviderTypeWebhook  ProviderType = "webhook"  // Webhook
	ProviderTypeLua      ProviderType = "lua"      // Lua 脚本
	ProviderTypeStatic   ProviderType = "static"   // 静态 API Keys
	ProviderTypeBuiltin  ProviderType = "builtin"  // 内置 SQLite 存储
)

// KeyStatus 别名，方便引用
type KeyStatus = types.KeyStatus

// 状态常量别名
const (
	KeyStatusActive        = types.KeyStatusActive
	KeyStatusDisabled      = types.KeyStatusDisabled
	KeyStatusQuotaExceeded = types.KeyStatusQuotaExceeded
	KeyStatusExpired       = types.KeyStatusExpired
)

// PipelineMode 管道执行模式
type PipelineMode string

const (
	PipelineModeFirstMatch PipelineMode = "first_match" // 第一个成功即放行
	PipelineModeAll        PipelineMode = "all"         // 全部通过才放行
)

// AuthResult Lua 脚本返回的鉴权结果
type AuthResult struct {
	Allow      bool                   `json:"allow"`              // 是否允许
	Message    string                 `json:"message,omitempty"`  // 拒绝时的错误消息
	StatusCode int                    `json:"status_code"`        // HTTP 状态码
	StatusName string                 `json:"status_name"`        // 状态名称（如 DISABLED, EXPIRED）
	Metadata   map[string]interface{} `json:"metadata,omitempty"` // 附加元数据
}

// ProviderResult Provider 查询结果
type ProviderResult struct {
	Found bool                   // 是否找到数据
	Data  map[string]interface{} // 查询到的数据
	Error error                  // 错误信息
}

// AuthContext 鉴权上下文，传递给 Lua 脚本
type AuthContext struct {
	APIKey   string                 // 当前请求的 API Key
	Data     map[string]interface{} // 从 Provider 查询到的数据
	Request  *RequestInfo           // 请求信息
	Metadata map[string]interface{} // 累积的元数据
}

// RequestInfo 请求信息
type RequestInfo struct {
	Method  string            // HTTP 方法
	Path    string            // 请求路径
	Headers map[string]string // 请求头
	IP      string            // 客户端 IP
}

// ErrorDetail 错误详情（嵌套结构）
type ErrorDetail struct {
	Code    string `json:"code"`    // 状态码名称（如 DISABLED, EXPIRED）
	Message string `json:"message"` // 错误消息
}

// ErrorResponse 错误响应结构
// 格式: {"error": {"code": "DISABLED", "message": "API Key 已被禁用"}}
type ErrorResponse struct {
	Error ErrorDetail `json:"error"` // 错误详情
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Storage    string `yaml:"storage"`     // 引用 storage.caches[name]
	KeyPattern string `yaml:"key_pattern"` // Key 模式，如 "llmproxy:key:{api_key}"
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Storage   string   `yaml:"storage"`    // 引用 storage.databases[name]
	Table     string   `yaml:"table"`      // 表名
	KeyColumn string   `yaml:"key_column"` // API Key 列名
	Fields    []string `yaml:"fields"`     // 需要查询的字段列表
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	URL     string            `yaml:"url"`     // Webhook URL
	Method  string            `yaml:"method"`  // HTTP 方法，默认 POST
	Timeout time.Duration     `yaml:"timeout"` // 超时时间
	Headers map[string]string `yaml:"headers"` // 自定义请求头
}

// ProviderConfig 单个 Provider 配置
type ProviderConfig struct {
	Name          string           `yaml:"name"`               // Provider 名称
	Type          ProviderType     `yaml:"type"`               // Provider 类型
	Enabled       bool             `yaml:"enabled"`            // 是否启用
	Redis         *RedisConfig     `yaml:"redis,omitempty"`    // Redis 配置
	Database      *DatabaseConfig  `yaml:"database,omitempty"` // 数据库配置
	Webhook       *WebhookConfig   `yaml:"webhook,omitempty"`  // Webhook 配置
	StaticKeys    []*config.APIKey `yaml:"static,omitempty"`   // 静态 API Keys
	LuaScript     string           `yaml:"lua_script"`         // Lua 脚本内容
	LuaScriptFile string           `yaml:"lua_script_file"`    // Lua 脚本文件路径
}

// PipelineConfig 鉴权管道配置
type PipelineConfig struct {
	Enabled     bool              `yaml:"enabled"`      // 是否启用管道鉴权
	HeaderNames []string          `yaml:"header_names"` // 自定义认证 Header 名称列表
	SkipPaths   []string          `yaml:"skip_paths"`   // 跳过鉴权的路径前缀
	Mode        PipelineMode      `yaml:"mode"`         // 管道模式：first_match 或 all
	Providers   []*ProviderConfig `yaml:"pipeline"`     // Provider 列表（按顺序执行）
}
