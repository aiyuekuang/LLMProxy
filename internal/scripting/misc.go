package scripting

import (
	"log"

	lua "github.com/yuin/gopher-lua"
)

// RateLimitScript 限流脚本执行器
type RateLimitScript struct {
	engine *Engine
}

// RateLimitResult 限流结果
type RateLimitResult struct {
	Allow      bool   // 是否允许请求
	Reason     string // 拒绝原因
	RetryAfter int    // 重试等待时间（秒）
}

// NewRateLimitScript 创建限流脚本执行器
// 参数：
//   - config: 引擎配置
// 返回：
//   - *RateLimitScript: 限流脚本执行器
//   - error: 错误信息
func NewRateLimitScript(config *EngineConfig) (*RateLimitScript, error) {
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	return &RateLimitScript{
		engine: engine,
	}, nil
}

// CheckRateLimit 执行限流脚本
// 参数：
//   - requestBody: 请求体
//   - userID: 用户 ID
//   - apiKey: API Key
//   - requestID: 请求 ID
//   - keyInfo: Key 信息
//   - rateLimitStatus: 限流状态
//   - currentTime: 当前时间信息
// 返回：
//   - *RateLimitResult: 限流结果（nil 表示使用标准限流）
//   - error: 错误信息
func (r *RateLimitScript) CheckRateLimit(
	requestBody map[string]interface{},
	userID string,
	apiKey string,
	requestID string,
	keyInfo map[string]interface{},
	rateLimitStatus map[string]interface{},
	currentTime map[string]interface{},
) (*RateLimitResult, error) {
	// 从池中获取 VM
	vmInterface := r.engine.vmPool.Get()
	if vmInterface == nil {
		return nil, nil
	}
	vm := vmInterface.(*lua.LState)
	defer r.engine.vmPool.Put(vm)

	// 设置全局变量
	SetGlobalMap(vm, "request", map[string]interface{}{
		"id":      requestID,
		"body":    requestBody,
		"user_id": userID,
		"api_key": apiKey,
	})
	SetGlobalMap(vm, "key_info", keyInfo)
	SetGlobalMap(vm, "rate_limit_status", rateLimitStatus)
	SetGlobalMap(vm, "current_time", currentTime)

	// 执行脚本
	if err := vm.DoString(r.engine.script); err != nil {
		log.Printf("限流脚本执行失败: %v", err)
		return nil, err
	}

	// 获取返回值
	result := vm.Get(-1)
	vm.Pop(1)

	// 如果返回 nil，使用标准限流
	if result.Type() == lua.LTNil {
		return nil, nil
	}

	// 解析返回值
	if table, ok := result.(*lua.LTable); ok {
		rateLimitResult := &RateLimitResult{
			Allow:      true,
			RetryAfter: 60,
		}

		if allow := table.RawGetString("allow"); allow.Type() == lua.LTBool {
			rateLimitResult.Allow = bool(allow.(lua.LBool))
		}

		if reason := table.RawGetString("reason"); reason.Type() == lua.LTString {
			rateLimitResult.Reason = string(reason.(lua.LString))
		}

		if retryAfter := table.RawGetString("retry_after"); retryAfter.Type() == lua.LTNumber {
			rateLimitResult.RetryAfter = int(retryAfter.(lua.LNumber))
		}

		return rateLimitResult, nil
	}

	return nil, nil
}

// Close 关闭限流脚本执行器
func (r *RateLimitScript) Close() {
	if r.engine != nil {
		r.engine.Close()
	}
}

// UsageScript 用量计算脚本执行器
type UsageScript struct {
	engine *Engine
}

// NewUsageScript 创建用量计算脚本执行器
// 参数：
//   - config: 引擎配置
// 返回：
//   - *UsageScript: 用量计算脚本执行器
//   - error: 错误信息
func NewUsageScript(config *EngineConfig) (*UsageScript, error) {
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	return &UsageScript{
		engine: engine,
	}, nil
}

