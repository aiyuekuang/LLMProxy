package pipeline

import (
	"context"
	"log"
	"net/http"
	"time"

	"llmproxy/internal/utils"
)

// Middleware 管道鉴权中间件
// 参数：
//   - executor: 管道执行器
//   - next: 下一个处理器
//
// 返回：
//   - http.HandlerFunc: HTTP 处理函数
func Middleware(executor *Executor, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// 0. 检查是否跳过鉴权
		if executor.ShouldSkip(r.URL.Path) {
			next(w, r)
			return
		}

		// 1. 提取 API Key
		apiKey := utils.ExtractAPIKeyFromHeaders(r.Header, executor.GetHeaderNames())
		if apiKey == "" {
			log.Println("鉴权管道: 缺少 API Key")
			WriteErrorResponse(w, &AuthResult{
				Allow:   false,
				Message: "缺少 API Key",
			}, http.StatusUnauthorized)
			return
		}

		// 2. 构建请求信息
		requestInfo := &RequestInfo{
			Method: r.Method,
			Path:   r.URL.Path,
			IP:     utils.GetClientIP(r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"), r.RemoteAddr),
			Headers: func() map[string]string {
				headers := make(map[string]string)
				for k, v := range r.Header {
					if len(v) > 0 {
						headers[k] = v[0]
					}
				}
				return headers
			}(),
		}

		// 3. 执行鉴权管道
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		result, err := executor.Execute(ctx, apiKey, requestInfo)
		if err != nil {
			log.Printf("鉴权管道: 执行错误: %v", err)
			WriteErrorResponse(w, &AuthResult{
				Allow:   false,
				Message: "鉴权服务异常",
			}, http.StatusInternalServerError)
			return
		}

		// 4. 检查结果
		if !result.Allow {
			log.Printf("鉴权管道: 拒绝访问 - %s (耗时: %v)", result.Message, time.Since(startTime))
			WriteErrorResponse(w, result, http.StatusForbidden)
			return
		}

		log.Printf("鉴权管道: 验证通过 (耗时: %v)", time.Since(startTime))

		// 5. 将元数据存入请求头（供后续处理器使用）
		if result.Metadata != nil {
			if userID, ok := result.Metadata["user_id"].(string); ok {
				r.Header.Set("X-API-Key-UserID", userID)
			}
			if name, ok := result.Metadata["name"].(string); ok {
				r.Header.Set("X-API-Key-Name", name)
			}
		}

		// 6. 调用下一个处理器
		next(w, r)
	}
}
