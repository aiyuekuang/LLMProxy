# LLMProxy Lua 脚本示例

本目录包含 LLMProxy 支持的所有 Lua 脚本示例。

## 脚本列表

### 1. routing.lua - 路由决策脚本
根据请求参数（如 viplevel）选择后端服务器。

**输入变量**:
- `request.body` - 请求体（包含所有用户参数）
- `request.user_id` - 用户 ID
- `request.api_key` - API Key
- `request.client_ip` - 客户端 IP
- `backends` - 可用后端列表

**返回值**: 后端名称（字符串）或 `nil`（使用默认负载均衡）

**示例**:
```lua
local viplevel = request.body.viplevel or 0
if viplevel >= 3 then
    return "ultra"
else
    return "standard"
end
```

---

### 2. auth.lua - 鉴权决策脚本
自定义鉴权逻辑，如基于模型的权限控制。

**输入变量**:
- `request.body` - 请求体
- `request.user_id` - 用户 ID
- `key_info` - API Key 详细信息
- `standard_checks` - 标准检查结果

**返回值**: 
```lua
{
    allow = true/false,
    reason = "拒绝原因",
    status_code = 403
}
```

**示例**:
```lua
local model = request.body.model
if model == "gpt-4" and key_info.total_quota < 1000000 then
    return {
        allow = false,
        reason = "您的账户等级不支持 GPT-4 模型"
    }
end
return {allow = true}
```

---

### 3. request_transform.lua - 请求转换脚本
修改发送到后端的请求参数。

**输入变量**:
- `request.body` - 原始请求体
- `request.headers` - 原始请求头
- `request.backend_url` - 目标后端 URL

**返回值**:
```lua
{
    body = {...},      -- 修改后的请求体
    headers = {...}    -- 修改后的请求头
}
```

**示例**:
```lua
-- 模型名称映射
local model_map = {
    ["gpt-4"] = "llama-3-70b-instruct"
}
local new_body = request.body
new_body.model = model_map[request.body.model] or request.body.model
return {body = new_body}
```

---

### 4. response_transform.lua - 响应转换脚本
修改返回给客户端的响应。

**输入变量**:
- `response.body` - 原始响应体
- `response.headers` - 原始响应头
- `response.status_code` - 原始状态码
- `backend_url` - 后端 URL
- `latency_ms` - 延迟（毫秒）

**返回值**:
```lua
{
    status_code = 200,
    body = {...},
    headers = {...}
}
```

---

### 5. rate_limit.lua - 限流决策脚本
自定义限流逻辑，如高峰期限流。

**输入变量**:
- `request.body` - 请求体
- `key_info` - Key 信息
- `rate_limit_status` - 限流状态
- `current_time` - 当前时间信息

**返回值**:
```lua
{
    allow = true/false,
    reason = "拒绝原因",
    retry_after = 60  -- 秒
}
```

---

### 6. usage.lua - 用量计算脚本
自定义计费规则，如 VIP 折扣。

**输入变量**:
- `request.body` - 请求体
- `usage` - 原始用量数据
- `response.body` - 响应体
- `metadata` - 元数据

**返回值**: 修改后的用量数据（map）

**示例**:
```lua
local viplevel = request.body.viplevel or 0
local discount = viplevel >= 3 and 0.5 or 1.0
return {
    total_tokens = usage.total_tokens,
    billable_tokens = math.floor(usage.total_tokens * discount),
    discount = discount
}
```

---

### 7. error_handler.lua - 错误处理脚本
自定义错误响应格式。

**输入变量**:
- `request.body` - 请求体
- `error.type` - 错误类型
- `error.message` - 错误消息
- `error.status_code` - 状态码

**返回值**:
```lua
{
    status_code = 500,
    body = {
        error = {
            message = "友好的错误消息",
            type = "error_type"
        }
    }
}
```

---

## 内置工具函数

### JSON 操作
```lua
-- 编码
local json_str = json.encode({key = "value"})

-- 解码
local obj = json.decode('{"key":"value"}')
```

### 字符串工具
```lua
-- 分割
local parts = string_utils.split("a,b,c", ",")

-- 去除空白
local trimmed = string_utils.trim("  hello  ")

-- 判断前缀/后缀
local has_prefix = string_utils.starts_with("hello", "he")
local has_suffix = string_utils.ends_with("hello", "lo")

-- 包含判断
local contains = string_utils.contains("hello world", "world")

-- 大小写转换
local lower = string_utils.lower("HELLO")
local upper = string_utils.upper("hello")
```

### 时间工具
```lua
-- 当前时间戳
local now = time.now()

-- 格式化时间
local formatted = time.format(now, "2006-01-02 15:04:05")

-- 解析时间
local timestamp = time.parse("2024-01-01 00:00:00", "2006-01-02 15:04:05")
```

### 哈希工具
```lua
-- MD5
local md5_hash = hash.md5("hello")

-- SHA256
local sha256_hash = hash.sha256("hello")
```

### 日志工具
```lua
log.info("信息日志")
log.warn("警告日志")
log.error("错误日志")
```

---

## 配置示例

```yaml
scripts:
  routing:
    enabled: true
    script_file: "/etc/llmproxy/scripts/routing.lua"
    timeout: 100ms
    max_memory: 10485760  # 10MB
```

---

## 安全限制

1. **禁止的操作**:
   - 文件系统访问（`io.open`、`os.execute` 等）
   - 网络访问
   - 加载外部模块（`require`）
   - 无限循环

2. **资源限制**:
   - 执行超时：默认 100ms
   - 内存限制：默认 10MB
   - 递归深度：最大 100 层

3. **沙箱环境**:
   - 脚本运行在隔离的沙箱环境中
   - 只能访问提供的全局变量和内置函数

---

## 最佳实践

1. **性能优化**:
   - 避免复杂计算
   - 缓存常用数据
   - 使用局部变量

2. **错误处理**:
   - 使用 `or` 提供默认值
   - 检查变量是否存在
   - 记录日志便于调试

3. **代码风格**:
   - 添加注释说明逻辑
   - 使用有意义的变量名
   - 保持代码简洁

---

## 调试技巧

1. **使用日志**:
```lua
log.info("变量值: " .. tostring(value))
```

2. **返回 nil 使用默认逻辑**:
```lua
-- 如果不确定，返回 nil 让网关使用默认逻辑
return nil
```

3. **查看网关日志**:
```bash
docker logs llmproxy
```
