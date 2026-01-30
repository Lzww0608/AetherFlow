package middleware

import (
	"net/http"
	"time"

	"github.com/aetherflow/aetherflow/internal/gateway/svc"
	"go.uber.org/zap"
)

// responseWriter 包装http.ResponseWriter以记录状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// LoggerMiddleware 日志中间件
// 记录每个HTTP请求的详细信息
func LoggerMiddleware(ctx *svc.ServiceContext) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// 包装ResponseWriter以捕获状态码
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     200, // 默认状态码
			}

			// 获取请求ID
			requestID := RequestIDFromContext(r.Context())

			// 记录请求开始
			ctx.Logger.Info("HTTP Request",
				zap.String("request_id", requestID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			)

			// 调用下一个处理器
			next(wrapped, r)

			// 计算请求处理时间
			duration := time.Since(startTime)

			// 记录请求完成
			ctx.Logger.Info("HTTP Response",
				zap.String("request_id", requestID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.statusCode),
				zap.Int("size", wrapped.size),
				zap.Duration("duration", duration),
			)
		}
	}
}
