package middleware

import (
	"net/http"

	"github.com/aetherflow/aetherflow/internal/gateway/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware 创建链路追踪中间件
func TracingMiddleware(tracer *tracing.Tracer) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !tracer.IsEnabled() {
				next(w, r)
				return
			}

			// 从 HTTP 头中提取追踪上下文
			headers := make(map[string][]string)
			for key, values := range r.Header {
				headers[key] = values
			}
			ctx := tracer.ExtractHTTPHeaders(r.Context(), headers)

			// 创建新的 span
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethod(r.Method),
					semconv.HTTPRoute(r.URL.Path),
					semconv.HTTPURL(r.URL.String()),
					semconv.HTTPScheme(r.URL.Scheme),
					semconv.HTTPTarget(r.URL.RequestURI()),
					semconv.NetHostName(r.Host),
					attribute.String("net.host.port", r.URL.Port()),
					attribute.String("http.user_agent", r.UserAgent()),
					attribute.String("http.client_ip", r.RemoteAddr),
				),
			)
			defer span.End()

			// 创建响应记录器
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// 将追踪信息添加到响应头
			traceID := tracer.GetTraceID(ctx)
			spanID := tracer.GetSpanID(ctx)
			if traceID != "" {
				rec.Header().Set("X-Trace-ID", traceID)
			}
			if spanID != "" {
				rec.Header().Set("X-Span-ID", spanID)
			}

			// 执行下一个处理器
			next(rec, r.WithContext(ctx))

			// 记录响应信息
			span.SetAttributes(
				semconv.HTTPStatusCode(rec.statusCode),
				attribute.Int64("http.response_size", rec.bytesWritten),
			)

			// 根据状态码设置 span 状态
			if rec.statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(rec.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		}
	}
}

// responseRecorder 记录响应信息
type responseRecorder struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int64
	headerWritten bool
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if !r.headerWritten {
		r.statusCode = statusCode
		r.ResponseWriter.WriteHeader(statusCode)
		r.headerWritten = true
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.headerWritten {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += int64(n)
	return n, err
}

// Flush 实现 http.Flusher 接口
func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