// CalculateUsage 执行用量计算脚本
// 参数：
//   - requestBody: 请求体
//   - userID: 用户 ID
//   - apiKey: API Key
//   - requestID: 请求 ID
//   - usage: 原始用量数据
//   - responseBody: 响应体
//   - metadata: 元数据
// 返回：
//   - map[string]interface{}: 修改后的用量数据（nil 表示不修改）
//   - error: 错误信息
func (u *UsageScript) CalculateUsage(
	requestBody map[string]interface{},
	userID string,
	apiKey string,
	requestID string,
	usage map[string]interface{},
	responseBody map[string]interface{},
	metadata map[string]interface{},
) (map[string]interface{}, error) {
	// 从池中获取 VM
	vmInterface := u.engine.vmPool.Get()
	if vmInterface == nil {
		return nil, nil
	}
	vm := vmInterface.(*lua.LState)
	defer u.engine.vmPool.Put(vm)

	// 设置全局变量
	SetGlobalMap(vm, "request", map[string]interface{}{
		"id":      requestID,
		"body":    requestBody,
		"user_id": userID,
		"api_key": apiKey,
	})
	SetGlobalMap(vm, "usage", usage)
	SetGlobalMap(vm, "response", map[string]interface{}{
		"body": responseBody,
	})
	SetGlobalMap(vm, "metadata", metadata)

	// 执行脚本
	if err := vm.DoString(u.engine.script); err != nil {
		log.Printf("用量计算脚本执行失败: %v", err)
		return nil, err
	}

	// 获取返回值
	result := vm.Get(-1)
	vm.Pop(1)

	// 如果返回 nil，不修改
	if result.Type() == lua.LTNil {
		return nil, nil
	}

	// 解析返回值
	if table, ok := result.(*lua.LTable); ok {
		return LuaTableToMap(table), nil
	}

	return nil, nil
}

// Close 关闭用量计算脚本执行器
func (u *UsageScript) Close() {
	if u.engine != nil {
		u.engine.Close()
	}
}

// ErrorHandlerScript 错误处理脚本执行器
type ErrorHandlerScript struct {
	engine *Engine
}

// ErrorHandlerResult 错误处理结果
type ErrorHandlerResult struct {
	StatusCode int                    // HTTP 状态码
	Body       map[string]interface{} // 响应体
}

// NewErrorHandlerScript 创建错误处理脚本执行器
// 参数：
//   - config: 引擎配置
// 返回：
//   - *ErrorHandlerScript: 错误处理脚本执行器
//   - error: 错误信息
func NewErrorHandlerScript(config *EngineConfig) (*ErrorHandlerScript, error) {
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	return &ErrorHandlerScript{
		engine: engine,
	}, nil
}

// HandleError 执行错误处理脚本
// 参数：
//   - requestBody: 请求体
//   - userID: 用户 ID
//   - requestID: 请求 ID
//   - errorInfo: 错误信息
// 返回：
//   - *ErrorHandlerResult: 错误处理结果（nil 表示使用默认错误响应）
//   - error: 错误信息
func (e *ErrorHandlerScript) HandleError(
	requestBody map[string]interface{},
	userID string,
	requestID string,
	errorInfo map[string]interface{},
) (*ErrorHandlerResult, error) {
	// 从池中获取 VM
	vmInterface := e.engine.vmPool.Get()
	if vmInterface == nil {
		return nil, nil
	}
	vm := vmInterface.(*lua.LState)
	defer e.engine.vmPool.Put(vm)

	// 设置全局变量
	SetGlobalMap(vm, "request", map[string]interface{}{
		"id":      requestID,
		"body":    requestBody,
		"user_id": userID,
	})
	SetGlobalMap(vm, "error", errorInfo)

	// 执行脚本
	if err := vm.DoString(e.engine.script); err != nil {
		log.Printf("错误处理脚本执行失败: %v", err)
		return nil, err
	}

	// 获取返回值
	result := vm.Get(-1)
	vm.Pop(1)

	// 如果返回 nil，使用默认错误响应
	if result.Type() == lua.LTNil {
		return nil, nil
	}

	// 解析返回值
	if table, ok := result.(*lua.LTable); ok {
		errorResult := &ErrorHandlerResult{
			StatusCode: 500,
		}

		if statusCode := table.RawGetString("status_code"); statusCode.Type() == lua.LTNumber {
			errorResult.StatusCode = int(statusCode.(lua.LNumber))
		}

		if body := table.RawGetString("body"); body.Type() == lua.LTTable {
			errorResult.Body = LuaTableToMap(body.(*lua.LTable))
		}

		return errorResult, nil
	}

	return nil, nil
}

// Close 关闭错误处理脚本执行器
func (e *ErrorHandlerScript) Close() {
	if e.engine != nil {
		e.engine.Close()
	}
}
