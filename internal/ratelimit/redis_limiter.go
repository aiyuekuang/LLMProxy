package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter 基于 Redis 的分布式限流器（令牌桶算法）
type RedisRateLimiter struct {
	client *redis.Client // Redis 客户端
	prefix string        // Key 前缀
}

// NewRedisRateLimiter 创建 Redis 限流器
// 参数：
//   - client: Redis 客户端
//   - prefix: Key 前缀（可选，默认 "ratelimit:"）
// 返回：
//   - RateLimiter: 限流器实例
func NewRedisRateLimiter(client *redis.Client, prefix string) RateLimiter {
	if prefix == "" {
		prefix = "ratelimit:"
	}
	return &RedisRateLimiter{
		client: client,
		prefix: prefix,
	}
}

// Allow 检查是否允许请求（消耗 1 个令牌）
// 参数：
//   - key: 限流 key
// 返回：
//   - bool: 是否允许
//   - error: 错误信息
func (r *RedisRateLimiter) Allow(key string) (bool, error) {
	allowed, _, err := r.AllowN(key, 100, 10, 1)
	return allowed, err
}

// tokenBucketScript Lua 脚本实现令牌桶算法
// KEYS[1]: bucket key
// ARGV[1]: max_tokens（桶容量）
// ARGV[2]: rate（每秒生成速率）
// ARGV[3]: now（当前时间戳，毫秒）
// ARGV[4]: requested（请求的令牌数）
// 返回：allowed(0/1), remaining
const tokenBucketScript = `
local key = KEYS[1]
local max_tokens = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_update')
local tokens = tonumber(bucket[1])
local last_update = tonumber(bucket[2])

if tokens == nil then
    tokens = max_tokens
    last_update = now
end

local elapsed = (now - last_update) / 1000.0
local new_tokens = math.min(max_tokens, tokens + elapsed * rate)

local allowed = 0
if new_tokens >= requested then
    new_tokens = new_tokens - requested
    allowed = 1
end

redis.call('HMSET', key, 'tokens', new_tokens, 'last_update', now)
redis.call('EXPIRE', key, 3600)

return {allowed, math.floor(new_tokens)}
`

// AllowN 检查是否允许指定数量的 tokens
// 参数：
//   - key: 限流 key
//   - maxTokens: 最大令牌数（桶容量）
//   - rate: 令牌生成速率（每秒）
//   - n: 请求消耗的令牌数
// 返回：
//   - bool: 是否允许
//   - int64: 剩余令牌数
//   - error: 错误信息
func (r *RedisRateLimiter) AllowN(key string, maxTokens, rate int64, n int64) (bool, int64, error) {
	ctx := context.Background()
	fullKey := r.prefix + key

	now := time.Now().UnixMilli()

	result, err := r.client.Eval(ctx, tokenBucketScript, []string{fullKey}, maxTokens, rate, now, n).Result()
	if err != nil {
		return false, 0, fmt.Errorf("Redis 限流脚本执行失败: %w", err)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return false, 0, fmt.Errorf("Redis 限流脚本返回值格式错误")
	}

	allowed := values[0].(int64) == 1
	remaining := values[1].(int64)

	return allowed, remaining, nil
}

// Remaining 获取剩余配额
// 参数：
//   - key: 限流 key
// 返回：
//   - int64: 剩余配额
//   - error: 错误信息
func (r *RedisRateLimiter) Remaining(key string) (int64, error) {
	ctx := context.Background()
	fullKey := r.prefix + key

	tokens, err := r.client.HGet(ctx, fullKey, "tokens").Float64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("获取剩余配额失败: %w", err)
	}

	return int64(tokens), nil
}

// IncrementConcurrent 增加并发计数
// 参数：
//   - key: 限流 key
// 返回：
//   - int64: 当前并发数
//   - error: 错误信息
func (r *RedisRateLimiter) IncrementConcurrent(key string) (int64, error) {
	ctx := context.Background()
	fullKey := r.prefix + "concurrent:" + key

	count, err := r.client.Incr(ctx, fullKey).Result()
	if err != nil {
		return 0, fmt.Errorf("增加并发计数失败: %w", err)
	}

	// 设置过期时间，防止泄漏
	r.client.Expire(ctx, fullKey, 5*time.Minute)

	return count, nil
}

// DecrementConcurrent 减少并发计数
// 参数：
//   - key: 限流 key
// 返回：
//   - error: 错误信息
func (r *RedisRateLimiter) DecrementConcurrent(key string) error {
	ctx := context.Background()
	fullKey := r.prefix + "concurrent:" + key

	count, err := r.client.Decr(ctx, fullKey).Result()
	if err != nil {
		return fmt.Errorf("减少并发计数失败: %w", err)
	}

	// 确保不为负数
	if count < 0 {
		r.client.Set(ctx, fullKey, 0, 5*time.Minute)
	}

	return nil
}
