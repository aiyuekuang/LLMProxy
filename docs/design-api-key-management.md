# API Key 管理与鉴权设计方案

## 一、功能概述

实现虚拟 API Key 管理系统，支持细粒度的访问控制、额度管理和权限管理。

## 二、核心功能

### 2.1 虚拟 Key 管理
- Key 生成（格式：`sk-llmproxy-{32位随机字符}`）
- Key 元数据存储（名称、额度、权限、过期时间等）
- Key 的启用/禁用
- Key 的删除

### 2.2 访问控制
- IP 白名单/黑名单
- 模型访问权限控制
- 后端访问权限控制
- 请求路径限制

### 2.3 额度管理
- Token 额度设置（prompt + completion）
- 额度消耗追踪
- 额度不足拦截
- 额度重置（按周期）

### 2.4 鉴权流程
- 请求拦截
- Key 验证
- 权限检查
- 额度检查

## 三、数据结构设计

### 3.1 APIKey 结构
```go
type APIKey struct {
    Key              string    `json:"key"`               // API Key
    Name             string    `json:"name"`              // Key 名称
    UserID           string    `json:"user_id"`           // 所属用户
    Status           string    `json:"status"`            // 状态：active/disabled
    
    // 额度管理
    TotalQuota       int64     `json:"total_quota"`       // 总额度（tokens）
    UsedQuota        int64     `json:"used_quota"`        // 已使用额度
    QuotaResetPeriod string    `json:"quota_reset_period"` // 重置周期：daily/weekly/monthly/never
    LastResetAt      time.Time `json:"last_reset_at"`     // 上次重置时间
    
    // 访问控制
    AllowedModels    []string  `json:"allowed_models"`    // 允许的模型列表，空表示全部
    AllowedBackends  []string  `json:"allowed_backends"`  // 允许的后端列表，空表示全部
    AllowedIPs       []string  `json:"allowed_ips"`       // IP 白名单（CIDR 格式）
    DeniedIPs        []string  `json:"denied_ips"`        // IP 黑名单（CIDR 格式）
    
    // 时间管理
    ExpiresAt        *time.Time `json:"expires_at"`       // 过期时间，nil 表示永不过期
    CreatedAt        time.Time  `json:"created_at"`       // 创建时间
    UpdatedAt        time.Time  `json:"updated_at"`       // 更新时间
}
```

### 3.2 存储方案
- **配置文件存储**（初期）：YAML 格式，适合小规模部署
- **Redis 存储**（推荐）：高性能，支持分布式
- **数据库存储**（可选）：MySQL/PostgreSQL，适合大规模部署

## 四、实现方案

### 4.1 配置文件方式
```yaml
# config.yaml
api_keys:
  - key: "sk-llmproxy-abc123..."
    name: "项目A"
    user_id: "user_001"
    status: "active"
    total_quota: 1000000
    used_quota: 0
    quota_reset_period: "monthly"
    allowed_models: ["gpt-4", "claude-3-opus"]
    allowed_ips: ["192.168.1.0/24", "10.0.0.1"]
    expires_at: "2026-12-31T23:59:59Z"
  
  - key: "sk-llmproxy-def456..."
    name: "测试环境"
    user_id: "user_002"
    status: "active"
    total_quota: 100000
    quota_reset_period: "daily"
```

### 4.2 Redis 存储方案
```
# Key 结构
apikey:{key} -> JSON(APIKey)

# 索引
apikey:user:{user_id} -> Set[key1, key2, ...]
apikey:active -> Set[key1, key2, ...]

# 额度缓存（快速检查）
apikey:quota:{key} -> used_quota
```

### 4.3 鉴权中间件
```go
func AuthMiddleware(keyStore KeyStore) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 提取 API Key
        apiKey := extractAPIKey(c)
        if apiKey == "" {
            c.JSON(401, gin.H{"error": "Missing API Key"})
            c.Abort()
            return
        }
        
        // 2. 验证 Key 是否存在
        key, err := keyStore.Get(apiKey)
        if err != nil {
            c.JSON(401, gin.H{"error": "Invalid API Key"})
            c.Abort()
            return
        }
        
        // 3. 检查状态
        if key.Status != "active" {
            c.JSON(403, gin.H{"error": "API Key is disabled"})
            c.Abort()
            return
        }
        
        // 4. 检查过期时间
        if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
            c.JSON(403, gin.H{"error": "API Key has expired"})
            c.Abort()
            return
        }
        
        // 5. 检查 IP 白名单/黑名单
        clientIP := c.ClientIP()
        if !checkIPAllowed(clientIP, key.AllowedIPs, key.DeniedIPs) {
            c.JSON(403, gin.H{"error": "IP not allowed"})
            c.Abort()
            return
        }
        
        // 6. 检查额度
        if key.TotalQuota > 0 && key.UsedQuota >= key.TotalQuota {
            c.JSON(429, gin.H{"error": "Quota exceeded"})
            c.Abort()
            return
        }
        
        // 7. 将 Key 信息存入上下文
        c.Set("api_key", key)
        c.Next()
    }
}
```

