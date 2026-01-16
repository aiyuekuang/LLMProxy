# LLMProxy Lua 脚本系统设计

## 文档概述

本文档定义 LLMProxy 中所有支持 Lua 脚本的扩展点，提供统一的脚本接口和执行环境。

---

## 支持 Lua 脚本的扩展点

### 1. 路由决策脚本 ⭐⭐⭐ (高优先级)

**位置**: 在负载均衡器选择后端之前

**用途**:
- 根据请求参数选择后端
- 处理复杂的路由逻辑
- 支持嵌套数据、多条件组合

**脚本输入**:
```lua
-- 全局变量
request = {
  id = "req_abc123",
  body = {
    model = "gpt-4",
    messages = {...},
    viplevel = 3,
    user = {
      profile = {
        viplevel = 3,
        region = "cn-north"
      }
    }
  },
  headers = {
    ["Content-Type"] = "application/json",
    ["User-Agent"] = "..."
  },
  user_id = "user_001",
  api_key = "sk-xxx",
  client_ip = "192.168.1.100",
  path = "/v1/chat/completions"
}

backends = {
  ["standard"] = {
    url = "http://standard-backend:8000",
    healthy = true,
    weight = 10,
    latency_ms = 120,
    active_connections = 5
  },
  ["high-performance"] = {
    url = "http://high-backend:8000",
    healthy = true,
    weight = 10,
    latency_ms = 80,
    active_connections = 3
  }
}
```

**脚本输出**:
```lua
-- 返回后端名称
return "high-performance"

-- 或返回 nil 使用默认负载均衡
return nil
```

**配置示例**:
```yaml
routing:
  script: |
    local viplevel = request.body.viplevel or 0
    if viplevel >= 3 then
      return "ultra"
    elseif viplevel >= 1 then
      return "high-performance"
    else
      return "standard"
    end
  
  # 或从文件加载
  script_file: "/etc/llmproxy/scripts/routing.lua"
```

---

### 2. 鉴权决策脚本 ⭐⭐⭐ (高优先级)

**位置**: 在标准鉴权检查之后

**用途**:
- 自定义鉴权逻辑
- 基于请求内容的权限控制
- 动态权限验证

**脚本输入**:
```lua
request = {
  id = "req_abc123",
  body = {...},
  headers = {...},
  user_id = "user_001",
  api_key = "sk-xxx",
  client_ip = "192.168.1.100"
}

key_info = {
  key = "sk-xxx",
  user_id = "user_001",
  name = "测试用户",
  status = "active",
  total_quota = 100000,
  used_quota = 50000,
  allowed_ips = {"192.168.1.0/24"},
  expires_at = "2026-12-31T23:59:59Z"
}

standard_checks = {
  key_valid = true,
  status_active = true,
  not_expired = true,
  ip_allowed = true,
  quota_available = true
}
```

**脚本输出**:
```lua
-- 允许访问
return {
  allow = true
}

-- 拒绝访问
return {
  allow = false,
  reason = "您没有权限使用此模型",
  status_code = 403
}
```

**配置示例**:
```yaml
auth:
  enabled: true
  script: |
    -- 检查用户是否有权限使用特定模型
    local model = request.body.model or ""
    local user_id = request.user_id
    
    -- VIP 用户可以使用所有模型
    if key_info.total_quota >= 1000000 then
      return {allow = true}
    end
    
    -- 普通用户只能使用 gpt-3.5
    if model == "gpt-4" then
      return {
        allow = false,
        reason = "您的账户等级不支持 GPT-4 模型"
      }
    end
    
    return {allow = true}
```

---

### 3. 请求转换脚本 ⭐⭐ (中优先级)

**位置**: 在发送到后端之前

**用途**:
- 修改请求参数
- 添加/删除字段
- 参数标准化

**脚本输入**:
```lua
request = {
  id = "req_abc123",
  body = {...},
  headers = {...},
  user_id = "user_001",
  backend_url = "http://backend:8000"
}
```

**脚本输出**:
```lua
-- 返回修改后的请求体和请求头
return {
  body = {
    model = "llama-3-70b-instruct",  -- 修改后的模型名
    messages = request.body.messages,
    temperature = 0.7
  },
  headers = {
    ["X-User-ID"] = request.user_id,
    ["X-Request-ID"] = request.id
  }
}

-- 或返回 nil 表示不修改
return nil
```

**配置示例**:
```yaml
routing:
  request_transform:
    enabled: true
    script: |
      -- 模型名称映射
      local model_map = {
        ["gpt-4"] = "llama-3-70b-instruct",
        ["gpt-3.5"] = "qwen-72b-chat"
      }
      
      local model = request.body.model
      local new_model = model_map[model] or model
      
      -- 修改请求体
      local new_body = {}
      for k, v in pairs(request.body) do
        new_body[k] = v
      end
      new_body.model = new_model
      
      -- 添加自定义请求头
      local new_headers = {
        ["X-User-ID"] = request.user_id,
        ["X-Original-Model"] = model
      }
      
      return {
        body = new_body,
        headers = new_headers
      }
```

