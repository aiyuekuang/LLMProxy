# 限流与速率控制设计方案

## 一、功能概述

实现多层级的限流控制，防止 API 滥用、保护后端服务、控制成本。

## 二、限流维度

### 2.1 全局限流
- 整个系统的 QPS/QPM 限制
- 保护后端服务不被打垮

### 2.2 API Key 级限流
- 每个 Key 的请求速率限制
- 每个 Key 的 Token 速率限制（TPM）
- 并发请求数限制

### 2.3 用户级限流
- 每个用户的总请求速率
- 跨多个 Key 的聚合限流

### 2.4 模型级限流
- 特定模型的请求速率限制
- 昂贵模型的特殊限制

### 2.5 后端级限流
- 每个后端的请求速率限制
- 保护后端不被单一来源打爆

## 三、限流算法

### 3.1 令牌桶算法（推荐）
- 支持突发流量
- 平滑限流
- 适合大部分场景

### 3.2 滑动窗口算法
- 更精确的限流
- 防止窗口边界突发
- 内存开销较大

### 3.3 固定窗口算法
- 实现简单
- 可能有窗口边界问题
- 适合粗粒度限流

## 四、数据结构设计

### 4.1 限流配置
```go
type RateLimitConfig struct {
    // 全局限流
    Global GlobalRateLimit `yaml:"global"`
    
    // API Key 级限流
    PerKey KeyRateLimit `yaml:"per_key"`
    
    // 用户级限流
    PerUser UserRateLimit `yaml:"per_user"`
    
    // 模型级限流
    PerModel map[string]ModelRateLimit `yaml:"per_model"`
    
    // 后端级限流
    PerBackend map[string]BackendRateLimit `yaml:"per_backend"`
}

type GlobalRateLimit struct {
    Enabled           bool  `yaml:"enabled"`
    RequestsPerSecond int   `yaml:"requests_per_second"`
    RequestsPerMinute int   `yaml:"requests_per_minute"`
    BurstSize         int   `yaml:"burst_size"`  // 突发容量
}

type KeyRateLimit struct {
    Enabled           bool  `yaml:"enabled"`
    RequestsPerSecond int   `yaml:"requests_per_second"`
    RequestsPerMinute int   `yaml:"requests_per_minute"`
    TokensPerMinute   int64 `yaml:"tokens_per_minute"`  // TPM 限制
    MaxConcurrent     int   `yaml:"max_concurrent"`     // 最大并发数
    BurstSize         int   `yaml:"burst_size"`
}

type ModelRateLimit struct {
    RequestsPerMinute int `yaml:"requests_per_minute"`
    TokensPerMinute   int64 `yaml:"tokens_per_minute"`
}
```

### 4.2 限流器接口
```go
type RateLimiter interface {
    // 检查是否允许请求
    Allow(key string) (bool, error)
    
    // 检查是否允许指定数量的 tokens
    AllowN(key string, n int64) (bool, error)
    
    // 获取剩余配额
    Remaining(key string) (int64, error)
    
    // 重置限流器
    Reset(key string) error
}
```

## 五、实现方案

### 5.1 基于 Redis 的令牌桶实现

#### Lua 脚本（原子操作）
```lua
-- rate_limit.lua
-- KEYS[1]: 限流 key
-- ARGV[1]: 最大令牌数（桶容量）
-- ARGV[2]: 令牌生成速率（每秒）
-- ARGV[3]: 当前时间戳
-- ARGV[4]: 请求消耗的令牌数

local key = KEYS[1]
local max_tokens = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

-- 获取当前状态
local state = redis.call('HMGET', key, 'tokens', 'last_update')
local tokens = tonumber(state[1]) or max_tokens
local last_update = tonumber(state[2]) or now

-- 计算新增令牌
local elapsed = now - last_update
local new_tokens = math.min(max_tokens, tokens + elapsed * rate)

-- 检查是否有足够令牌
if new_tokens >= requested then
    new_tokens = new_tokens - requested
    redis.call('HMSET', key, 'tokens', new_tokens, 'last_update', now)
    redis.call('EXPIRE', key, 60)  -- 60秒过期
    return {1, new_tokens}  -- 允许请求，返回剩余令牌
else
    redis.call('HMSET', key, 'tokens', new_tokens, 'last_update', now)
    redis.call('EXPIRE', key, 60)
    return {0, new_tokens}  -- 拒绝请求，返回剩余令牌
end
```