### 4.4 额度扣减
```go
func DeductQuota(keyStore KeyStore, apiKey string, tokens int64) error {
    // 原子操作扣减额度
    return keyStore.IncrementUsedQuota(apiKey, tokens)
}
```

### 4.5 额度重置
```go
func ResetQuotaIfNeeded(key *APIKey) bool {
    if key.QuotaResetPeriod == "never" {
        return false
    }
    
    now := time.Now()
    var shouldReset bool
    
    switch key.QuotaResetPeriod {
    case "daily":
        shouldReset = now.Sub(key.LastResetAt) >= 24*time.Hour
    case "weekly":
        shouldReset = now.Sub(key.LastResetAt) >= 7*24*time.Hour
    case "monthly":
        shouldReset = now.Month() != key.LastResetAt.Month()
    }
    
    if shouldReset {
        key.UsedQuota = 0
        key.LastResetAt = now
        return true
    }
    return false
}
```

## 五、API 接口设计

### 5.1 管理 API（需要管理员权限）

#### 创建 API Key
```
POST /admin/api-keys
Authorization: Bearer {admin_token}

Request:
{
  "name": "项目A",
  "user_id": "user_001",
  "total_quota": 1000000,
  "quota_reset_period": "monthly",
  "allowed_models": ["gpt-4"],
  "allowed_ips": ["192.168.1.0/24"],
  "expires_at": "2026-12-31T23:59:59Z"
}

Response:
{
  "key": "sk-llmproxy-abc123...",
  "name": "项目A",
  "created_at": "2026-01-14T10:00:00Z"
}
```

#### 列出 API Keys
```
GET /admin/api-keys?user_id=user_001&status=active
Authorization: Bearer {admin_token}

Response:
{
  "keys": [
    {
      "key": "sk-llmproxy-abc123...",
      "name": "项目A",
      "status": "active",
      "total_quota": 1000000,
      "used_quota": 50000,
      "created_at": "2026-01-14T10:00:00Z"
    }
  ]
}
```

#### 更新 API Key
```
PATCH /admin/api-keys/{key}
Authorization: Bearer {admin_token}

Request:
{
  "status": "disabled",
  "total_quota": 2000000
}
```

#### 删除 API Key
```
DELETE /admin/api-keys/{key}
Authorization: Bearer {admin_token}
```

#### 查询 Key 使用情况
```
GET /admin/api-keys/{key}/usage
Authorization: Bearer {admin_token}

Response:
{
  "key": "sk-llmproxy-abc123...",
  "total_quota": 1000000,
  "used_quota": 50000,
  "remaining_quota": 950000,
  "usage_percentage": 5.0,
  "last_used_at": "2026-01-14T12:00:00Z"
}
```

## 六、配置示例

```yaml
# config.yaml
auth:
  enabled: true
  storage: "redis"  # 或 "file" 或 "mysql"
  
  # Redis 配置
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
  
  # 管理员配置
  admin:
    enabled: true
    token: "admin-secret-token-change-me"
  
  # 默认配置
  defaults:
    quota_reset_period: "monthly"
    total_quota: 1000000

# API Keys（仅当 storage=file 时使用）
api_keys:
  - key: "sk-llmproxy-test123"
    name: "测试 Key"
    status: "active"
    total_quota: 100000
```

## 七、实现优先级

### Phase 1（核心功能）
- [x] Key 验证中间件
- [x] 基于配置文件的 Key 存储
- [x] 基本鉴权（Key 验证、状态检查）
- [x] 额度检查与扣减

### Phase 2（访问控制）
- [ ] IP 白名单/黑名单
- [ ] 模型访问权限
- [ ] 过期时间检查

### Phase 3（高级功能）
- [ ] Redis 存储支持
- [ ] 管理 API
- [ ] 额度自动重置
- [ ] 使用统计

## 八、安全考虑

1. **Key 格式**：使用 `sk-llmproxy-` 前缀，便于识别和防止泄露
2. **传输安全**：强制使用 HTTPS
3. **存储安全**：Key 不加密存储（因为需要原文比对），但配置文件权限需严格控制
4. **日志脱敏**：日志中只记录 Key 的前 8 位
5. **管理员权限**：管理 API 需要独立的 admin token

## 九、监控指标

```go
// Prometheus 指标
apikey_requests_total{key_prefix, status}  // 请求总数
apikey_quota_used{key_prefix}              // 额度使用量
apikey_auth_failures_total{reason}         // 鉴权失败次数
apikey_active_keys                         // 活跃 Key 数量
```

## 十、测试用例

1. 正常请求（有效 Key）
2. 无效 Key
3. 已禁用的 Key
4. 已过期的 Key
5. 额度不足
6. IP 不在白名单
7. 模型权限不足
8. 并发请求下的额度扣减

---

**设计版本：** v1.0  
**创建时间：** 2026-01-14