---

### 4. 响应转换脚本 ⭐⭐ (中优先级)

**位置**: 在返回给客户端之前

**用途**:
- 修改响应格式
- 添加/删除字段
- 错误信息标准化

**脚本输入**:
```lua
request = {
  id = "req_abc123",
  user_id = "user_001"
}

response = {
  status_code = 200,
  body = {
    id = "chatcmpl-xxx",
    model = "llama-3-70b-instruct",
    choices = {...},
    usage = {
      prompt_tokens = 15,
      completion_tokens = 42,
      total_tokens = 57
    }
  },
  headers = {
    ["Content-Type"] = "application/json"
  }
}

backend_url = "http://backend:8000"
latency_ms = 1234
```

**脚本输出**:
```lua
-- 返回修改后的响应
return {
  status_code = 200,
  body = {
    id = response.body.id,
    model = "gpt-4",  -- 映射回原始模型名
    choices = response.body.choices,
    usage = response.body.usage,
    -- 添加自定义字段
    metadata = {
      backend = backend_url,
      latency_ms = latency_ms
    }
  },
  headers = response.headers
}

-- 或返回 nil 表示不修改
return nil
```

**配置示例**:
```yaml
routing:
  response_transform:
    enabled: true
    script: |
      -- 模型名称反向映射
      local model_map = {
        ["llama-3-70b-instruct"] = "gpt-4",
        ["qwen-72b-chat"] = "gpt-3.5"
      }
      
      local model = response.body.model
      local original_model = model_map[model] or model
      
      -- 修改响应体
      local new_body = {}
      for k, v in pairs(response.body) do
        new_body[k] = v
      end
      new_body.model = original_model
      
      return {
        status_code = response.status_code,
        body = new_body,
        headers = response.headers
      }
```

---

### 5. 限流决策脚本 ⭐⭐ (中优先级)

**位置**: 在标准限流检查之后

**用途**:
- 自定义限流逻辑
- 动态调整限流阈值
- 基于业务规则的限流

**脚本输入**:
```lua
request = {
  id = "req_abc123",
  body = {...},
  user_id = "user_001",
  api_key = "sk-xxx"
}

key_info = {
  user_id = "user_001",
  total_quota = 100000,
  used_quota = 50000
}

rate_limit_status = {
  global_remaining = 950,
  key_remaining = 45,
  concurrent_count = 3
}

-- 当前时间
current_time = {
  hour = 14,  -- 14点
  weekday = 3,  -- 周三
  timestamp = 1705392000
}
```

**脚本输出**:
```lua
-- 允许请求
return {
  allow = true
}

-- 拒绝请求
return {
  allow = false,
  reason = "高峰期限流",
  retry_after = 60  -- 秒
}

-- 返回 nil 使用标准限流逻辑
return nil
```

**配置示例**:
```yaml
rate_limit:
  enabled: true
  script: |
    -- 高峰期（9-18点）更严格的限流
    local hour = current_time.hour
    
    if hour >= 9 and hour <= 18 then
      -- 高峰期：普通用户限流更严格
      if key_info.total_quota < 1000000 then
        if rate_limit_status.key_remaining < 10 then
          return {
            allow = false,
            reason = "高峰期限流，请稍后重试"
          }
        end
      end
    end
    
    -- 使用标准限流逻辑
    return nil
```

---

### 6. 用量计算脚本 ⭐ (低优先级)

**位置**: 在用量上报之前

**用途**:
- 自定义计费规则
- 用量数据增强
- 计费策略调整

**脚本输入**:
```lua
request = {
  id = "req_abc123",
  body = {...},
  user_id = "user_001",
  api_key = "sk-xxx"
}

usage = {
  prompt_tokens = 15,
  completion_tokens = 42,
  total_tokens = 57
}

response = {
  status_code = 200,
  body = {...}
}

metadata = {
  backend_url = "http://backend:8000",
  latency_ms = 1234,
  is_stream = true
}
```

**脚本输出**:
```lua
-- 返回修改后的用量数据
return {
  prompt_tokens = usage.prompt_tokens,
  completion_tokens = usage.completion_tokens,
  total_tokens = usage.total_tokens,
  -- 自定义计费
  billable_tokens = usage.total_tokens * 1.5,  -- VIP 用户打折
  cost = usage.total_tokens * 0.0001,  -- 计算成本
  -- 添加自定义字段
  custom_fields = {
    model_type = "large",
    priority = "high"
  }
}
```

**配置示例**:
```yaml
usage_hook:
  enabled: true
  script: |
    -- VIP 用户打折
    local viplevel = request.body.viplevel or 0
    local discount = 1.0
    
    if viplevel >= 3 then
      discount = 0.5  -- 5折
    elseif viplevel >= 1 then
      discount = 0.8  -- 8折
    end
    
    return {
      prompt_tokens = usage.prompt_tokens,
      completion_tokens = usage.completion_tokens,
      total_tokens = usage.total_tokens,
      billable_tokens = math.floor(usage.total_tokens * discount),
      discount = discount,
      viplevel = viplevel
    }
```