#### Go 实现
```go
type RedisRateLimiter struct {
    client *redis.Client
    script *redis.Script
}

func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
    script := redis.NewScript(rateLimitLuaScript)
    return &RedisRateLimiter{
        client: client,
        script: script,
    }
}

func (r *RedisRateLimiter) AllowN(key string, maxTokens, rate int64, n int64) (bool, int64, error) {
    now := time.Now().Unix()
    
    result, err := r.script.Run(
        context.Background(),
        r.client,
        []string{key},
        maxTokens,
        rate,
        now,
        n,
    ).Result()
    
    if err != nil {
        return false, 0, err
    }
    
    res := result.([]interface{})
    allowed := res[0].(int64) == 1
    remaining := res[1].(int64)
    
    return allowed, remaining, nil
}
```

### 5.2 限流中间件

```go
func RateLimitMiddleware(limiter *RateLimiter, config *RateLimitConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 全局限流
        if config.Global.Enabled {
            allowed, _, err := limiter.AllowN(
                "global",
                int64(config.Global.BurstSize),
                int64(config.Global.RequestsPerSecond),
                1,
            )
            if err != nil || !allowed {
                c.Header("X-RateLimit-Limit", strconv.Itoa(config.Global.RequestsPerSecond))
                c.Header("X-RateLimit-Remaining", "0")
                c.Header("Retry-After", "1")
                c.JSON(429, gin.H{
                    "error": "Global rate limit exceeded",
                    "message": "Too many requests, please try again later",
                })
                c.Abort()
                return
            }
        }
        
        // 2. API Key 级限流
        apiKey, exists := c.Get("api_key")
        if exists && config.PerKey.Enabled {
            key := apiKey.(*APIKey)
            keyLimitKey := fmt.Sprintf("ratelimit:key:%s", key.Key)
            
            // 请求数限流
            allowed, remaining, err := limiter.AllowN(
                keyLimitKey,
                int64(config.PerKey.BurstSize),
                int64(config.PerKey.RequestsPerSecond),
                1,
            )
            
            c.Header("X-RateLimit-Limit", strconv.Itoa(config.PerKey.RequestsPerSecond))
            c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
            
            if err != nil || !allowed {
                c.Header("Retry-After", "1")
                c.JSON(429, gin.H{
                    "error": "Rate limit exceeded",
                    "message": "Too many requests for this API key",
                })
                c.Abort()
                return
            }
            
            // 并发数限流
            if config.PerKey.MaxConcurrent > 0 {
                concurrentKey := fmt.Sprintf("concurrent:key:%s", key.Key)
                current, err := limiter.IncrementConcurrent(concurrentKey)
                if err != nil || current > int64(config.PerKey.MaxConcurrent) {
                    limiter.DecrementConcurrent(concurrentKey)
                    c.JSON(429, gin.H{
                        "error": "Concurrent limit exceeded",
                        "message": "Too many concurrent requests",
                    })
                    c.Abort()
                    return
                }
                
                // 请求结束后减少并发计数
                defer limiter.DecrementConcurrent(concurrentKey)
            }
        }
        
        // 3. 模型级限流（在请求解析后检查）
        c.Next()
    }
}
```

### 5.3 Token 级限流（TPM）

```go
func TokenRateLimitMiddleware(limiter *RateLimiter, config *RateLimitConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 在响应后执行（需要知道实际消耗的 tokens）
        c.Next()
        
        // 获取消耗的 tokens
        tokens, exists := c.Get("tokens_used")
        if !exists {
            return
        }
        
        apiKey, exists := c.Get("api_key")
        if !exists || !config.PerKey.Enabled {
            return
        }
        
        key := apiKey.(*APIKey)
        tpmKey := fmt.Sprintf("ratelimit:tpm:%s", key.Key)
        
        // 检查 TPM 限制
        allowed, _, err := limiter.AllowN(
            tpmKey,
            config.PerKey.TokensPerMinute,
            config.PerKey.TokensPerMinute/60,  // 每秒速率
            tokens.(int64),
        )
        
        if err != nil || !allowed {
            // 记录超限日志
            log.Warnf("TPM limit exceeded for key %s", key.Key[:8])
        }
    }
}
```

### 5.4 滑动窗口实现（备选）

