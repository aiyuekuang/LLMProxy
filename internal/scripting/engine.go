package scripting

import (
	"fmt"
	"log"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Engine Lua 脚本引擎
type Engine struct {
	script      string        // 脚本内容
	scriptFile  string        // 脚本文件路径
	vmPool      *sync.Pool    // VM 池
	timeout     time.Duration // 脚本执行超时时间
	maxMemory   int           // 最大内存限制（字节）
	initialized bool          // 是否已初始化
	mu          sync.RWMutex  // 读写锁
}

// EngineConfig 引擎配置
type EngineConfig struct {
	Script     string        // 脚本内容（内联）
	ScriptFile string        // 脚本文件路径
	Timeout    time.Duration // 执行超时时间（默认 100ms）
	MaxMemory  int           // 最大内存限制（默认 10MB）
}

// NewEngine 创建 Lua 引擎
// 参数：
//   - config: 引擎配置
//
// 返回：
//   - *Engine: 引擎实例
//   - error: 错误信息
func NewEngine(config *EngineConfig) (*Engine, error) {
	if config.Script == "" && config.ScriptFile == "" {
		return nil, fmt.Errorf("脚本内容和脚本文件路径不能同时为空")
	}

	// 设置默认值
	if config.Timeout == 0 {
		config.Timeout = 100 * time.Millisecond
	}
	if config.MaxMemory == 0 {
		config.MaxMemory = 10 * 1024 * 1024 // 10MB
	}

	engine := &Engine{
		script:     config.Script,
		scriptFile: config.ScriptFile,
		timeout:    config.Timeout,
		maxMemory:  config.MaxMemory,
	}

	// 创建 VM 池
	engine.vmPool = &sync.Pool{
		New: func() interface{} {
			return engine.createVM()
		},
	}

	// 验证脚本是否可以加载
	if err := engine.validateScript(); err != nil {
		return nil, fmt.Errorf("脚本验证失败: %w", err)
	}

	engine.initialized = true
	log.Printf("Lua 引擎初始化成功")

	return engine, nil
}

// createVM 创建新的 Lua VM
// 返回：
//   - *lua.LState: Lua VM 实例
func (e *Engine) createVM() *lua.LState {
	vm := lua.NewState()

	// 设置沙箱环境
	setupSandbox(vm)

	// 加载标准库
	setupStdlib(vm)

	// 加载脚本
	var err error
	if e.scriptFile != "" {
		err = vm.DoFile(e.scriptFile)
	} else {
		err = vm.DoString(e.script)
	}

	if err != nil {
		log.Printf("加载脚本失败: %v", err)
		vm.Close()
		return nil
	}

	return vm
}

// validateScript 验证脚本是否可以加载
// 返回：
//   - error: 错误信息
func (e *Engine) validateScript() error {
	vm := e.createVM()
	if vm == nil {
		return fmt.Errorf("无法创建 VM")
	}
	vm.Close()
	return nil
}

// Execute 执行脚本
// 参数：
//   - functionName: 要调用的函数名
//   - args: 函数参数（Lua 值）
//
// 返回：
//   - lua.LValue: 返回值
//   - error: 错误信息
func (e *Engine) Execute(functionName string, args ...lua.LValue) (lua.LValue, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return lua.LNil, fmt.Errorf("引擎未初始化")
	}
	e.mu.RUnlock()

	// 从池中获取 VM
	vmInterface := e.vmPool.Get()
	if vmInterface == nil {
		return lua.LNil, fmt.Errorf("无法获取 VM")
	}
	vm := vmInterface.(*lua.LState)
	defer e.vmPool.Put(vm)

	// 注意：gopher-lua 不直接支持执行超时，这里通过 goroutine + select 实现
	_ = vm.Context() // 可选：检查是否已设置 context

	done := make(chan struct{})
	var result lua.LValue
	var execErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("脚本执行 panic: %v", r)
			}
			close(done)
		}()

		// 获取函数
		fn := vm.GetGlobal(functionName)
		if fn.Type() != lua.LTFunction {
			execErr = fmt.Errorf("函数 %s 不存在或不是函数类型", functionName)
			return
		}

		// 调用函数
		if err := vm.CallByParam(lua.P{
			Fn:      fn,
			NRet:    1,
			Protect: true,
		}, args...); err != nil {
			execErr = err
			return
		}

		// 获取返回值
		result = vm.Get(-1)
		vm.Pop(1)
	}()

	// 等待执行完成或超时
	select {
	case <-done:
		return result, execErr
	case <-time.After(e.timeout):
		return lua.LNil, fmt.Errorf("脚本执行超时（%v）", e.timeout)
	}
}

// ExecuteSimple 执行简单脚本（直接返回值，不调用函数）
// 参数：
//   - context: 上下文数据（会设置为全局变量）
//
// 返回：
//   - lua.LValue: 返回值
//   - error: 错误信息
func (e *Engine) ExecuteSimple(context map[string]lua.LValue) (lua.LValue, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return lua.LNil, fmt.Errorf("引擎未初始化")
	}
	e.mu.RUnlock()

	// 从池中获取 VM
	vmInterface := e.vmPool.Get()
	if vmInterface == nil {
		return lua.LNil, fmt.Errorf("无法获取 VM")
	}
	vm := vmInterface.(*lua.LState)
	defer e.vmPool.Put(vm)

	// 设置上下文变量
	for key, value := range context {
		vm.SetGlobal(key, value)
	}

	// 设置超时
	done := make(chan struct{})
	var result lua.LValue
	var execErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("脚本执行 panic: %v", r)
			}
			close(done)
		}()

		// 执行脚本
		if e.scriptFile != "" {
			execErr = vm.DoFile(e.scriptFile)
		} else {
			execErr = vm.DoString(e.script)
		}

		if execErr != nil {
			return
		}

		// 获取返回值（栈顶）
		result = vm.Get(-1)
		vm.Pop(1)
	}()

	// 等待执行完成或超时
	select {
	case <-done:
		return result, execErr
	case <-time.After(e.timeout):
		return lua.LNil, fmt.Errorf("脚本执行超时（%v）", e.timeout)
	}
}

// Close 关闭引擎，释放资源
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.initialized {
		return
	}

	// 清空 VM 池
	// 注意：sync.Pool 没有提供清空方法，这里只是标记为未初始化
	e.initialized = false

	log.Printf("Lua 引擎已关闭")
}

// Reload 重新加载脚本
// 返回：
//   - error: 错误信息
func (e *Engine) Reload() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 验证新脚本
	if err := e.validateScript(); err != nil {
		return fmt.Errorf("脚本验证失败: %w", err)
	}

	// 重新创建 VM 池
	e.vmPool = &sync.Pool{
		New: func() interface{} {
			return e.createVM()
		},
	}

	log.Printf("Lua 脚本已重新加载")
	return nil
}
