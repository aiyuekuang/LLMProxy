package scripting

import (
	"log"

	lua "github.com/yuin/gopher-lua"
)

// TransformScript 转换脚本执行器
type TransformScript struct {
	engine *Engine
}

// TransformResult 转换结果
type TransformResult struct {
	Body       map[string]interface{} // 转换后的请求体/响应体
	Headers    map[string]string      // 转换后的请求头/响应头
	StatusCode int                    // 响应状态码（仅响应转换）
}

// NewTransformScript 创建转换脚本执行器
// 参数：
//   - config: 引擎配置
//
// 返回：
//   - *TransformScript: 转换脚本执行器
//   - error: 错误信息
func NewTransformScript(config *EngineConfig) (*TransformScript, error) {
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	return &TransformScript{
		engine: engine,
	}, nil
}

// TransformRequest 执行请求转换脚本
// 参数：
//   - requestBody: 原始请求体
//   - headers: 原始请求头
//   - userID: 用户 ID
//   - apiKey: API Key
//   - requestID: 请求 ID
//   - backendURL: 目标后端 URL
//
// 返回：
//   - *TransformResult: 转换结果（nil 表示不转换）
//   - error: 错误信息
func (t *TransformScript) TransformRequest(
	requestBody map[string]interface{},
	headers map[string]string,
	userID string,
	apiKey string,
	requestID string,
	backendURL string,
) (*TransformResult, error) {
	// 从池中获取 VM
	vmInterface := t.engine.vmPool.Get()
	if vmInterface == nil {
		return nil, nil
	}
	vm := vmInterface.(*lua.LState)
	defer t.engine.vmPool.Put(vm)

	// 构造 request 表
	requestTable := vm.NewTable()
	requestTable.RawSetString("id", lua.LString(requestID))
	requestTable.RawSetString("body", MapToLuaTable(vm, requestBody))
	requestTable.RawSetString("user_id", lua.LString(userID))
	requestTable.RawSetString("api_key", lua.LString(apiKey))
	requestTable.RawSetString("backend_url", lua.LString(backendURL))

	// 添加 headers
	headersTable := vm.NewTable()
	for k, v := range headers {
		headersTable.RawSetString(k, lua.LString(v))
	}
	requestTable.RawSetString("headers", headersTable)

	vm.SetGlobal("request", requestTable)

	// 执行脚本
	if err := vm.DoString(t.engine.script); err != nil {
		log.Printf("请求转换脚本执行失败: %v", err)
		return nil, err
	}

	// 获取返回值
	result := vm.Get(-1)
	vm.Pop(1)

	// 如果返回 nil，不转换
	if result.Type() == lua.LTNil {
		return nil, nil
	}

	// 解析返回值
	if table, ok := result.(*lua.LTable); ok {
		transformResult := &TransformResult{}

		// 获取 body 字段
		if body := table.RawGetString("body"); body.Type() == lua.LTTable {
			transformResult.Body = LuaTableToMap(body.(*lua.LTable))
		}

		// 获取 headers 字段
		if headers := table.RawGetString("headers"); headers.Type() == lua.LTTable {
			headersMap := LuaTableToMap(headers.(*lua.LTable))
			transformResult.Headers = make(map[string]string)
			for k, v := range headersMap {
				if str, ok := v.(string); ok {
					transformResult.Headers[k] = str
				}
			}
		}

		log.Printf("请求转换脚本执行成功")
		return transformResult, nil
	}

	return nil, nil
}

// TransformResponse 执行响应转换脚本
// 参数：
//   - responseBody: 原始响应体
//   - headers: 原始响应头
//   - statusCode: 原始状态码
//   - userID: 用户 ID
//   - requestID: 请求 ID
//   - backendURL: 后端 URL
//   - latencyMS: 延迟（毫秒）
//
// 返回：
//   - *TransformResult: 转换结果（nil 表示不转换）
//   - error: 错误信息
func (t *TransformScript) TransformResponse(
	responseBody map[string]interface{},
	headers map[string]string,
	statusCode int,
	userID string,
	requestID string,
	backendURL string,
	latencyMS int64,
) (*TransformResult, error) {
	// 从池中获取 VM
	vmInterface := t.engine.vmPool.Get()
	if vmInterface == nil {
		return nil, nil
	}
	vm := vmInterface.(*lua.LState)
	defer t.engine.vmPool.Put(vm)

	// 构造 request 表
	requestTable := vm.NewTable()
	requestTable.RawSetString("id", lua.LString(requestID))
	requestTable.RawSetString("user_id", lua.LString(userID))
	vm.SetGlobal("request", requestTable)

	// 构造 response 表
	responseTable := vm.NewTable()
	responseTable.RawSetString("status_code", lua.LNumber(statusCode))
	responseTable.RawSetString("body", MapToLuaTable(vm, responseBody))

	// 添加 headers
	headersTable := vm.NewTable()
	for k, v := range headers {
		headersTable.RawSetString(k, lua.LString(v))
	}
	responseTable.RawSetString("headers", headersTable)

	vm.SetGlobal("response", responseTable)

	// 设置其他变量
	vm.SetGlobal("backend_url", lua.LString(backendURL))
	vm.SetGlobal("latency_ms", lua.LNumber(latencyMS))

	// 执行脚本
	if err := vm.DoString(t.engine.script); err != nil {
		log.Printf("响应转换脚本执行失败: %v", err)
		return nil, err
	}

	// 获取返回值
	result := vm.Get(-1)
	vm.Pop(1)

	// 如果返回 nil，不转换
	if result.Type() == lua.LTNil {
		return nil, nil
	}

	// 解析返回值
	if table, ok := result.(*lua.LTable); ok {
		transformResult := &TransformResult{
			StatusCode: statusCode, // 默认保持原状态码
		}

		// 获取 body 字段
		if body := table.RawGetString("body"); body.Type() == lua.LTTable {
			transformResult.Body = LuaTableToMap(body.(*lua.LTable))
		}

		// 获取 headers 字段
		if headers := table.RawGetString("headers"); headers.Type() == lua.LTTable {
			headersMap := LuaTableToMap(headers.(*lua.LTable))
			transformResult.Headers = make(map[string]string)
			for k, v := range headersMap {
				if str, ok := v.(string); ok {
					transformResult.Headers[k] = str
				}
			}
		}

		// 获取 status_code 字段
		if statusCode := table.RawGetString("status_code"); statusCode.Type() == lua.LTNumber {
			transformResult.StatusCode = int(statusCode.(lua.LNumber))
		}

		log.Printf("响应转换脚本执行成功")
		return transformResult, nil
	}

	return nil, nil
}

// Close 关闭转换脚本执行器
func (t *TransformScript) Close() {
	if t.engine != nil {
		t.engine.Close()
	}
}
