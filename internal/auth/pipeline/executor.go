package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/admin"
	"llmproxy/internal/config"
)

// Executor 管道执行器
// 负责按顺序执行鉴权管道中的各个 Provider
type Executor struct {
	config      *PipelineConfig      // 管道配置
	providers   []providerWithConfig // Provider 列表
	luaExecutor *LuaExecutor         // Lua 执行器
	keyStore    *admin.KeyStore      // KeyStore 实例（用于 builtin provider）
	statusCodes *config.StatusCodes  // 状态码配置
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
//
// 返回：
//   - *Executor: 执行器实例
//   - error: 错误信息
func NewExecutor(cfg *PipelineConfig, apiKeys []*config.APIKey) (*Executor, error) {
	return NewExecutorWithStorage(cfg, nil, apiKeys, nil, nil)
}

// NewExecutorWithStorage 创建带存储管理器的管道执行器
// 参数：
//   - cfg: 管道配置
//   - storageManager: 存储管理器
//   - apiKeys: API Key 列表（用于 file provider）
//   - keyStore: KeyStore 实例（用于 builtin provider）
//   - statusCodes: 状态码配置（新）
//
// 返回：
//   - *Executor: 执行器实例
//   - error: 错误信息
func NewExecutorWithStorage(cfg *PipelineConfig, storageManager interface{}, apiKeys []*config.APIKey, keyStore *admin.KeyStore, statusCodes *config.StatusCodes) (*Executor, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	executor := &Executor{
		config:      cfg,
		providers:   make([]providerWithConfig, 0),
		luaExecutor: NewLuaExecutor(),
		keyStore:    keyStore,
		statusCodes: statusCodes,
	}

	// 初始化各个 Provider
	for _, providerCfg := range cfg.Providers {
		if !providerCfg.Enabled {
			continue
		}

		provider, err := executor.createProviderWithStorage(providerCfg, storageManager, apiKeys)
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

// createProviderWithStorage 创建带存储管理器的 Provider 实例
func (e *Executor) createProviderWithStorage(cfg *ProviderConfig, storageManager interface{}, apiKeys []*config.APIKey) (Provider, error) {
	switch cfg.Type {
	case ProviderTypeFile:
		return NewFileProvider(cfg.Name, apiKeys), nil

	case ProviderTypeStatic:
		// 静态配置使用 StaticKeys
		return NewFileProvider(cfg.Name, cfg.StaticKeys), nil

	case ProviderTypeRedis:
		if cfg.Redis == nil {
			return nil, fmt.Errorf("redis provider 配置为空")
		}
		if cfg.Redis.Storage == "" {
			return nil, fmt.Errorf("redis provider 需要配置 storage 引用")
		}
		if storageManager == nil {
			return nil, fmt.Errorf("redis provider 需要 StorageManager")
		}
		sm, ok := storageManager.(interface{ GetCache(string) interface{} })
		if !ok {
			return nil, fmt.Errorf("StorageManager 不支持 GetCache")
		}
		cache := sm.GetCache(cfg.Redis.Storage)
		if cache == nil {
			return nil, fmt.Errorf("redis 缓存 [%s] 未找到", cfg.Redis.Storage)
		}
		redisClient, ok := cache.(RedisClient)
		if !ok {
			return nil, fmt.Errorf("无效的 Redis 客户端类型")
		}
		return NewRedisProviderWithClient(cfg.Name, redisClient, cfg.Redis.KeyPattern)

	case ProviderTypeDatabase:
		if cfg.Database == nil {
			return nil, fmt.Errorf("database provider 配置为空")
		}
		if cfg.Database.Storage == "" {
			return nil, fmt.Errorf("database provider 需要配置 storage 引用")
		}
		if storageManager == nil {
			return nil, fmt.Errorf("database provider 需要 StorageManager")
		}
		sm, ok := storageManager.(interface{ GetDatabase(string) interface{} })
		if !ok {
			return nil, fmt.Errorf("StorageManager 不支持 GetDatabase")
		}
		db := sm.GetDatabase(cfg.Database.Storage)
		if db == nil {
			return nil, fmt.Errorf("数据库 [%s] 未找到", cfg.Database.Storage)
		}
		return NewDatabaseProviderWithDB(cfg.Name, db, cfg.Database)

	case ProviderTypeWebhook:
		return NewWebhookProvider(cfg.Name, cfg.Webhook)

	case ProviderTypeLua:
		// Lua 类型使用脚本执行
		return NewLuaProvider(cfg.Name, cfg.LuaScript, cfg.LuaScriptFile)

	case ProviderTypeBuiltin:
		// Builtin 类型使用 KeyStore
		if e.keyStore == nil {
			return nil, fmt.Errorf("builtin provider 需要 KeyStore 实例，请确保已启用 admin")
		}
		return NewBuiltinProvider(cfg.Name, e.keyStore), nil

	default:
		return nil, fmt.Errorf("未知的 Provider 类型: %s", cfg.Type)
	}
}

// Execute 执行鉴权管道
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
//   - requestInfo: 请求信息
//
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
	return e.buildStatusResult("NOT_FOUND", KeyStatusActive), nil
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

	// 检查 status 字段（支持整数状态码）
	if status, ok := e.getInt64(data, "status"); ok {
		switch KeyStatus(status) {
		case KeyStatusActive:
			// 状态正常，继续检查
		case KeyStatusDisabled:
			return e.buildStatusResult("DISABLED", KeyStatusDisabled), nil
		case KeyStatusQuotaExceeded:
			return e.buildStatusResult("QUOTA_EXCEEDED", KeyStatusQuotaExceeded), nil
		case KeyStatusExpired:
			return e.buildStatusResult("EXPIRED", KeyStatusExpired), nil
		default:
			return e.buildStatusResult("INVALID", KeyStatusDisabled), nil
		}
	} else if status, ok := data["status"].(string); ok {
		// 兼容字符串状态
		switch status {
		case "active":
			// 状态正常，继续检查
		case "disabled":
			return e.buildStatusResult("DISABLED", KeyStatusDisabled), nil
		case "expired":
			return e.buildStatusResult("EXPIRED", KeyStatusExpired), nil
		case "quota_exceeded":
			return e.buildStatusResult("QUOTA_EXCEEDED", KeyStatusQuotaExceeded), nil
		default:
			return e.buildStatusResult("DISABLED", KeyStatusDisabled), nil
		}
	}

	// 检查过期时间（方案 A: LLMProxy 自动判断过期）
	if expiresAt, ok := e.getInt64(data, "expires_at"); ok && expiresAt > 0 {
		if time.Now().Unix() > expiresAt {
			return e.buildStatusResult("EXPIRED", KeyStatusExpired), nil
		}
	}

	// 检查额度
	if totalQuota, ok := e.getInt64(data, "total_quota"); ok && totalQuota > 0 {
		usedQuota, _ := e.getInt64(data, "used_quota")
		if usedQuota >= totalQuota {
			return e.buildStatusResult("QUOTA_EXCEEDED", KeyStatusQuotaExceeded), nil
		}
	}

	// 检查余额
	if balance, ok := e.getFloat64(data, "balance"); ok {
		if balance <= 0 {
			return e.buildStatusResult("QUOTA_EXCEEDED", KeyStatusQuotaExceeded), nil
		}
	}

	return &AuthResult{Allow: true, StatusCode: 200, StatusName: "ACTIVE"}, nil
}

// buildStatusResult 根据状态构建鉴权结果
// 参数：
//   - statusName: 状态名称（如 DISABLED, EXPIRED, QUOTA_EXCEEDED, NOT_FOUND）
//   - keyStatus: 内部状态码
//
// 返回：
//   - *AuthResult: 鉴权结果
func (e *Executor) buildStatusResult(statusName string, keyStatus KeyStatus) *AuthResult {
	// 获取状态码配置
	statusConfig := e.getStatusConfig(statusName)
	if statusConfig == nil {
		// 默认配置
		return &AuthResult{
			Allow:      false,
			Message:    "API Key 无效",
			StatusCode: 401,
			StatusName: statusName,
		}
	}

	return &AuthResult{
		Allow:      statusConfig.Allow,
		Message:    statusConfig.Message,
		StatusCode: statusConfig.HttpCode,
		StatusName: statusName,
	}
}

// getStatusConfig 根据状态名称获取配置
func (e *Executor) getStatusConfig(statusName string) *config.StatusCodeConfig {
	if e.statusCodes == nil {
		return nil
	}

	switch statusName {
	case "ACTIVE":
		return e.statusCodes.Active
	case "DISABLED", "INVALID":
		return e.statusCodes.Disabled
	case "EXPIRED":
		return e.statusCodes.Expired
	case "QUOTA_EXCEEDED":
		return e.statusCodes.QuotaExceeded
	case "NOT_FOUND":
		return e.statusCodes.NotFound
	default:
		return e.statusCodes.Disabled
	}
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
		_, _ = fmt.Sscanf(val, "%d", &i)
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
		_, _ = fmt.Sscanf(val, "%f", &f)
		return f, true
	}
	return 0, false
}

// WriteErrorResponse 写入错误响应（JSON 格式）
// 响应格式: {"error": {"code": "DISABLED", "message": "API Key 已被禁用"}}
// 参数：
//   - w: HTTP 响应写入器
//   - result: 鉴权结果
//   - statusCode: HTTP 状态码（可选，优先使用 result.StatusCode）
func WriteErrorResponse(w http.ResponseWriter, result *AuthResult, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	// 优先使用 result 中的状态码
	httpCode := statusCode
	if result.StatusCode > 0 {
		httpCode = result.StatusCode
	}
	w.WriteHeader(httpCode)

	// 构建嵌套错误响应
	statusName := result.StatusName
	if statusName == "" {
		statusName = "ERROR"
	}

	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    statusName,
			Message: result.Message,
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("写入错误响应失败: %v", err)
	}
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
	if e.config == nil {
		return nil
	}
	return e.config.HeaderNames
}

// ShouldSkip 检查路径是否应跳过鉴权
func (e *Executor) ShouldSkip(path string) bool {
	if e.config == nil || len(e.config.SkipPaths) == 0 {
		return false
	}
	for _, skipPath := range e.config.SkipPaths {
		// 前缀匹配
		if len(path) >= len(skipPath) && path[:len(skipPath)] == skipPath {
			return true
		}
	}
	return false
}
