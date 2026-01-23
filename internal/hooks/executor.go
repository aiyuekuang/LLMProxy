package hooks

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"llmproxy/internal/config"

	lua "github.com/yuin/gopher-lua"
)

// HookType 钩子类型
type HookType string

const (
	HookOnRequest  HookType = "on_request"
	HookOnAuth     HookType = "on_auth"
	HookOnRoute    HookType = "on_route"
	HookOnResponse HookType = "on_response"
	HookOnError    HookType = "on_error"
	HookOnComplete HookType = "on_complete"
)

// RequestInfo 请求信息
type RequestInfo struct {
	Method   string
	Path     string
	ClientIP string
	Headers  map[string]string
	Body     []byte
	APIKey   string
	UserID   string
}

// ResponseInfo 响应信息
type ResponseInfo struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	LatencyMs  int64
	BackendURL string
}

// HookContext 钩子上下文
type HookContext struct {
	Request   *RequestInfo
	Response  *ResponseInfo
	Error     error
	Metadata  map[string]interface{}
	Timestamp time.Time
}

// HookResult 钩子执行结果
type HookResult struct {
	Continue bool                   // 是否继续执行
	Modified bool                   // 是否修改了数据
	Headers  map[string]string      // 修改后的 headers
	Body     []byte                 // 修改后的 body
	Metadata map[string]interface{} // 元数据
	Error    string                 // 错误消息
}

// Executor 钩子执行器
type Executor struct {
	cfg        *config.HooksConfig
	onRequest  *hookEngine
	onAuth     *hookEngine
	onRoute    *hookEngine
	onResponse *hookEngine
	onError    *hookEngine
	onComplete *hookEngine
}

// hookEngine 单个钩子引擎
type hookEngine struct {
	script     string
	scriptFile string
	timeout    time.Duration
}

// NewExecutor 创建钩子执行器
func NewExecutor(cfg *config.HooksConfig) (*Executor, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	executor := &Executor{cfg: cfg}

	// 初始化各个钩子
	if cfg.OnRequest != nil && cfg.OnRequest.Enabled {
		engine, err := newHookEngine(cfg.OnRequest)
		if err != nil {
			return nil, fmt.Errorf("初始化 on_request 钩子失败: %w", err)
		}
		executor.onRequest = engine
		log.Println("钩子已启用: on_request")
	}

	if cfg.OnAuth != nil && cfg.OnAuth.Enabled {
		engine, err := newHookEngine(cfg.OnAuth)
		if err != nil {
			return nil, fmt.Errorf("初始化 on_auth 钩子失败: %w", err)
		}
		executor.onAuth = engine
		log.Println("钩子已启用: on_auth")
	}

	if cfg.OnRoute != nil && cfg.OnRoute.Enabled {
		engine, err := newHookEngine(cfg.OnRoute)
		if err != nil {
			return nil, fmt.Errorf("初始化 on_route 钩子失败: %w", err)
		}
		executor.onRoute = engine
		log.Println("钩子已启用: on_route")
	}

	if cfg.OnResponse != nil && cfg.OnResponse.Enabled {
		engine, err := newHookEngine(cfg.OnResponse)
		if err != nil {
			return nil, fmt.Errorf("初始化 on_response 钩子失败: %w", err)
		}
		executor.onResponse = engine
		log.Println("钩子已启用: on_response")
	}

	if cfg.OnError != nil && cfg.OnError.Enabled {
		engine, err := newHookEngine(cfg.OnError)
		if err != nil {
			return nil, fmt.Errorf("初始化 on_error 钩子失败: %w", err)
		}
		executor.onError = engine
		log.Println("钩子已启用: on_error")
	}

	if cfg.OnComplete != nil && cfg.OnComplete.Enabled {
		engine, err := newHookEngine(cfg.OnComplete)
		if err != nil {
			return nil, fmt.Errorf("初始化 on_complete 钩子失败: %w", err)
		}
		executor.onComplete = engine
		log.Println("钩子已启用: on_complete")
	}

	return executor, nil
}

