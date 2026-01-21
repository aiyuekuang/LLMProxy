package pipeline

import (
	"context"
	"fmt"
	"os"
)

// LuaProvider Lua 脚本鉴权 Provider
type LuaProvider struct {
	name       string
	script     string
	scriptFile string
	executor   *LuaExecutor
}

// NewLuaProvider 创建 Lua Provider
func NewLuaProvider(name, script, scriptFile string) (*LuaProvider, error) {
	p := &LuaProvider{
		name:       name,
		script:     script,
		scriptFile: scriptFile,
		executor:   NewLuaExecutor(),
	}

	// 验证脚本配置
	if script == "" && scriptFile == "" {
		return nil, fmt.Errorf("Lua Provider 需要 script 或 script_file 配置")
	}

	// 如果是文件，检查文件是否存在
	if scriptFile != "" {
		if _, err := os.Stat(scriptFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("Lua 脚本文件不存在: %s", scriptFile)
		}
	}

	return p, nil
}

// Name 返回 Provider 名称
func (p *LuaProvider) Name() string {
	return p.name
}

// Type 返回 Provider 类型
func (p *LuaProvider) Type() ProviderType {
	return ProviderTypeLua
}

// Query 查询 API Key（Lua Provider 总是返回 Found=true，让脚本自己处理）
func (p *LuaProvider) Query(ctx context.Context, apiKey string) *ProviderResult {
	// Lua Provider 直接返回 Found=true，数据由脚本自己处理
	return &ProviderResult{
		Found: true,
		Data: map[string]interface{}{
			"api_key": apiKey,
		},
	}
}

// Close 关闭 Provider
func (p *LuaProvider) Close() error {
	if p.executor != nil {
		p.executor.Close()
	}
	return nil
}
