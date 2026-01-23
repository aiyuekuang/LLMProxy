# 鉴权管道（Auth Pipeline）

LLMProxy 支持可编排的多源鉴权管道，用户可以自由组合配置文件、Redis、数据库、Webhook 等多种鉴权方式，并通过 Lua 脚本自定义决策逻辑。

## 核心特性

- **多数据源支持**：配置文件 / Redis / 数据库 / Webhook
- **可编排**：自由调整 Provider 顺序
- **Lua 脚本**：自定义决策逻辑，返回放行/拒绝及错误消息
- **两种模式**：`first_match`（首个成功即放行）或 `all`（全部通过才放行）
- **JSON 错误响应**：标准化错误返回格式

## 配置结构

```yaml
auth:
  enabled: true
  header_names:
    - "Authorization"
    - "X-API-Key"
  mode: "first_match"  # first_match | all
  
  pipeline:
    # Provider 1: 配置文件
    - name: "config_file"
      type: "file"
      enabled: true
      lua_script: |
        if data.status ~= "active" then
          return {allow = false, message = "Key 已禁用"}
        end
        return {allow = true}
    
    # Provider 2: Redis
    - name: "redis_auth"
      type: "redis"
      enabled: true
      redis:
        addr: "localhost:6379"
        password: ""
        db: 0
        key_pattern: "llmproxy:key:{api_key}"
      lua_script: |
        if tonumber(data.balance) <= 0 then
          return {allow = false, message = "余额不足，请充值"}
        end
        return {allow = true}
    
    # Provider 3: 数据库
    - name: "db_auth"
      type: "database"
      enabled: false
      database:
        driver: "mysql"  # mysql / postgres / sqlite
        dsn: "user:pass@tcp(localhost:3306)/llmproxy"
        table: "api_keys"
        key_column: "api_key"
        fields: ["user_id", "status", "balance", "expired_at"]
      lua_script: |
        if data.expired_at and data.expired_at < now() then
          return {allow = false, message = "Key 已过期"}
        end
        return {allow = true}
    
    # Provider 4: Webhook
    - name: "webhook_auth"
      type: "webhook"
      enabled: false
      webhook:
        url: "https://api.example.com/auth/verify"
        method: "POST"
        timeout: 3s
        headers:
          X-Internal-Token: "your-secret-token"
      lua_script: |
        if data.code ~= 0 then
          return {allow = false, message = data.message or "验证失败"}
        end
        return {allow = true, metadata = {user_id = data.user_id}}
```

## Provider 类型

### 1. File（配置文件）

从 `config.yaml` 中的 `api_keys` 列表读取。

```yaml
- name: "config_file"
  type: "file"
  enabled: true
```

无需额外配置，直接使用主配置中的 `api_keys`。

### 2. Redis

从 Redis 读取 Key 信息，支持 Hash 和 String（JSON）两种格式。

```yaml
- name: "redis_auth"
  type: "redis"
  enabled: true
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
    key_pattern: "llmproxy:key:{api_key}"  # {api_key} 会被替换为实际的 Key
```

**业务系统写入示例**：

```bash
# Hash 格式
HSET llmproxy:key:sk-test123 status "active" balance "1000" user_id "user_001"

# String (JSON) 格式
SET llmproxy:key:sk-test123 '{"status":"active","balance":1000,"user_id":"user_001"}'
```

### 3. Database（数据库）

支持 MySQL、PostgreSQL、SQLite。

```yaml
- name: "db_auth"
  type: "database"
  enabled: true
  database:
    driver: "mysql"          # mysql / postgres / sqlite
    dsn: "user:pass@tcp(localhost:3306)/llmproxy"
    table: "api_keys"
    key_column: "api_key"    # API Key 所在列
    fields:                  # 需要查询的字段
      - "user_id"
      - "status"
      - "balance"
      - "expired_at"
```

**表结构示例**：

```sql
CREATE TABLE api_keys (
    id INT AUTO_INCREMENT PRIMARY KEY,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    status VARCHAR(16) DEFAULT 'active',
    balance DECIMAL(10,2) DEFAULT 0,
    expired_at DATETIME NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 4. Webhook

调用外部 HTTP 服务验证。

```yaml
- name: "webhook_auth"
  type: "webhook"
  enabled: true
  webhook:
    url: "https://api.example.com/auth/verify"
    method: "POST"
    timeout: 3s
    headers:
      X-Internal-Token: "your-secret-token"
```

**请求格式**：

```json
{
  "api_key": "sk-xxx",
  "timestamp": 1705555555
}
```

**期望响应**：

```json
{
  "code": 0,
  "message": "success",
  "user_id": "user_001",
  "balance": 1000
}
```

### 5. Builtin（内置 SQLite 存储）

使用 Admin API 管理的 SQLite 数据库，**需要同时启用 `admin.enabled: true`**。

```yaml
- name: "builtin_auth"
  type: "builtin"
  enabled: true