// newHookEngine 创建钩子引擎
func newHookEngine(cfg *config.ScriptConfig) (*hookEngine, error) {
	if cfg.Script == "" && cfg.Path == "" {
		return nil, fmt.Errorf("脚本内容和脚本文件路径不能同时为空")
	}

	engine := &hookEngine{
		script:     cfg.Script,
		scriptFile: cfg.Path,
		timeout:    cfg.Timeout,
	}

	if engine.timeout == 0 {
		engine.timeout = 100 * time.Millisecond
	}

	// 如果是文件，检查文件是否存在
	if cfg.Path != "" {
		if _, err := os.Stat(cfg.Path); os.IsNotExist(err) {
			return nil, fmt.Errorf("脚本文件不存在: %s", cfg.Path)
		}
	}

	// 验证脚本
	if err := engine.validate(); err != nil {
		return nil, fmt.Errorf("脚本验证失败: %w", err)
	}

	return engine, nil
}

// validate 验证脚本
func (e *hookEngine) validate() error {
	L := lua.NewState()
	defer L.Close()

	var err error
	if e.scriptFile != "" {
		err = L.DoFile(e.scriptFile)
	} else {
		err = L.DoString(e.script)
	}
	return err
}

// ExecuteOnRequest 执行 on_request 钩子
func (e *Executor) ExecuteOnRequest(ctx *HookContext) *HookResult {
	if e == nil || e.onRequest == nil {
		return &HookResult{Continue: true}
	}
	return e.onRequest.execute(HookOnRequest, ctx)
}

// ExecuteOnAuth 执行 on_auth 钩子
func (e *Executor) ExecuteOnAuth(ctx *HookContext) *HookResult {
	if e == nil || e.onAuth == nil {
		return &HookResult{Continue: true}
	}
	return e.onAuth.execute(HookOnAuth, ctx)
}

// ExecuteOnRoute 执行 on_route 钩子
func (e *Executor) ExecuteOnRoute(ctx *HookContext) *HookResult {
	if e == nil || e.onRoute == nil {
		return &HookResult{Continue: true}
	}
	return e.onRoute.execute(HookOnRoute, ctx)
}

// ExecuteOnResponse 执行 on_response 钩子
func (e *Executor) ExecuteOnResponse(ctx *HookContext) *HookResult {
	if e == nil || e.onResponse == nil {
		return &HookResult{Continue: true}
	}
	return e.onResponse.execute(HookOnResponse, ctx)
}

// ExecuteOnError 执行 on_error 钩子
func (e *Executor) ExecuteOnError(ctx *HookContext) *HookResult {
	if e == nil || e.onError == nil {
		return &HookResult{Continue: true}
	}
	return e.onError.execute(HookOnError, ctx)
}

// ExecuteOnComplete 执行 on_complete 钩子（异步）
func (e *Executor) ExecuteOnComplete(ctx *HookContext) {
	if e == nil || e.onComplete == nil {
		return
	}
	go func() {
		e.onComplete.execute(HookOnComplete, ctx)
	}()
}

// execute 执行钩子脚本
func (e *hookEngine) execute(hookType HookType, ctx *HookContext) *HookResult {
	result := &HookResult{
		Continue: true,
		Metadata: make(map[string]interface{}),
	}

	L := lua.NewState()
	defer L.Close()

	// 设置全局变量
	e.setGlobals(L, ctx)

	// 注册辅助函数
	registerHelperFunctions(L)

	// 执行脚本
	done := make(chan error, 1)
	go func() {
		var err error
		if e.scriptFile != "" {
			err = L.DoFile(e.scriptFile)
		} else {
			err = L.DoString(e.script)
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("钩子 [%s] 执行失败: %v", hookType, err)
			result.Error = err.Error()
			return result
		}
	case <-time.After(e.timeout):
		log.Printf("钩子 [%s] 执行超时", hookType)
		result.Error = "执行超时"
		return result
	}

	// 获取返回值
	ret := L.Get(-1)
	if ret == lua.LNil {
		return result
	}

	// 解析返回值
	if tbl, ok := ret.(*lua.LTable); ok {
		// continue
		if v := tbl.RawGetString("continue"); v != lua.LNil {
			if b, ok := v.(lua.LBool); ok {
				result.Continue = bool(b)
			}
		}

		// modified
		if v := tbl.RawGetString("modified"); v != lua.LNil {
			if b, ok := v.(lua.LBool); ok {
				result.Modified = bool(b)
			}
		}

		// error
		if v := tbl.RawGetString("error"); v != lua.LNil {
			if s, ok := v.(lua.LString); ok {
				result.Error = string(s)
			}
		}

		// headers
		if v := tbl.RawGetString("headers"); v != lua.LNil {
			if headersTable, ok := v.(*lua.LTable); ok {
				result.Headers = make(map[string]string)
				headersTable.ForEach(func(k, v lua.LValue) {
					result.Headers[k.String()] = v.String()
				})
			}
		}

		// metadata
		if v := tbl.RawGetString("metadata"); v != lua.LNil {
			if metaTable, ok := v.(*lua.LTable); ok {
				metaTable.ForEach(func(k, v lua.LValue) {
					result.Metadata[k.String()] = luaValueToGo(v)
				})
			}
		}
	}

	return result
}

