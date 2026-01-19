package auth

import (
	"log"
	"net/http"
	"time"

	"llmproxy/internal/utils"
)

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	HeaderNames []string // 自定义认证 Header 名称列表
}

// Middleware 鉴权中间件
// 参数：
//   - keyStore: Key 存储
//   - next: 下一个处理器
// 返回：
//   - http.HandlerFunc: HTTP 处理函数
func Middleware(keyStore KeyStore, next http.HandlerFunc) http.HandlerFunc {
	return MiddlewareWithConfig(keyStore, nil, next)
}

// MiddlewareWithConfig 带配置的鉴权中间件
// 参数：
//   - keyStore: Key 存储
//   - config: 中间件配置（可选）
//   - next: 下一个处理器
// 返回：
//   - http.HandlerFunc: HTTP 处理函数
func MiddlewareWithConfig(keyStore KeyStore, config *MiddlewareConfig, next http.HandlerFunc) http.HandlerFunc {
	// 获取 Header 名称列表
	var headerNames []string
	if config != nil && len(config.HeaderNames) > 0 {
		headerNames = config.HeaderNames
	}
	
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 提取 API Key（支持自定义 Header）
		apiKey := utils.ExtractAPIKeyFromHeaders(r.Header, headerNames)
		if apiKey == "" {
			log.Println("鉴权失败: 缺少 API Key")
			http.Error(w, `{"error":"Missing API Key"}`, http.StatusUnauthorized)
			return
		}
		
		// 2. 验证 Key 是否存在
		key, err := keyStore.Get(apiKey)
		if err != nil {
			log.Printf("鉴权失败: API Key 无效: %v", err)
			http.Error(w, `{"error":"Invalid API Key"}`, http.StatusUnauthorized)
			return
		}
		
		// 3. 检查状态
		if key.Status != "active" {
			log.Printf("鉴权失败: API Key 已禁用: %s", utils.MaskKey(apiKey))
			http.Error(w, `{"error":"API Key is disabled"}`, http.StatusForbidden)
			return
		}
		
		// 4. 检查过期时间
		if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
			log.Printf("鉴权失败: API Key 已过期: %s", utils.MaskKey(apiKey))
			http.Error(w, `{"error":"API Key has expired"}`, http.StatusForbidden)
			return
		}
		
		// 5. 检查 IP 白名单/黑名单
		clientIP := utils.GetClientIP(r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"), r.RemoteAddr)
		if !CheckIPAllowed(clientIP, key.AllowedIPs, key.DeniedIPs) {
			log.Printf("鉴权失败: IP 不允许: %s, key: %s", clientIP, utils.MaskKey(apiKey))
			http.Error(w, `{"error":"IP not allowed"}`, http.StatusForbidden)
			return
		}
		
		// 6. 检查额度
		if !CheckQuota(key) {
			log.Printf("鉴权失败: 额度不足: %s", utils.MaskKey(apiKey))
			http.Error(w, `{"error":"Quota exceeded"}`, http.StatusTooManyRequests)
			return
		}
		
		// 7. 将 Key 信息存入请求上下文（通过 Header 传递）
		r.Header.Set("X-API-Key-UserID", key.UserID)
		r.Header.Set("X-API-Key-Name", key.Name)
		
		// 8. 调用下一个处理器
		next(w, r)
	}
}


