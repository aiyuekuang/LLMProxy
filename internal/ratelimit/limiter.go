package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter 限流器接口
type RateLimiter interface {
	// Allow 检查是否允许请求
	Allow(key string) (bool, error)

	// AllowN 检查是否允许指定数量的 tokens
	AllowN(key string, maxTokens, rate int64, n int64) (bool, int64, error)

	// Remaining 获取剩余配额
	Remaining(key string) (int64, error)

	// IncrementConcurrent 增加并发计数
	IncrementConcurrent(key string) (int64, error)

	// DecrementConcurrent 减少并发计数
	DecrementConcurrent(key string) error
}

// MemoryRateLimiter 基于内存的限流器（令牌桶算法）
type MemoryRateLimiter struct {
	buckets    map[string]*tokenBucket // key -> 令牌桶
	mu         sync.RWMutex            // 读写锁
	concurrent map[string]int64        // 并发计数
}

// tokenBucket 令牌桶
type tokenBucket struct {
	tokens     float64   // 当前令牌数
	maxTokens  float64   // 最大令牌数
	rate       float64   // 令牌生成速率（每秒）
	lastUpdate time.Time // 上次更新时间
}

// NewMemoryRateLimiter 创建内存限流器
// 返回：
//   - RateLimiter: 限流器实例
func NewMemoryRateLimiter() RateLimiter {
	return &MemoryRateLimiter{
		buckets:    make(map[string]*tokenBucket),
		concurrent: make(map[string]int64),
	}
}

// Allow 检查是否允许请求（消耗 1 个令牌）
// 参数：
//   - key: 限流 key
//
// 返回：
//   - bool: 是否允许
//   - error: 错误信息
func (m *MemoryRateLimiter) Allow(key string) (bool, error) {
	allowed, _, err := m.AllowN(key, 100, 10, 1)
	return allowed, err
}

// AllowN 检查是否允许指定数量的 tokens
// 参数：
//   - key: 限流 key
//   - maxTokens: 最大令牌数（桶容量）
//   - rate: 令牌生成速率（每秒）
//   - n: 请求消耗的令牌数
//
// 返回：
//   - bool: 是否允许
//   - int64: 剩余令牌数
//   - error: 错误信息
func (m *MemoryRateLimiter) AllowN(key string, maxTokens, rate int64, n int64) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 获取或创建令牌桶
	bucket, ok := m.buckets[key]
	if !ok {
		bucket = &tokenBucket{
			tokens:     float64(maxTokens),
			maxTokens:  float64(maxTokens),
			rate:       float64(rate),
			lastUpdate: now,
		}
		m.buckets[key] = bucket
	}

	// 计算新增令牌
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	newTokens := bucket.tokens + elapsed*bucket.rate
	if newTokens > bucket.maxTokens {
		newTokens = bucket.maxTokens
	}

	// 检查是否有足够令牌
	if newTokens >= float64(n) {
		bucket.tokens = newTokens - float64(n)
		bucket.lastUpdate = now
		return true, int64(bucket.tokens), nil
	}

	// 令牌不足
	bucket.tokens = newTokens
	bucket.lastUpdate = now
	return false, int64(bucket.tokens), nil
}

// Remaining 获取剩余配额
// 参数：
//   - key: 限流 key
//
// 返回：
//   - int64: 剩余配额
//   - error: 错误信息
func (m *MemoryRateLimiter) Remaining(key string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, ok := m.buckets[key]
	if !ok {
		return 0, fmt.Errorf("key 不存在")
	}

	return int64(bucket.tokens), nil
}

// IncrementConcurrent 增加并发计数
// 参数：
//   - key: 限流 key
//
// 返回：
//   - int64: 当前并发数
//   - error: 错误信息
func (m *MemoryRateLimiter) IncrementConcurrent(key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.concurrent[key]++
	return m.concurrent[key], nil
}

// DecrementConcurrent 减少并发计数
// 参数：
//   - key: 限流 key
//
// 返回：
//   - error: 错误信息
func (m *MemoryRateLimiter) DecrementConcurrent(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.concurrent[key] > 0 {
		m.concurrent[key]--
	}

	return nil
}