// setGlobals 设置全局变量
func (e *hookEngine) setGlobals(L *lua.LState, ctx *HookContext) {
	// request 表
	if ctx.Request != nil {
		reqTable := L.NewTable()
		reqTable.RawSetString("method", lua.LString(ctx.Request.Method))
		reqTable.RawSetString("path", lua.LString(ctx.Request.Path))
		reqTable.RawSetString("client_ip", lua.LString(ctx.Request.ClientIP))
		reqTable.RawSetString("api_key", lua.LString(ctx.Request.APIKey))
		reqTable.RawSetString("user_id", lua.LString(ctx.Request.UserID))

		// headers
		headersTable := L.NewTable()
		for k, v := range ctx.Request.Headers {
			headersTable.RawSetString(k, lua.LString(v))
		}
		reqTable.RawSetString("headers", headersTable)

		// body
		if ctx.Request.Body != nil {
			reqTable.RawSetString("body", lua.LString(string(ctx.Request.Body)))
		}

		L.SetGlobal("request", reqTable)
	}

	// response 表
	if ctx.Response != nil {
		respTable := L.NewTable()
		respTable.RawSetString("status_code", lua.LNumber(ctx.Response.StatusCode))
		respTable.RawSetString("latency_ms", lua.LNumber(ctx.Response.LatencyMs))
		respTable.RawSetString("backend_url", lua.LString(ctx.Response.BackendURL))

		// headers
		headersTable := L.NewTable()
		for k, v := range ctx.Response.Headers {
			headersTable.RawSetString(k, lua.LString(v))
		}
		respTable.RawSetString("headers", headersTable)

		// body
		if ctx.Response.Body != nil {
			respTable.RawSetString("body", lua.LString(string(ctx.Response.Body)))
		}

		L.SetGlobal("response", respTable)
	}

	// error
	if ctx.Error != nil {
		L.SetGlobal("error_message", lua.LString(ctx.Error.Error()))
	}

	// metadata 表
	metaTable := L.NewTable()
	for k, v := range ctx.Metadata {
		metaTable.RawSetString(k, goValueToLua(L, v))
	}
	L.SetGlobal("metadata", metaTable)

	// timestamp
	L.SetGlobal("timestamp", lua.LNumber(ctx.Timestamp.Unix()))
}

// registerHelperFunctions 注册辅助函数
func registerHelperFunctions(L *lua.LState) {
	// log(message)
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)
		log.Printf("[Hook] %s", msg)
		return 0
	}))

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
}

// luaValueToGo 将 Lua 值转换为 Go 值
func luaValueToGo(v lua.LValue) interface{} {
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
			m[k.String()] = luaValueToGo(v)
		})
		return m
	default:
		return nil
	}
}

// goValueToLua 将 Go 值转换为 Lua 值
func goValueToLua(L *lua.LState, v interface{}) lua.LValue {
	switch val := v.(type) {
	case bool:
		return lua.LBool(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case string:
		return lua.LString(val)
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, v := range val {
			tbl.RawSetString(k, goValueToLua(L, v))
		}
		return tbl
	default:
		return lua.LNil
	}
}

// ExtractRequestInfo 从 HTTP 请求中提取请求信息
func ExtractRequestInfo(r *http.Request, body []byte, clientIP, apiKey, userID string) *RequestInfo {
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &RequestInfo{
		Method:   r.Method,
		Path:     r.URL.Path,
		ClientIP: clientIP,
		Headers:  headers,
		Body:     body,
		APIKey:   apiKey,
		UserID:   userID,
	}
}

// Close 关闭执行器
func (e *Executor) Close() error {
	// 目前没有需要清理的资源
	return nil
}
