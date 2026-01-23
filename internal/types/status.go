package types

// KeyStatus API Key 状态枚举
type KeyStatus int

const (
	KeyStatusActive        KeyStatus = 0 // 正常状态
	KeyStatusDisabled      KeyStatus = 1 // 已禁用
	KeyStatusQuotaExceeded KeyStatus = 2 // 额度超限
	KeyStatusExpired       KeyStatus = 3 // 已过期
)

// String 返回状态的字符串表示
func (s KeyStatus) String() string {
	switch s {
	case KeyStatusActive:
		return "active"
	case KeyStatusDisabled:
		return "disabled"
	case KeyStatusQuotaExceeded:
		return "quota_exceeded"
	case KeyStatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// StatusName 返回状态的大写名称（用于错误响应）
func (s KeyStatus) StatusName() string {
	switch s {
	case KeyStatusActive:
		return "ACTIVE"
	case KeyStatusDisabled:
		return "DISABLED"
	case KeyStatusQuotaExceeded:
		return "QUOTA_EXCEEDED"
	case KeyStatusExpired:
		return "EXPIRED"
	default:
		return "UNKNOWN"
	}
}
