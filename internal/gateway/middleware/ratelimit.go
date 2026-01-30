package middleware

import (
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimitMiddleware 限流中间件
// 使用令牌桶算法限制请求速率
func RateLimitMiddleware(r int, burst int) func(http.HandlerFunc) http.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(r), burst)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next(w, r)
		}
	}
}