```

**特点**：
- 无需外部数据库依赖
- 通过 Admin API 管理 Key（创建/更新/删除/同步）
- 数据持久化到本地 SQLite
- 适合单机部署或开发环境

**返回的数据格式**：

```lua
-- data 包含以下字段（如果存在）：
data.key        -- API Key
data.status     -- 状态：0=active, 1=disabled, 2=quota_exceeded, 3=expired
data.name       -- 名称/备注
data.user_id    -- 用户 ID
data.starts_at  -- 生效时间（Unix 时间戳）
data.expires_at -- 过期时间（Unix 时间戳）
data.created_at -- 创建时间（Unix 时间戳）
data.updated_at -- 更新时间（Unix 时间戳）
```

**配合 Admin API 使用**：

```yaml
admin:
  enabled: true
  token: "your-secure-admin-token"
  db_path: "./data/keys.db"

auth:
  enabled: true
  header_names: ["Authorization", "X-API-Key"]
  mode: "first_match"
  pipeline:
    - name: "builtin_auth"
      type: "builtin"
      enabled: true
```

## Lua 脚本

每个 Provider 可以配置 Lua 脚本来自定义决策逻辑。

### 可用变量

| 变量 | 类型 | 说明 |
|------|------|------|
| `api_key` | string | 当前请求的 API Key |
| `data` | table | 从 Provider 查询到的数据 |
| `key` | table | `data` 的别名 |
| `request` | table | 请求信息 |
| `request.method` | string | HTTP 方法 |
| `request.path` | string | 请求路径 |
| `request.ip` | string | 客户端 IP |
| `request.headers` | table | 请求头 |
| `metadata` | table | 累积的元数据 |

### 可用函数

| 函数 | 说明 |
|------|------|
| `now()` | 返回当前时间戳（秒） |
| `now_ms()` | 返回当前时间戳（毫秒） |
| `log(msg)` | 打印日志 |

### 返回格式

```lua
return {
  allow = true,              -- 是否允许（必需）
  message = "错误原因",       -- 拒绝时的错误消息（可选）
  metadata = {               -- 附加元数据（可选）
    user_id = "user_001"
  }
}
```

### 脚本示例

**检查状态和额度**：

```lua
if data.status ~= "active" then
  return {allow = false, message = "Key 已禁用"}
end

if tonumber(data.used_quota) >= tonumber(data.total_quota) then
  return {allow = false, message = "额度不足"}
end

return {allow = true}
```

**检查过期时间**：

```lua
if data.expired_at and tonumber(data.expired_at) < now() then
  return {allow = false, message = "Key 已过期"}
end
return {allow = true}
```

**IP 白名单**：

```lua
local allowed_ips = {"192.168.1.100", "10.0.0.1"}
for _, ip in ipairs(allowed_ips) do
  if request.ip == ip then
    return {allow = true}
  end
end
return {allow = false, message = "IP 不在白名单中"}
```

**从文件加载脚本**：

```yaml
- name: "custom_auth"
  type: "file"
  enabled: true
  lua_script_file: "./scripts/auth.lua"
```

## 管道模式

### first_match（默认）

第一个 Provider 验证成功即放行，任何一个返回 `allow=false` 立即拒绝。

适用场景：多个数据源，Key 可能存在于任意一个。

### all

所有 Provider 都必须验证通过才放行。

适用场景：多重验证，如先查 Redis 检查余额，再调 Webhook 做风控。

## 错误响应格式

当鉴权失败时，返回 JSON 格式错误：

```json
{
  "error": "余额不足，请充值",
  "code": 403
}
```

## 兼容旧配置

如果不配置 `pipeline`，将自动使用旧的 `storage: file` 模式：

```yaml
auth:
  enabled: true
  storage: "file"  # 兼容旧配置

api_keys:
  - key: "sk-test123"
    name: "测试 Key"
    status: "active"
```

## 完整配置示例

```yaml
listen: ":8000"

backends:
  - url: "http://localhost:8001"
    weight: 1

auth:
  enabled: true
  header_names: ["Authorization", "X-API-Key"]
  mode: "first_match"
  
  pipeline:
    # 1. 先查配置文件（开发测试用）
    - name: "config_file"
      type: "file"
      enabled: true
      lua_script: |
        if data.status ~= "active" then
          return {allow = false, message = "Key 已禁用"}
        end
        return {allow = true, metadata = {user_id = data.user_id}}
    
    # 2. 再查 Redis（生产环境）
    - name: "redis_production"
      type: "redis"
      enabled: true
      redis:
        addr: "redis:6379"
        key_pattern: "llmproxy:key:{api_key}"
      lua_script: |
        if tonumber(data.balance or 0) <= 0 then
          return {allow = false, message = "余额不足，请充值"}
        end
        return {allow = true, metadata = {user_id = data.user_id, balance = data.balance}}

# 配置文件中的 API Keys（用于 file provider）
api_keys:
  - key: "sk-dev-123"
    name: "开发测试 Key"
    user_id: "dev_user"
    status: "active"
```
