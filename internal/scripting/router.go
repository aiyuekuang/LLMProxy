package scripting

import (
	"log"

	"llmproxy/internal/lb"

	lua "github.com/yuin/gopher-lua"
)

// RouterScript 路由脚本执行器
type RouterScript struct {
	engine *Engine
}

// NewRouterScript 创建路由脚本执行器
// 参数：
//   - config: 引擎配置
//
// 返回：
//   - *RouterScript: 路由脚本执行器
//   - error: 错误信息
func NewRouterScript(config *EngineConfig) (*RouterScript, error) {
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	return &RouterScript{
		engine: engine,
	}, nil
}

// SelectBackend 执行路由脚本选择后端
// 参数：
//   - requestBody: 请求体
//   - userID: 用户 ID
//   - apiKey: API Key
//   - clientIP: 客户端 IP
//   - requestID: 请求 ID
//   - path: 请求路径
//   - headers: 请求头
//   - backends: 可用后端列表
//
// 返回：
//   - string: 后端名称（空字符串表示使用默认负载均衡）
//   - error: 错误信息
func (r *RouterScript) SelectBackend(
	requestBody map[string]interface{},
	userID string,
	apiKey string,
	clientIP string,
	requestID string,
	path string,
	headers map[string]string,
	backends map[string]*lb.Backend,
) (string, error) {
	// 从池中获取 VM
	vmInterface := r.engine.vmPool.Get()
	if vmInterface == nil {
		return "", nil
	}
	vm := vmInterface.(*lua.LState)
	defer r.engine.vmPool.Put(vm)

	// 构造 request 表
	requestTable := vm.NewTable()
	requestTable.RawSetString("id", lua.LString(requestID))
	requestTable.RawSetString("body", MapToLuaTable(vm, requestBody))
	requestTable.RawSetString("user_id", lua.LString(userID))
	requestTable.RawSetString("api_key", lua.LString(apiKey))
	requestTable.RawSetString("client_ip", lua.LString(clientIP))
	requestTable.RawSetString("path", lua.LString(path))

	// 添加 headers
	headersTable := vm.NewTable()
	for k, v := range headers {
		headersTable.RawSetString(k, lua.LString(v))
	}
	requestTable.RawSetString("headers", headersTable)

	vm.SetGlobal("request", requestTable)

	// 构造 backends 表
	backendsTable := vm.NewTable()
	for name, backend := range backends {
		backendTable := vm.NewTable()
		backendTable.RawSetString("url", lua.LString(backend.URL))
		backendTable.RawSetString("healthy", lua.LBool(backend.Healthy))
		backendTable.RawSetString("weight", lua.LNumber(backend.Weight))
		// 注意: lb.Backend 当前没有 AvgLatency 和 ActiveConnections 字段
		// 如需这些信息，需要扩展 lb.Backend 结构体
		backendTable.RawSetString("latency_ms", lua.LNumber(0))
		backendTable.RawSetString("active_connections", lua.LNumber(0))
		backendsTable.RawSetString(name, backendTable)
	}
	vm.SetGlobal("backends", backendsTable)

	// 执行脚本（脚本应该返回后端名称）
	if err := vm.DoString(r.engine.script); err != nil {
		log.Printf("路由脚本执行失败: %v", err)
		return "", err
	}

	// 获取返回值
	result := vm.Get(-1)
	vm.Pop(1)

	// 如果返回 nil，使用默认负载均衡
	if result.Type() == lua.LTNil {
		return "", nil
	}

	// 返回后端名称
	if str, ok := result.(lua.LString); ok {
		backendName := string(str)
		log.Printf("路由脚本选择后端: %s", backendName)
		return backendName, nil
	}

	return "", nil
}

// Close 关闭路由脚本执行器
func (r *RouterScript) Close() {
	if r.engine != nil {
		r.engine.Close()
	}
}