```go
type SlidingWindowLimiter struct {
    client *redis.Client
}

func (s *SlidingWindowLimiter) Allow(key string, limit int, window time.Duration) (bool, error) {
    now := time.Now().UnixNano()
    windowStart := now - window.Nanoseconds()
    
    pipe := s.client.Pipeline()
    
    // 移除过期的记录
    pipe.ZRemRangeByScore(context.Background(), key, "0", strconv.FormatInt(windowStart, 10))
    
    // 统计当前窗口内的请求数
    pipe.ZCard(context.Background(), key)
    
    // 添加当前请求
    pipe.ZAdd(context.Background(), key, &redis.Z{
        Score:  float64(now),
        Member: now,
    })
    
    // 设置过期时间
    pipe.Expire(context.Background(), key, window)
    
    _, err := pipe.Exec(context.Background())
    if err != nil {
        return false, err
    }
    
    count, err := s.client.ZCard(context.Background(), key).Result()
    if err != nil {
        return false, err
    }
    
    return count <= int64(limit), nil
}
```

## 六、配置示例

```yaml
# config.yaml
rate_limit:
  enabled: true
  storage: "redis"  # 或 "memory"
  
  # Redis 配置
  redis:
    addr: "localhost:6379"
    password: ""
    db: 1
  
  # 全局限流
  global:
    enabled: true
    requests_per_second: 1000
    requests_per_minute: 50000
    burst_size: 2000  # 允许突发到 2000 QPS
  
  # API Key 级限流
  per_key:
    enabled: true
    requests_per_second: 10
    requests_per_minute: 500
    tokens_per_minute: 100000  # TPM 限制
    max_concurrent: 5          # 最大并发数
    burst_size: 20
  
  # 用户级限流
  per_user:
    enabled: true
    requests_per_minute: 1000
  
  # 模型级限流
  per_model:
    gpt-4:
      requests_per_minute: 100
      tokens_per_minute: 50000
    claude-3-opus:
      requests_per_minute: 50
      tokens_per_minute: 30000
  
  # 后端级限流
  per_backend:
    vllm-1:
      requests_per_second: 100
    tgi-1:
      requests_per_second: 50
```

## 七、响应头标准

遵循 HTTP 标准的限流响应头：

```
X-RateLimit-Limit: 100          # 限制值
X-RateLimit-Remaining: 95       # 剩余配额
X-RateLimit-Reset: 1705234567   # 重置时间（Unix 时间戳）
Retry-After: 60                 # 建议重试时间（秒）
```

## 八、监控指标

```go
// Prometheus 指标
ratelimit_requests_total{limiter, status}     // 限流检查总数
ratelimit_rejected_total{limiter, reason}     // 拒绝请求总数
ratelimit_tokens_consumed{key_prefix}         // Token 消耗量
ratelimit_concurrent_requests{key_prefix}     // 当前并发数
```

## 九、错误响应

### 9.1 请求速率超限
```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded",
    "param": null
  }
}
```

### 9.2 Token 速率超限
```json
{
  "error": {
    "message": "Token rate limit exceeded",
    "type": "rate_limit_error",
    "code": "token_rate_limit_exceeded",
    "param": null
  }
}
```

### 9.3 并发数超限
```json
{
  "error": {
    "message": "Too many concurrent requests",
    "type": "rate_limit_error",
    "code": "concurrent_limit_exceeded",
    "param": null
  }
}
```

## 十、实现优先级

### Phase 1（核心功能）
- [x] 全局限流（QPS）
- [x] API Key 级限流（QPS）
- [x] 基于 Redis 的令牌桶实现

### Phase 2（高级功能）
- [ ] Token 级限流（TPM）
- [ ] 并发数限制
- [ ] 模型级限流

### Phase 3（优化）
- [ ] 滑动窗口算法
- [ ] 用户级限流
- [ ] 后端级限流
- [ ] 动态限流调整

## 十一、性能优化

1. **Redis Pipeline**：批量操作减少网络往返
2. **本地缓存**：缓存限流配置，减少 Redis 访问
3. **Lua 脚本**：原子操作，避免竞态条件
4. **异步记录**：限流日志异步写入，不阻塞主流程

## 十二、测试用例

1. 正常请求（未超限）
2. 超过 QPS 限制
3. 超过 QPM 限制
4. 超过 TPM 限制
5. 超过并发数限制
6. 突发流量处理
7. 多个 Key 并发请求
8. 限流重置测试

---

**设计版本：** v1.0  
**创建时间：** 2026-01-14
