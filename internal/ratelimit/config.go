package ratelimit

import (
	"llmproxy/internal/config"
)

// RateLimitConfig 限流配置（使用 config 包中的定义）
type RateLimitConfig = config.RateLimitConfig
type GlobalLimit = config.GlobalLimit
type KeyLimit = config.KeyLimit
