package auth

import (
	"time"
)

// CheckQuota 检查额度是否充足
// 参数：
//   - key: API Key
// 返回：
//   - bool: 是否充足
func CheckQuota(key *APIKey) bool {
	// 如果总额度为 0，表示不限制
	if key.TotalQuota <= 0 {
		return true
	}
	
	return key.UsedQuota < key.TotalQuota
}

// DeductQuota 扣减额度
// 参数：
//   - key: API Key
//   - tokens: 消耗的 tokens
func DeductQuota(key *APIKey, tokens int64) {
	key.UsedQuota += tokens
	key.UpdatedAt = time.Now()
}

// ResetQuotaIfNeeded 按周期重置额度
// 参数：
//   - key: API Key
// 返回：
//   - bool: 是否重置了
func ResetQuotaIfNeeded(key *APIKey) bool {
	if key.QuotaResetPeriod == "never" || key.QuotaResetPeriod == "" {
		return false
	}
	
	now := time.Now()
	var shouldReset bool
	
	switch key.QuotaResetPeriod {
	case "daily":
		// 检查是否跨天
		shouldReset = now.Sub(key.LastResetAt) >= 24*time.Hour
	case "weekly":
		// 检查是否跨周
		shouldReset = now.Sub(key.LastResetAt) >= 7*24*time.Hour
	case "monthly":
		// 检查是否跨月
		shouldReset = now.Month() != key.LastResetAt.Month() || now.Year() != key.LastResetAt.Year()
	}
	
	if shouldReset {
		key.UsedQuota = 0
		key.LastResetAt = now
		key.UpdatedAt = now
		return true
	}
	
	return false
}

// CheckModelAllowed 检查模型是否允许访问
// 参数：
//   - key: API Key
//   - model: 模型名
// 返回：
//   - bool: 是否允许
func CheckModelAllowed(key *APIKey, model string) bool {
	// 空列表表示允许所有模型
	if len(key.AllowedModels) == 0 {
		return true
	}
	
	for _, m := range key.AllowedModels {
		if m == model {
			return true
		}
		// 支持通配符匹配
		if len(m) > 0 && m[len(m)-1] == '*' {
			prefix := m[:len(m)-1]
			if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
				return true
			}
		}
	}
	
	return false
}
