package routing

import (
	"fmt"
	"log"
	"net"
	"time"
)

// HTTPError HTTP 错误
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// shouldRetry 判断是否应该重试
// 网络错误、超时、5xx 错误应该重试
// 4xx 客户端错误不应该重试
// 参数：
//   - err: 错误信息
//   - statusCode: HTTP 状态码（如果有）
// 返回：
//   - bool: 是否应该重试
func shouldRetry(err error, statusCode int) bool {
	if err != nil {
		// 网络错误或超时
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		// 其他网络错误
		if _, ok := err.(net.Error); ok {
			return true
		}
		return true
	}
	
	// HTTP 状态码判断
	if statusCode >= 500 {
		// 5xx 服务器错误，应该重试
		return true
	}
	
	if statusCode == 429 {
		// 限流错误，应该重试
		return true
	}
	
	// 4xx 客户端错误，不应该重试
	return false
}

// calculateBackoff 计算退避时间
// 使用指数退避算法
// 参数：
//   - attempt: 当前重试次数（从 1 开始）
//   - config: 重试配置
// 返回：
//   - time.Duration: 等待时间
func calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	// 指数退避：initialWait * multiplier^(attempt-1)
	wait := float64(config.InitialWait)
	for i := 1; i < attempt; i++ {
		wait *= config.Multiplier
	}
	
	duration := time.Duration(wait)
	
	// 限制最大等待时间
	if duration > config.MaxWait {
		duration = config.MaxWait
	}
	
	return duration
}

// retryRequest 执行重试逻辑
// 参数：
//   - config: 重试配置
//   - fn: 请求函数
// 返回：
//   - error: 错误信息
func retryRequest(config *RetryConfig, fn func() (int, error)) error {
	if !config.Enabled {
		_, err := fn()
		return err
	}
	
	var lastErr error
	var lastStatusCode int
	
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 计算退避时间
			wait := calculateBackoff(attempt, config)
			log.Printf("重试 %d/%d，等待 %v", attempt, config.MaxRetries, wait)
			time.Sleep(wait)
		}
		
		// 执行请求
		statusCode, err := fn()
		lastStatusCode = statusCode
		lastErr = err
		
		// 请求成功
		if err == nil && statusCode >= 200 && statusCode < 300 {
			if attempt > 0 {
				log.Printf("重试成功，尝试次数: %d", attempt)
			}
			return nil
		}
		
		// 判断是否应该重试
		if !shouldRetry(err, statusCode) {
			log.Printf("请求失败，不应重试: status=%d, err=%v", statusCode, err)
			return lastErr
		}
		
		// 最后一次尝试失败
		if attempt == config.MaxRetries {
			log.Printf("达到最大重试次数 %d，放弃", config.MaxRetries)
			break
		}
	}
	
	if lastErr != nil {
		return fmt.Errorf("重试失败: %w", lastErr)
	}
	return &HTTPError{
		StatusCode: lastStatusCode,
		Message:    "max retries exceeded",
	}
}
