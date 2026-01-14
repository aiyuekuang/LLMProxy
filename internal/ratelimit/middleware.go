package ratelimit

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"llmproxy/internal/utils"
)

// Middleware 限流中间件
// 参数：
//   - limiter: 限流器
//   - config: 限流配置
//   - next: 下一个处理器
// 返回：
//   - http.HandlerFunc: HTTP 处理函数
func Middleware(limiter RateLimiter, config *RateLimitConfig, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 全局限流
		if config.Global != nil && config.Global.Enabled {
			burstSize := config.Global.BurstSize
			if burstSize <= 0 {
				burstSize = config.Global.RequestsPerSecond * 2
			}
			
			allowed, remaining, err := limiter.AllowN(
				"global",
				int64(burstSize),
				int64(config.Global.RequestsPerSecond),
				1,
			)
			
			// 设置响应头
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.Global.RequestsPerSecond))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			
			if err != nil || !allowed {
				w.Header().Set("Retry-After", "1")
				log.Println("全局限流: 请求被拒绝")
				http.Error(w, `{"error":"Global rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
		}
		
		// 2. API Key 级限流
		apiKey := utils.ExtractAPIKey(r.Header.Get("Authorization"), r.Header.Get("X-API-Key"))
		if apiKey != "" && config.PerKey != nil && config.PerKey.Enabled {
			keyLimitKey := fmt.Sprintf("ratelimit:key:%s", apiKey)
			
			// 请求数限流
			burstSize := config.PerKey.BurstSize
			if burstSize <= 0 {
				burstSize = config.PerKey.RequestsPerSecond * 2
			}
			
			allowed, remaining, err := limiter.AllowN(
				keyLimitKey,
				int64(burstSize),
				int64(config.PerKey.RequestsPerSecond),
				1,
			)
			
			// 设置响应头
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.PerKey.RequestsPerSecond))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			
			if err != nil || !allowed {
				w.Header().Set("Retry-After", "1")
				log.Printf("Key 级限流: 请求被拒绝, key: %s", utils.MaskKey(apiKey))
				http.Error(w, `{"error":"Rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			
			// 并发数限流
			if config.PerKey.MaxConcurrent > 0 {
				concurrentKey := fmt.Sprintf("concurrent:key:%s", apiKey)
				current, err := limiter.IncrementConcurrent(concurrentKey)
				if err != nil || current > int64(config.PerKey.MaxConcurrent) {
					limiter.DecrementConcurrent(concurrentKey)
					log.Printf("并发数限流: 请求被拒绝, key: %s, concurrent: %d", utils.MaskKey(apiKey), current)
					http.Error(w, `{"error":"Concurrent limit exceeded"}`, http.StatusTooManyRequests)
					return
				}
				
				// 请求结束后减少并发计数
				defer limiter.DecrementConcurrent(concurrentKey)
			}
		}
		
		// 3. 调用下一个处理器
		next(w, r)
	}
}
