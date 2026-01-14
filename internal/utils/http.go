package utils

import (
	"strings"
)

// ExtractAPIKey 从请求中提取 API Key
// 支持两种方式：
// 1. Authorization: Bearer sk-xxx
// 2. Header: X-API-Key: sk-xxx
// 参数：
//   - authHeader: Authorization Header 值
//   - apiKeyHeader: X-API-Key Header 值
// 返回：
//   - string: API Key
func ExtractAPIKey(authHeader, apiKeyHeader string) string {
	// 方式 1: Authorization Header
	if authHeader != "" {
		// Bearer token
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	
	// 方式 2: X-API-Key Header
	if apiKeyHeader != "" {
		return apiKeyHeader
	}
	
	return ""
}

// GetClientIP 获取客户端 IP
// 参数：
//   - xForwardedFor: X-Forwarded-For Header 值
//   - xRealIP: X-Real-IP Header 值
//   - remoteAddr: RemoteAddr 值
// 返回：
//   - string: 客户端 IP
func GetClientIP(xForwardedFor, xRealIP, remoteAddr string) string {
	// 1. 尝试从 X-Forwarded-For 获取（代理场景）
	if xForwardedFor != "" {
		// X-Forwarded-For 可能包含多个 IP，取第一个
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// 2. 尝试从 X-Real-IP 获取
	if xRealIP != "" {
		return xRealIP
	}
	
	// 3. 使用 RemoteAddr
	ip := remoteAddr
	// 去掉端口号
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	
	return ip
}

// MaskKey 脱敏 API Key（只显示前 8 位）
// 参数：
//   - key: API Key
// 返回：
//   - string: 脱敏后的 Key
func MaskKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8] + "..."
}
