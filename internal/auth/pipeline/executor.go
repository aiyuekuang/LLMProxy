package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"llmproxy/internal/config"
)

// Executor 管道执行器
// 负责按顺序执行鉴权管道中的各个 Provider
type Executor struct {
	config      *PipelineConfig        // 管道配置
	providers   []providerWithConfig   // Provider 列表
	luaExecutor *LuaExecutor           // Lua 执行器
}

// providerWithConfig Provider 及其配置
type providerWithConfig struct {
	provider Provider        // Provider 实例
	config   *ProviderConfig // Provider 配置
}

// NewExecutor 创建管道执行器
// 参数：
//   - cfg: 管道配置
//   - apiKeys: API Key 列表（用于 file provider）
// 返回：
//   - *Executor: 执行器实例
//   - error: 错误信息
func NewExecutor(cfg *PipelineConfig, apiKeys []*config.APIKey) (*Executor, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	executor := &Executor{
		config:      cfg,
		providers:   make([]providerWithConfig, 0),
		luaExecutor: NewLuaExecutor(),
	}

	// 初始化各个 Provider
	for _, providerCfg := range cfg.Providers {
		if !providerCfg.Enabled {
			continue
		}

		provider, err := executor.createProvider(providerCfg, apiKeys)
		if err != nil {
			return nil, fmt.Errorf("创建 Provider [%s] 失败: %w", providerCfg.Name, err)
		}

		executor.providers = append(executor.providers, providerWithConfig{
			provider: provider,
			config:   providerCfg,
		})

		log.Printf("鉴权管道: 已加载 Provider [%s] (类型: %s)", providerCfg.Name, providerCfg.Type)
	}

	if len(executor.providers) == 0 {
		return nil, fmt.Errorf("鉴权管道: 没有启用的 Provider")
	}

	log.Printf("鉴权管道: 已加载 %d 个 Provider, 模式: %s", len(executor.providers), cfg.Mode)

	return executor, nil
}

// createProvider 创建 Provider 实例
func (e *Executor) createProvider(cfg *ProviderConfig, apiKeys []*config.APIKey) (Provider, error) {
	switch cfg.Type {
	case ProviderTypeFile:
		return NewFileProvider(cfg.Name, apiKeys), nil

	case ProviderTypeRedis:
		return NewRedisProvider(cfg.Name, cfg.Redis)

	case ProviderTypeDatabase:
		return NewDatabaseProvider(cfg.Name, cfg.Database)

	case ProviderTypeWebhook:
		return NewWebhookProvider(cfg.Name, cfg.Webhook)

	default:
		return nil, fmt.Errorf("未知的 Provider 类型: %s", cfg.Type)
	}
}

// Execute 执行鉴权管道
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
//   - requestInfo: 请求信息
// 返回：
//   - *AuthResult: 鉴权结果
//   - error: 错误信息
func (e *Executor) Execute(ctx context.Context, apiKey string, requestInfo *RequestInfo) (*AuthResult, error) {
	// 累积的元数据
	metadata := make(map[string]interface{})

	// 记录是否有任何 Provider 成功匹配
	anyMatched := false

	for _, pwc := range e.providers {
		// 查询 Provider
		result := pwc.provider.Query(ctx, apiKey)

		// 处理查询错误
		if result.Error != nil {
			log.Printf("鉴权管道: Provider [%s] 查询错误: %v", pwc.provider.Name(), result.Error)
			// 继续下一个 Provider
			continue
		}

		// 如果没有找到数据
		if !result.Found {
			log.Printf("鉴权管道: Provider [%s] 未找到 Key", pwc.provider.Name())
			// 继续下一个 Provider
			continue
		}

		anyMatched = true

		// 执行 Lua 脚本（如果有）
		luaResult, err := e.executeLuaScript(pwc.config, &AuthContext{
			APIKey:   apiKey,
			Data:     result.Data,
			Request:  requestInfo,
			Metadata: metadata,
		})

		if err != nil {
			log.Printf("鉴权管道: Provider [%s] Lua 脚本执行错误: %v", pwc.provider.Name(), err)
			return &AuthResult{
				Allow:   false,
				Message: fmt.Sprintf("鉴权脚本执行错误: %v", err),
			}, nil
		}

		// 合并元数据
		for k, v := range luaResult.Metadata {
			metadata[k] = v
		}

		// 根据管道模式处理结果
		switch e.config.Mode {
		case PipelineModeFirstMatch:
			// first_match 模式：Lua 返回 allow=true 即放行
			if luaResult.Allow {
				log.Printf("鉴权管道: Provider [%s] 验证通过 (first_match)", pwc.provider.Name())
				luaResult.Metadata = metadata
				return luaResult, nil
			}
			// Lua 返回 allow=false，立即拒绝
			log.Printf("鉴权管道: Provider [%s] 验证拒绝: %s", pwc.provider.Name(), luaResult.Message)
			return luaResult, nil

		case PipelineModeAll:
			// all 模式：任何一个 Lua 返回 allow=false 即拒绝
			if !luaResult.Allow {
				log.Printf("鉴权管道: Provider [%s] 验证拒绝: %s", pwc.provider.Name(), luaResult.Message)
				return luaResult, nil
			}
			log.Printf("鉴权管道: Provider [%s] 验证通过，继续下一个", pwc.provider.Name())
		}
	}

	// 根据模式返回最终结果
	if e.config.Mode == PipelineModeAll && anyMatched {
		// all 模式下，所有都通过
		return &AuthResult{
			Allow:    true,
			Metadata: metadata,
		}, nil
	}

	// 没有任何 Provider 匹配到
	return &AuthResult{
		Allow:   false,
		Message: "API Key 无效",
	}, nil
}

