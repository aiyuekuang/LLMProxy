package pipeline

import (
	"fmt"
	"os"
	"time"

	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-luar"
)

// LuaExecutor Lua 脚本执行器
type LuaExecutor struct {
	state *lua.LState // Lua 状态机
}

// NewLuaExecutor 创建 Lua 执行器
// 返回：
//   - *LuaExecutor: Lua 执行器实例
func NewLuaExecutor() *LuaExecutor {
	L := lua.NewState()

	// 注册全局函数
	registerGlobalFunctions(L)

	return &LuaExecutor{
		state: L,
	}
}

// registerGlobalFunctions 注册全局函数
func registerGlobalFunctions(L *lua.LState) {
	// now() 返回当前时间戳
	L.SetGlobal("now", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().Unix()))
		return 1
	}))

	// now_ms() 返回当前毫秒时间戳
	L.SetGlobal("now_ms", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().UnixMilli()))
		return 1
	}))

	// log(message) 打印日志
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)
		fmt.Printf("[Lua] %s\n", msg)
		return 0
	}))
}

// Execute 执行 Lua 脚本
// 参数：
//   - script: Lua 脚本内容
//   - ctx: 鉴权上下文
// 返回：
//   - *AuthResult: 鉴权结果
//   - error: 错误信息
func (e *LuaExecutor) Execute(script string, ctx *AuthContext) (*AuthResult, error) {
	// 创建新的 Lua 状态机（避免并发问题）
	L := lua.NewState()
	defer L.Close()

	// 注册全局函数
	registerGlobalFunctions(L)

	// 设置上下文变量
	e.setContextVariables(L, ctx)

	// 执行脚本
	if err := L.DoString(script); err != nil {
		return nil, fmt.Errorf("Lua 脚本执行失败: %w", err)
	}

	// 获取返回值
	result := L.Get(-1)
	L.Pop(1)

	return e.parseResult(result)
}

// ExecuteFile 从文件执行 Lua 脚本
// 参数：
//   - filePath: Lua 脚本文件路径
//   - ctx: 鉴权上下文
// 返回：
//   - *AuthResult: 鉴权结果
//   - error: 错误信息
func (e *LuaExecutor) ExecuteFile(filePath string, ctx *AuthContext) (*AuthResult, error) {
	script, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 Lua 脚本文件失败: %w", err)
	}
	return e.Execute(string(script), ctx)
}

// setContextVariables 设置上下文变量
func (e *LuaExecutor) setContextVariables(L *lua.LState, ctx *AuthContext) {
	// 设置 api_key
	L.SetGlobal("api_key", lua.LString(ctx.APIKey))

	// 设置 data（从 Provider 查询到的数据）
	if ctx.Data != nil {
		dataTable := e.mapToTable(L, ctx.Data)
		L.SetGlobal("data", dataTable)
		L.SetGlobal("key", dataTable) // 兼容别名
	}

	// 设置 request（请求信息）
	if ctx.Request != nil {
		requestTable := L.NewTable()
		requestTable.RawSetString("method", lua.LString(ctx.Request.Method))
		requestTable.RawSetString("path", lua.LString(ctx.Request.Path))
		requestTable.RawSetString("ip", lua.LString(ctx.Request.IP))

		// 设置 headers
		headersTable := L.NewTable()
		for k, v := range ctx.Request.Headers {
			headersTable.RawSetString(k, lua.LString(v))
		}
		requestTable.RawSetString("headers", headersTable)

		L.SetGlobal("request", requestTable)
	}

	// 设置 metadata（累积的元数据）
	if ctx.Metadata != nil {
		metadataTable := e.mapToTable(L, ctx.Metadata)
		L.SetGlobal("metadata", metadataTable)
	}
}

// mapToTable 将 Go map 转换为 Lua table
func (e *LuaExecutor) mapToTable(L *lua.LState, m map[string]interface{}) *lua.LTable {
	table := L.NewTable()
	for k, v := range m {
		table.RawSetString(k, luajson.New(L, v))
	}
	return table
}

// parseResult 解析 Lua 返回值
func (e *LuaExecutor) parseResult(result lua.LValue) (*AuthResult, error) {
	// 如果返回 nil，默认允许
	if result == lua.LNil {
		return &AuthResult{Allow: true}, nil
	}

	// 如果返回布尔值
	if b, ok := result.(lua.LBool); ok {
		return &AuthResult{Allow: bool(b)}, nil
	}

	// 如果返回 table
	if table, ok := result.(*lua.LTable); ok {
		authResult := &AuthResult{
			Allow:    true,
			Metadata: make(map[string]interface{}),
		}

		// 获取 allow 字段
		allowVal := table.RawGetString("allow")
		if allowVal != lua.LNil {
			if b, ok := allowVal.(lua.LBool); ok {
				authResult.Allow = bool(b)
			}
		}

		// 获取 message 字段
		messageVal := table.RawGetString("message")
		if messageVal != lua.LNil {
			if s, ok := messageVal.(lua.LString); ok {
				authResult.Message = string(s)
			}
		}

		// 获取 metadata 字段
		metadataVal := table.RawGetString("metadata")
		if metadataVal != lua.LNil {
			if metaTable, ok := metadataVal.(*lua.LTable); ok {
				metaTable.ForEach(func(k, v lua.LValue) {
					authResult.Metadata[k.String()] = e.luaValueToGo(v)
				})
			}
		}

		return authResult, nil
	}

	return nil, fmt.Errorf("无效的 Lua 返回值类型: %T", result)
}

// luaValueToGo 将 Lua 值转换为 Go 值
func (e *LuaExecutor) luaValueToGo(v lua.LValue) interface{} {
	switch val := v.(type) {
	case lua.LBool:
		return bool(val)
	case lua.LNumber:
		return float64(val)
	case lua.LString:
		return string(val)
	case *lua.LTable:
		m := make(map[string]interface{})
		val.ForEach(func(k, v lua.LValue) {
			m[k.String()] = e.luaValueToGo(v)
		})
		return m
	default:
		return nil
	}
}

// Close 关闭 Lua 状态机
func (e *LuaExecutor) Close() {
	if e.state != nil {
		e.state.Close()
	}
}
