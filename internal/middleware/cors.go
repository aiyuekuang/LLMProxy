package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"llmproxy/internal/config"
)

// CORSMiddleware 创建 CORS 中间件
// 参数：
//   - cfg: CORS 配置
//   - next: 下一个处理器
//
// 返回：
//   - http.Handler: 带 CORS 支持的处理器
func CORSMiddleware(cfg *config.CORSConfig, next http.Handler) http.Handler {
	if cfg == nil || !cfg.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// 检查 Origin 是否允许
		allowed := false
		for _, o := range cfg.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed && origin != "" {
			// 设置 CORS 响应头
			if len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if len(cfg.ExposeHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
			}
		}

		// 处理预检请求
		if r.Method == http.MethodOptions {
			if allowed {
				// 设置预检响应头
				if len(cfg.AllowedMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				} else {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				}

				if len(cfg.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				} else {
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
				}

				if cfg.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
