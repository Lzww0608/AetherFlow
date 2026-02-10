package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aetherflow/aetherflow/internal/gateway/metrics"
)

// MetricsMiddleware 创建指标中间件
func MetricsMiddleware(m *metrics.Metrics) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 增加活跃请求计数
			m.HTTPActiveRequests.WithLabelValues(r.Method, r.URL.Path).Inc()
			defer m.HTTPActiveRequests.WithLabelValues(r.Method, r.URL.Path).Dec()

			// 记录请求大小
			reqSize := r.ContentLength
			if reqSize < 0 {
				reqSize = 0
			}

			// 创建响应记录器
			rec := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// 执行请求
			next(rec, r)

			// 记录指标
			duration := time.Since(start)
			statusCode := strconv.Itoa(rec.statusCode)
			
			m.RecordHTTPRequest(
				r.Method,
				r.URL.Path,
				statusCode,
				duration,
				reqSize,
				rec.bytesWritten,
			)
		}
	}
}

// metricsResponseWriter 响应记录器
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (r *metricsResponseWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += int64(n)
	return n, err
}