// executeLuaScript 执行 Lua 脚本
func (e *Executor) executeLuaScript(cfg *ProviderConfig, ctx *AuthContext) (*AuthResult, error) {
	// 如果没有配置 Lua 脚本，使用默认逻辑
	if cfg.LuaScript == "" && cfg.LuaScriptFile == "" {
		return e.defaultAuthLogic(ctx)
	}

	// 从文件加载脚本
	if cfg.LuaScriptFile != "" {
		return e.luaExecutor.ExecuteFile(cfg.LuaScriptFile, ctx)
	}

	// 执行内联脚本
	return e.luaExecutor.Execute(cfg.LuaScript, ctx)
}

// defaultAuthLogic 默认鉴权逻辑（无 Lua 脚本时使用）
func (e *Executor) defaultAuthLogic(ctx *AuthContext) (*AuthResult, error) {
	data := ctx.Data

	// 检查 status 字段
	if status, ok := data["status"].(string); ok {
		if status != "active" {
			return &AuthResult{
				Allow:   false,
				Message: "API Key 已禁用",
			}, nil
		}
	}

	// 检查额度
	if totalQuota, ok := e.getInt64(data, "total_quota"); ok && totalQuota > 0 {
		usedQuota, _ := e.getInt64(data, "used_quota")
		if usedQuota >= totalQuota {
			return &AuthResult{
				Allow:   false,
				Message: "额度不足",
			}, nil
		}
	}

	// 检查余额
	if balance, ok := e.getFloat64(data, "balance"); ok {
		if balance <= 0 {
			return &AuthResult{
				Allow:   false,
				Message: "余额不足",
			}, nil
		}
	}

	return &AuthResult{Allow: true}, nil
}

// getInt64 从 map 中获取 int64 值
func (e *Executor) getInt64(data map[string]interface{}, key string) (int64, bool) {
	v, ok := data[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	case string:
		var i int64
		fmt.Sscanf(val, "%d", &i)
		return i, true
	}
	return 0, false
}

// getFloat64 从 map 中获取 float64 值
func (e *Executor) getFloat64(data map[string]interface{}, key string) (float64, bool) {
	v, ok := data[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int64:
		return float64(val), true
	case int:
		return float64(val), true
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f, true
	}
	return 0, false
}

// WriteErrorResponse 写入错误响应（JSON 格式）
// 参数：
//   - w: HTTP 响应写入器
//   - result: 鉴权结果
//   - statusCode: HTTP 状态码
func WriteErrorResponse(w http.ResponseWriter, result *AuthResult, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := ErrorResponse{
		Error: result.Message,
		Code:  statusCode,
	}

	json.NewEncoder(w).Encode(resp)
}

// Close 关闭所有 Provider
func (e *Executor) Close() error {
	for _, pwc := range e.providers {
		if err := pwc.provider.Close(); err != nil {
			log.Printf("关闭 Provider [%s] 失败: %v", pwc.provider.Name(), err)
		}
	}
	if e.luaExecutor != nil {
		e.luaExecutor.Close()
	}
	return nil
}

// GetHeaderNames 获取认证 Header 名称列表
func (e *Executor) GetHeaderNames() []string {
	return e.config.HeaderNames
}
