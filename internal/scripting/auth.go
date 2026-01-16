package scripting

import (
	"log"

	"llmproxy/internal/auth"

	lua "github.com/yuin/gopher-lua"
)

// AuthScript 鉴权脚本执行器
type AuthScript struct {
	engine *Engine
}

// AuthResult 鉴权结果
type AuthResult struct {
	Allow      bool   // 是否允许访问
	Reason     string // 拒绝原因
	StatusCode int    // HTTP 状态码
}

// NewAuthScript 创建鉴权脚本执行器
// 参数：
//   - config: 引擎配置
// 返回：
//   - *AuthScript: 鉴权脚本执行器
//   - error: 错误信息
func NewAuthScript(config *EngineConfig) (*AuthScript, error) {
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	return &AuthScript{
		engine: engine,
	}, nil
}

// CheckAuth 执行鉴权脚本
// 参数：
//   - requestBody: 请求体
//   - userID: 用户 ID
//   - apiKey: API Key
//   - clientIP: 客户端 IP
//   - requestID: 请求 ID
//   - keyInfo: API Key 信息
//   - standardChecks: 标准检查结果
// 返回：
//   - *AuthResult: 鉴权结果
//   - error: 错误信息
func (a *AuthScript) CheckAuth(
	requestBody map[string]interface{},
	userID string,
	apiKey string,
	clientIP string,
	requestID string,
	keyInfo *auth.APIKey,
	standardChecks map[string]bool,
) (*AuthResult, error) {
	// 从池中获取 VM
	vmInterface := a.engine.vmPool.Get()
	if vmInterface == nil {
		return nil, nil
	}
	vm := vmInterface.(*lua.LState)
	defer a.engine.vmPool.Put(vm)

	// 构造 request 表
	requestTable := vm.NewTable()
	requestTable.RawSetString("id", lua.LString(requestID))
	requestTable.RawSetString("body", MapToLuaTable(vm, requestBody))
	requestTable.RawSetString("user_id", lua.LString(userID))
	requestTable.RawSetString("api_key", lua.LString(apiKey))
	requestTable.RawSetString("client_ip", lua.LString(clientIP))
	vm.SetGlobal("request", requestTable)

	// 构造 key_info 表
	keyInfoTable := vm.NewTable()
	keyInfoTable.RawSetString("key", lua.LString(keyInfo.Key))
	keyInfoTable.RawSetString("user_id", lua.LString(keyInfo.UserID))
	keyInfoTable.RawSetString("name", lua.LString(keyInfo.Name))
	keyInfoTable.RawSetString("status", lua.LString(keyInfo.Status))
	keyInfoTable.RawSetString("total_quota", lua.LNumber(keyInfo.TotalQuota))
	keyInfoTable.RawSetString("used_quota", lua.LNumber(keyInfo.UsedQuota))

	// 添加 allowed_ips
	allowedIPsTable := vm.NewTable()
	for i, ip := range keyInfo.AllowedIPs {
		allowedIPsTable.RawSetInt(i+1, lua.LString(ip))
	}
	keyInfoTable.RawSetString("allowed_ips", allowedIPsTable)

	// 添加 expires_at
	if keyInfo.ExpiresAt != nil {
		keyInfoTable.RawSetString("expires_at", lua.LString(keyInfo.ExpiresAt.Format("2006-01-02T15:04:05Z")))
	}

	vm.SetGlobal("key_info", keyInfoTable)

	// 构造 standard_checks 表
	standardChecksTable := vm.NewTable()
	for k, v := range standardChecks {
		standardChecksTable.RawSetString(k, lua.LBool(v))
	}
	vm.SetGlobal("standard_checks", standardChecksTable)

	// 执行脚本
	if err := vm.DoString(a.engine.script); err != nil {
		log.Printf("鉴权脚本执行失败: %v", err)
		return nil, err
	}

	// 获取返回值
	result := vm.Get(-1)
	vm.Pop(1)

	// 如果返回 nil，使用标准鉴权结果
	if result.Type() == lua.LTNil {
		return nil, nil
	}

	// 解析返回值
	if table, ok := result.(*lua.LTable); ok {
		authResult := &AuthResult{
			Allow:      true,
			StatusCode: 403,
		}

		// 获取 allow 字段
		if allow := table.RawGetString("allow"); allow.Type() == lua.LTBool {
			authResult.Allow = bool(allow.(lua.LBool))
		}

		// 获取 reason 字段
		if reason := table.RawGetString("reason"); reason.Type() == lua.LTString {
			authResult.Reason = string(reason.(lua.LString))
		}

		// 获取 status_code 字段
		if statusCode := table.RawGetString("status_code"); statusCode.Type() == lua.LTNumber {
			authResult.StatusCode = int(statusCode.(lua.LNumber))
		}

		log.Printf("鉴权脚本结果: allow=%v, reason=%s", authResult.Allow, authResult.Reason)
		return authResult, nil
	}

	return nil, nil
}

// Close 关闭鉴权脚本执行器
func (a *AuthScript) Close() {
	if a.engine != nil {
		a.engine.Close()
	}
}
