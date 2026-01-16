package scripting

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// setupStdlib 设置标准库函数
// 参数：
//   - vm: Lua VM 实例
func setupStdlib(vm *lua.LState) {
	// JSON 库
	setupJSONLib(vm)

	// 字符串工具库
	setupStringUtilsLib(vm)

	// 时间工具库
	setupTimeLib(vm)

	// 哈希工具库
	setupHashLib(vm)

	// 日志工具库
	setupLogLib(vm)
}

// setupJSONLib 设置 JSON 库
// 参数：
//   - vm: Lua VM 实例
func setupJSONLib(vm *lua.LState) {
	jsonTable := vm.NewTable()

	// json.encode(obj) - 将 Lua table 编码为 JSON 字符串
	jsonTable.RawSetString("encode", vm.NewFunction(func(L *lua.LState) int {
		value := L.Get(1)
		goValue := LuaValueToGo(value)

		jsonBytes, err := json.Marshal(goValue)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LString(string(jsonBytes)))
		return 1
	}))

	// json.decode(str) - 将 JSON 字符串解码为 Lua table
	jsonTable.RawSetString("decode", vm.NewFunction(func(L *lua.LState) int {
		jsonStr := L.CheckString(1)

		var result interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(GoValueToLua(L, result))
		return 1
	}))

	vm.SetGlobal("json", jsonTable)
}

// setupStringUtilsLib 设置字符串工具库
// 参数：
//   - vm: Lua VM 实例
func setupStringUtilsLib(vm *lua.LState) {
	stringUtilsTable := vm.NewTable()

	// string_utils.split(str, delimiter) - 分割字符串
	stringUtilsTable.RawSetString("split", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		delimiter := L.CheckString(2)

		parts := strings.Split(str, delimiter)
		resultTable := L.NewTable()
		for i, part := range parts {
			resultTable.RawSetInt(i+1, lua.LString(part))
		}

		L.Push(resultTable)
		return 1
	}))

	// string_utils.trim(str) - 去除首尾空白
	stringUtilsTable.RawSetString("trim", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		L.Push(lua.LString(strings.TrimSpace(str)))
		return 1
	}))

	// string_utils.starts_with(str, prefix) - 判断是否以指定前缀开头
	stringUtilsTable.RawSetString("starts_with", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		prefix := L.CheckString(2)
		L.Push(lua.LBool(strings.HasPrefix(str, prefix)))
		return 1
	}))

	// string_utils.ends_with(str, suffix) - 判断是否以指定后缀结尾
	stringUtilsTable.RawSetString("ends_with", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		suffix := L.CheckString(2)
		L.Push(lua.LBool(strings.HasSuffix(str, suffix)))
		return 1
	}))

	// string_utils.contains(str, substr) - 判断是否包含子串
	stringUtilsTable.RawSetString("contains", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		substr := L.CheckString(2)
		L.Push(lua.LBool(strings.Contains(str, substr)))
		return 1
	}))

	// string_utils.lower(str) - 转换为小写
	stringUtilsTable.RawSetString("lower", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		L.Push(lua.LString(strings.ToLower(str)))
		return 1
	}))

	// string_utils.upper(str) - 转换为大写
	stringUtilsTable.RawSetString("upper", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		L.Push(lua.LString(strings.ToUpper(str)))
		return 1
	}))

	vm.SetGlobal("string_utils", stringUtilsTable)
}

// setupTimeLib 设置时间工具库
// 参数：
//   - vm: Lua VM 实例
func setupTimeLib(vm *lua.LState) {
	timeTable := vm.NewTable()

	// time.now() - 返回当前时间戳（秒）
	timeTable.RawSetString("now", vm.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().Unix()))
		return 1
	}))

	// time.format(timestamp, format) - 格式化时间
	timeTable.RawSetString("format", vm.NewFunction(func(L *lua.LState) int {
		timestamp := L.CheckInt64(1)
		format := L.OptString(2, "2006-01-02 15:04:05")

		t := time.Unix(timestamp, 0)
		L.Push(lua.LString(t.Format(format)))
		return 1
	}))

	// time.parse(str, format) - 解析时间字符串
	timeTable.RawSetString("parse", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		format := L.OptString(2, "2006-01-02 15:04:05")

		t, err := time.Parse(format, str)
		if err != nil {
			L.Push(lua.LNumber(0))
			L.Push(lua.LString(err.Error()))
			return 2
		}

		L.Push(lua.LNumber(t.Unix()))
		return 1
	}))

	vm.SetGlobal("time", timeTable)
}

// setupHashLib 设置哈希工具库
// 参数：
//   - vm: Lua VM 实例
func setupHashLib(vm *lua.LState) {
	hashTable := vm.NewTable()

	// hash.md5(str) - 计算 MD5 哈希
	hashTable.RawSetString("md5", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		hash := md5.Sum([]byte(str))
		L.Push(lua.LString(hex.EncodeToString(hash[:])))
		return 1
	}))

	// hash.sha256(str) - 计算 SHA256 哈希
	hashTable.RawSetString("sha256", vm.NewFunction(func(L *lua.LState) int {
		str := L.CheckString(1)
		hash := sha256.Sum256([]byte(str))
		L.Push(lua.LString(hex.EncodeToString(hash[:])))
		return 1
	}))

	vm.SetGlobal("hash", hashTable)
}

// setupLogLib 设置日志工具库
// 参数：
//   - vm: Lua VM 实例
func setupLogLib(vm *lua.LState) {
	logTable := vm.NewTable()

	// log.info(msg) - 记录 INFO 日志
	logTable.RawSetString("info", vm.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		log.Printf("[Lua Script] INFO: %s", msg)
		return 0
	}))

	// log.warn(msg) - 记录 WARN 日志
	logTable.RawSetString("warn", vm.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		log.Printf("[Lua Script] WARN: %s", msg)
		return 0
	}))

	// log.error(msg) - 记录 ERROR 日志
	logTable.RawSetString("error", vm.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		log.Printf("[Lua Script] ERROR: %s", msg)
		return 0
	}))

	vm.SetGlobal("log", logTable)
}
