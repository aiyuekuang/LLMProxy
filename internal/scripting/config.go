package scripting

import (
	"time"
)

// ScriptsConfig Lua 脚本配置
type ScriptsConfig struct {
	Routing          *ScriptConfig `yaml:"routing"`           // 路由决策脚本
	Auth             *ScriptConfig `yaml:"auth"`              // 鉴权决策脚本
	RequestTransform *ScriptConfig `yaml:"request_transform"` // 请求转换脚本
	ResponseTransform *ScriptConfig `yaml:"response_transform"` // 响应转换脚本
	RateLimit        *ScriptConfig `yaml:"rate_limit"`        // 限流决策脚本
	Usage            *ScriptConfig `yaml:"usage"`             // 用量计算脚本
	ErrorHandler     *ScriptConfig `yaml:"error_handler"`     // 错误处理脚本
}

// ScriptConfig 单个脚本配置
type ScriptConfig struct {
	Enabled    bool          `yaml:"enabled"`     // 是否启用
	Script     string        `yaml:"script"`      // 脚本内容（内联）
	ScriptFile string        `yaml:"script_file"` // 脚本文件路径
	Timeout    time.Duration `yaml:"timeout"`     // 执行超时时间
	MaxMemory  int           `yaml:"max_memory"`  // 最大内存限制
}

// ToEngineConfig 转换为引擎配置
// 返回：
//   - *EngineConfig: 引擎配置
func (c *ScriptConfig) ToEngineConfig() *EngineConfig {
	return &EngineConfig{
		Script:     c.Script,
		ScriptFile: c.ScriptFile,
		Timeout:    c.Timeout,
		MaxMemory:  c.MaxMemory,
	}
}
