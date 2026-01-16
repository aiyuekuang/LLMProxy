package scripting

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// MapToLuaTable 将 Go map 转换为 Lua table（支持嵌套）
// 参数：
//   - vm: Lua VM 实例
//   - m: Go map
// 返回：
//   - *lua.LTable: Lua table
func MapToLuaTable(vm *lua.LState, m map[string]interface{}) *lua.LTable {
	table := vm.NewTable()

	for k, v := range m {
		table.RawSetString(k, GoValueToLua(vm, v))
	}

	return table
}

// GoValueToLua 将 Go 值转换为 Lua 值（支持嵌套）
// 参数：
//   - vm: Lua VM 实例
//   - v: Go 值
// 返回：
//   - lua.LValue: Lua 值
func GoValueToLua(vm *lua.LState, v interface{}) lua.LValue {
	if v == nil {
		return lua.LNil
	}

	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case float32:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	case map[string]interface{}:
		// 递归处理嵌套 map
		return MapToLuaTable(vm, val)
	case []interface{}:
		// 处理数组
		arrayTable := vm.NewTable()
		for i, item := range val {
			arrayTable.RawSetInt(i+1, GoValueToLua(vm, item)) // Lua 数组从 1 开始
		}
		return arrayTable
	case []string:
		// 处理字符串数组
		arrayTable := vm.NewTable()
		for i, item := range val {
			arrayTable.RawSetInt(i+1, lua.LString(item))
		}
		return arrayTable
	default:
		// 其他类型转换为字符串
		return lua.LString(fmt.Sprintf("%v", val))
	}
}

// LuaTableToMap 将 Lua table 转换为 Go map（支持嵌套）
// 参数：
//   - table: Lua table
// 返回：
//   - map[string]interface{}: Go map
func LuaTableToMap(table *lua.LTable) map[string]interface{} {
	result := make(map[string]interface{})

	table.ForEach(func(key, value lua.LValue) {
		keyStr := key.String()
		result[keyStr] = LuaValueToGo(value)
	})

	return result
}

// LuaValueToGo 将 Lua 值转换为 Go 值（支持嵌套）
// 参数：
//   - value: Lua 值
// 返回：
//   - interface{}: Go 值
func LuaValueToGo(value lua.LValue) interface{} {
	switch v := value.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		// 判断是数组还是 map
		if isArray(v) {
			return luaTableToArray(v)
		}
		return LuaTableToMap(v)
	case *lua.LNilType:
		return nil
	default:
		return value.String()
	}
}

// isArray 判断 Lua table 是否为数组
// 参数：
//   - table: Lua table
// 返回：
//   - bool: 是否为数组
func isArray(table *lua.LTable) bool {
	// 如果所有 key 都是连续的整数（从 1 开始），则认为是数组
	maxN := table.MaxN()
	if maxN == 0 {
		return false
	}

	count := 0
	table.ForEach(func(key, value lua.LValue) {
		count++
	})

	return count == maxN
}

// luaTableToArray 将 Lua table 转换为 Go 数组
// 参数：
//   - table: Lua table
// 返回：
//   - []interface{}: Go 数组
func luaTableToArray(table *lua.LTable) []interface{} {
	maxN := table.MaxN()
	result := make([]interface{}, maxN)

	for i := 1; i <= maxN; i++ {
		value := table.RawGetInt(i)
		result[i-1] = LuaValueToGo(value)
	}

	return result
}

// SetGlobalMap 设置全局 map 变量
// 参数：
//   - vm: Lua VM 实例
//   - name: 变量名
//   - m: Go map
func SetGlobalMap(vm *lua.LState, name string, m map[string]interface{}) {
	table := MapToLuaTable(vm, m)
	vm.SetGlobal(name, table)
}