---

### 7. 错误处理脚本 ⭐ (低优先级)

**位置**: 请求失败时

**用途**:
- 自定义错误响应
- 错误信息转换
- 降级处理

**脚本输入**:
```lua
request = {
  id = "req_abc123",
  body = {...},
  user_id = "user_001"
}

error = {
  type = "backend_error",  -- auth_error, rate_limit, backend_error, timeout
  message = "连接超时",
  status_code = 504,
  backend_url = "http://backend:8000",
  retry_count = 3
}
```

**脚本输出**:
```lua
-- 返回自定义错误响应
return {
  status_code = 503,
  body = {
    error = {
      message = "服务暂时不可用，请稍后重试",
      type = "service_unavailable",
      code = "SERVICE_UNAVAILABLE"
    }
  }
}

-- 返回 nil 使用默认错误响应
return nil
```

**配置示例**:
```yaml
error_handler:
  enabled: true
  script: |
    -- 统一错误格式
    local error_messages = {
      backend_error = "后端服务异常",
      timeout = "请求超时",
      rate_limit = "请求过于频繁"
    }
    
    local message = error_messages[error.type] or "未知错误"
    
    return {
      status_code = error.status_code,
      body = {
        error = {
          message = message,
          type = error.type,
          request_id = request.id
        }
      }
    }
```

---

## 统一配置格式

```yaml
# config.yaml
listen: ":8000"

backends:
  - url: "http://standard-backend:8000"
    name: "standard"
  - url: "http://high-performance-backend:8000"
    name: "high-performance"

# Lua 脚本配置
scripts:
  # 路由决策
  routing:
    enabled: true
    script_file: "/etc/llmproxy/scripts/routing.lua"
    # 或内联脚本
    # script: |
    #   return "standard"
  
  # 鉴权决策
  auth:
    enabled: true
    script_file: "/etc/llmproxy/scripts/auth.lua"
  
  # 请求转换
  request_transform:
    enabled: false
    script_file: "/etc/llmproxy/scripts/request_transform.lua"
  
  # 响应转换
  response_transform:
    enabled: false
    script_file: "/etc/llmproxy/scripts/response_transform.lua"
  
  # 限流决策
  rate_limit:
    enabled: false
    script_file: "/etc/llmproxy/scripts/rate_limit.lua"
  
  # 用量计算
  usage:
    enabled: false
    script_file: "/etc/llmproxy/scripts/usage.lua"
  
  # 错误处理
  error_handler:
    enabled: false
    script_file: "/etc/llmproxy/scripts/error_handler.lua"
```

---

## Lua 脚本公共库

为了方便用户编写脚本，提供一些公共函数：

```lua
-- 内置工具函数（由网关提供）

-- JSON 编解码
json = {
  encode = function(obj) end,
  decode = function(str) end
}

-- 字符串工具
string_utils = {
  split = function(str, delimiter) end,
  trim = function(str) end,
  starts_with = function(str, prefix) end,
  ends_with = function(str, suffix) end
}

-- 时间工具
time = {
  now = function() end,  -- 返回当前时间戳
  format = function(timestamp, format) end,
  parse = function(str, format) end
}

-- 哈希工具
hash = {
  md5 = function(str) end,
  sha256 = function(str) end
}

-- 日志工具
log = {
  info = function(msg) end,
  warn = function(msg) end,
  error = function(msg) end
}
```

---

## 实现优先级

### Phase 1: 核心脚本（必须实现）
1. **路由决策脚本** - 最重要，解决复杂路由问题
2. **鉴权决策脚本** - 支持自定义权限控制

### Phase 2: 转换脚本（推荐实现）
3. **请求转换脚本** - 参数映射和标准化
4. **响应转换脚本** - 响应格式统一

### Phase 3: 高级脚本（可选实现）
5. **限流决策脚本** - 动态限流策略
6. **用量计算脚本** - 自定义计费规则
7. **错误处理脚本** - 错误响应标准化

---

## 性能考虑

1. **脚本缓存**: 脚本加载后缓存，避免重复解析
2. **VM 池**: 使用 Lua VM 池，避免频繁创建销毁
3. **超时控制**: 脚本执行超时自动终止（默认 100ms）
4. **内存限制**: 限制脚本可用内存（默认 10MB）
5. **沙箱隔离**: 禁止访问文件系统、网络等危险操作

---

## 安全考虑

1. **沙箱环境**: 脚本运行在受限环境中
2. **禁止危险操作**: 
   - 禁止 `os.execute`、`io.open` 等系统调用
   - 禁止 `require` 加载外部模块
   - 禁止无限循环
3. **资源限制**: 
   - CPU 时间限制
   - 内存使用限制
   - 递归深度限制

---

## 总结

通过 Lua 脚本系统，LLMProxy 可以在保持核心简单的同时，提供强大的可编程能力。建议优先实现**路由决策**和**鉴权决策**两个核心脚本，这两个可以解决 90% 的实际需求。
