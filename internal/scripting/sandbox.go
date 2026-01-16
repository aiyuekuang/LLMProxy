package scripting

import (
	lua "github.com/yuin/gopher-lua"
)

// setupSandbox 设置沙箱环境，禁用危险函数
// 参数：
//   - vm: Lua VM 实例
func setupSandbox(vm *lua.LState) {
	// 禁用危险的标准库函数
	disableDangerousFunctions(vm)

	// 限制递归深度
	vm.SetMaxStackSize(100)
}

// disableDangerousFunctions 禁用危险函数
// 参数：
//   - vm: Lua VM 实例
func disableDangerousFunctions(vm *lua.LState) {
	// 禁用 os 库的危险函数
	osTable := vm.GetGlobal("os")
	if osTable.Type() == lua.LTTable {
		table := osTable.(*lua.LTable)
		// 禁用系统命令执行
		table.RawSetString("execute", lua.LNil)
		table.RawSetString("exit", lua.LNil)
		table.RawSetString("remove", lua.LNil)
		table.RawSetString("rename", lua.LNil)
		table.RawSetString("tmpname", lua.LNil)
		table.RawSetString("getenv", lua.LNil)
		table.RawSetString("setlocale", lua.LNil)
	}

	// 禁用 io 库（文件操作）
	vm.SetGlobal("io", lua.LNil)

	// 禁用 package 库（模块加载）
	vm.SetGlobal("package", lua.LNil)
	vm.SetGlobal("require", lua.LNil)
	vm.SetGlobal("dofile", lua.LNil)
	vm.SetGlobal("loadfile", lua.LNil)

	// 禁用 debug 库
	vm.SetGlobal("debug", lua.LNil)

	// 保留安全的 os 函数
	safeOsTable := vm.NewTable()
	if osTable.Type() == lua.LTTable {
		table := osTable.(*lua.LTable)
		// 只保留时间相关函数
		safeOsTable.RawSetString("time", table.RawGetString("time"))
		safeOsTable.RawSetString("date", table.RawGetString("date"))
		safeOsTable.RawSetString("clock", table.RawGetString("clock"))
		safeOsTable.RawSetString("difftime", table.RawGetString("difftime"))
	}
	vm.SetGlobal("os", safeOsTable)
}
