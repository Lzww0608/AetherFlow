package middleware

import (
	"net/http"

	guuid "github.com/Lzww0608/GUUID"
)

const (
	// RequestIDHeader 请求ID的Header名称
	RequestIDHeader = "X-Request-ID"
)

// RequestIDMiddleware 请求ID中间件
// 为每个请求生成唯一的UUIDv7作为请求ID
func RequestIDMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 尝试从请求头获取请求ID
		requestID := r.Header.Get(RequestIDHeader)

		// 如果请求头中没有，生成新的UUIDv7
		if requestID == "" {
			uuid, err := guuid.NewV7()
			if err != nil {
				// 生成失败，使用默认值
				requestID = "unknown"
			} else {
				requestID = uuid.String()
			}
		}

		// 将请求ID添加到响应头
		w.Header().Set(RequestIDHeader, requestID)

		// 将请求ID添加到请求的context中 (供后续处理使用)
		r = r.WithContext(requestIDToContext(r.Context(), requestID))

		// 调用下一个处理器
		next(w, r)
	}
}
