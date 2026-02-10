package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 指标收集器
type Metrics struct {
	// HTTP 请求指标
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestSize      *prometheus.HistogramVec
	HTTPResponseSize     *prometheus.HistogramVec
	HTTPActiveRequests   *prometheus.GaugeVec

	// gRPC 请求指标
	GRPCRequestsTotal    *prometheus.CounterVec
	GRPCRequestDuration  *prometheus.HistogramVec
	GRPCStreamMessages   *prometheus.CounterVec
	GRPCActiveStreams    *prometheus.GaugeVec

	// WebSocket 指标
	WSConnectionsTotal   *prometheus.CounterVec
	WSActiveConnections  prometheus.Gauge
	WSMessagesTotal      *prometheus.CounterVec
	WSMessageSize        *prometheus.HistogramVec

	// 业务指标
	SessionsTotal        *prometheus.CounterVec
	ActiveSessions       prometheus.Gauge
	DocumentsTotal       *prometheus.CounterVec
	ActiveDocuments      prometheus.Gauge
	OperationsTotal      *prometheus.CounterVec
	ConflictsTotal       *prometheus.CounterVec

	// 系统指标
	ErrorsTotal          *prometheus.CounterVec
	PanicsTotal          *prometheus.CounterVec
	GoRoutines           prometheus.Gauge

	// 熔断器指标
	CircuitBreakerState  *prometheus.GaugeVec
	CircuitBreakerTrips  *prometheus.CounterVec

	// 缓存指标
	CacheHits            *prometheus.CounterVec
	CacheMisses          *prometheus.CounterVec
	CacheEvictions       *prometheus.CounterVec

	// 链路追踪指标
	TracesTotal          *prometheus.CounterVec
	SpansTotal           *prometheus.CounterVec
}

// NewMetrics 创建指标收集器
func NewMetrics(namespace, subsystem string) *Metrics {
	return &Metrics{
		// HTTP 请求指标
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request latency distributions",
				Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
			},
			[]string{"method", "path"},
		),
		HTTPRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_request_size_bytes",
				Help:      "HTTP request size distributions",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 7), // 100B to ~10MB
			},
			[]string{"method", "path"},
		),
		HTTPResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_response_size_bytes",
				Help:      "HTTP response size distributions",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 7), // 100B to ~10MB
			},
			[]string{"method", "path"},
		),
		HTTPActiveRequests: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "http_active_requests",
				Help:      "Number of active HTTP requests",
			},
			[]string{"method", "path"},
		),

		// gRPC 请求指标
		GRPCRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "grpc_requests_total",
				Help:      "Total number of gRPC requests",
			},
			[]string{"service", "method", "status"},
		),
		GRPCRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "grpc_request_duration_seconds",
				Help:      "gRPC request latency distributions",
				Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
			},
			[]string{"service", "method"},
		),
		GRPCStreamMessages: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "grpc_stream_messages_total",
				Help:      "Total number of gRPC stream messages",
			},
			[]string{"service", "method", "direction"}, // direction: sent/received
		),
		GRPCActiveStreams: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "grpc_active_streams",
				Help:      "Number of active gRPC streams",
			},
			[]string{"service", "method"},
		),

		// WebSocket 指标
		WSConnectionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "websocket_connections_total",
				Help:      "Total number of WebSocket connections",
			},
			[]string{"status"}, // status: connected/disconnected
		),
		WSActiveConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "websocket_active_connections",
				Help:      "Number of active WebSocket connections",
			},
		),
		WSMessagesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "websocket_messages_total",
				Help:      "Total number of WebSocket messages",
			},
			[]string{"type", "direction"}, // direction: sent/received
		),
		WSMessageSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "websocket_message_size_bytes",
				Help:      "WebSocket message size distributions",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 7), // 100B to ~10MB
			},
			[]string{"type"},
		),

		// 业务指标
		SessionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "sessions_total",
				Help:      "Total number of sessions",
			},
			[]string{"action"}, // action: created/deleted
		),
		ActiveSessions: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "active_sessions",
				Help:      "Number of active sessions",
			},
		),
		DocumentsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "documents_total",
				Help:      "Total number of documents",
			},
			[]string{"action"}, // action: created/deleted
		),
		ActiveDocuments: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "active_documents",
				Help:      "Number of active documents",
			},
		),
		OperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "operations_total",
				Help:      "Total number of operations",
			},
			[]string{"type", "status"}, // type: insert/delete/update, status: success/failed
		),
		ConflictsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "conflicts_total",
				Help:      "Total number of conflicts",
			},
			[]string{"resolution"}, // resolution: lww/manual/merge
		),

		// 系统指标
		ErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "errors_total",
				Help:      "Total number of errors",
			},
			[]string{"type", "code"},
		),
		PanicsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "panics_total",
				Help:      "Total number of panics",
			},
			[]string{"location"},
		),
		GoRoutines: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "goroutines",
				Help:      "Number of goroutines",
			},
		),

		// 熔断器指标
		CircuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "circuit_breaker_state",
				Help:      "Circuit breaker state (0=closed, 1=open, 2=half-open)",
			},
			[]string{"name"},
		),
		CircuitBreakerTrips: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "circuit_breaker_trips_total",
				Help:      "Total number of circuit breaker trips",
			},
			[]string{"name"},
		),

		// 缓存指标
		CacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "cache_hits_total",
				Help:      "Total number of cache hits",
			},
			[]string{"cache"},
		),
		CacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "cache_misses_total",
				Help:      "Total number of cache misses",
			},
			[]string{"cache"},
		),
		CacheEvictions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "cache_evictions_total",
				Help:      "Total number of cache evictions",
			},
			[]string{"cache"},
		),

		// 链路追踪指标
		TracesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "traces_total",
				Help:      "Total number of traces",
			},
			[]string{"sampled"},
		),
		SpansTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "spans_total",
				Help:      "Total number of spans",
			},
			[]string{"operation"},
		),
	}
}

// RecordHTTPRequest 记录 HTTP 请求
func (m *Metrics) RecordHTTPRequest(method, path, statusCode string, duration time.Duration, reqSize, respSize int64) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	m.HTTPRequestSize.WithLabelValues(method, path).Observe(float64(reqSize))
	m.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(respSize))
}

// RecordGRPCRequest 记录 gRPC 请求
func (m *Metrics) RecordGRPCRequest(service, method, status string, duration time.Duration) {
	m.GRPCRequestsTotal.WithLabelValues(service, method, status).Inc()
	m.GRPCRequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

// RecordWSConnection 记录 WebSocket 连接
func (m *Metrics) RecordWSConnection(connected bool) {
	if connected {
		m.WSConnectionsTotal.WithLabelValues("connected").Inc()
		m.WSActiveConnections.Inc()
	} else {
		m.WSConnectionsTotal.WithLabelValues("disconnected").Inc()
		m.WSActiveConnections.Dec()
	}
}

// RecordWSMessage 记录 WebSocket 消息
func (m *Metrics) RecordWSMessage(msgType, direction string, size int64) {
	m.WSMessagesTotal.WithLabelValues(msgType, direction).Inc()
	m.WSMessageSize.WithLabelValues(msgType).Observe(float64(size))
}

// RecordSession 记录会话
func (m *Metrics) RecordSession(action string) {
	m.SessionsTotal.WithLabelValues(action).Inc()
	if action == "created" {
		m.ActiveSessions.Inc()
	} else if action == "deleted" {
		m.ActiveSessions.Dec()
	}
}

// RecordDocument 记录文档
func (m *Metrics) RecordDocument(action string) {
	m.DocumentsTotal.WithLabelValues(action).Inc()
	if action == "created" {
		m.ActiveDocuments.Inc()
	} else if action == "deleted" {
		m.ActiveDocuments.Dec()
	}
}

// RecordOperation 记录操作
func (m *Metrics) RecordOperation(opType, status string) {
	m.OperationsTotal.WithLabelValues(opType, status).Inc()
}

// RecordConflict 记录冲突
func (m *Metrics) RecordConflict(resolution string) {
	m.ConflictsTotal.WithLabelValues(resolution).Inc()
}

// RecordError 记录错误
func (m *Metrics) RecordError(errType, code string) {
	m.ErrorsTotal.WithLabelValues(errType, code).Inc()
}

// RecordPanic 记录 panic
func (m *Metrics) RecordPanic(location string) {
	m.PanicsTotal.WithLabelValues(location).Inc()
}

// UpdateCircuitBreakerState 更新熔断器状态
func (m *Metrics) UpdateCircuitBreakerState(name string, state float64) {
	m.CircuitBreakerState.WithLabelValues(name).Set(state)
}

// RecordCircuitBreakerTrip 记录熔断器跳闸
func (m *Metrics) RecordCircuitBreakerTrip(name string) {
	m.CircuitBreakerTrips.WithLabelValues(name).Inc()
}

// RecordCacheAccess 记录缓存访问
func (m *Metrics) RecordCacheAccess(cache string, hit bool) {
	if hit {
		m.CacheHits.WithLabelValues(cache).Inc()
	} else {
		m.CacheMisses.WithLabelValues(cache).Inc()
	}
}

// RecordCacheEviction 记录缓存驱逐
func (m *Metrics) RecordCacheEviction(cache string) {
	m.CacheEvictions.WithLabelValues(cache).Inc()
}

// RecordTrace 记录追踪
func (m *Metrics) RecordTrace(sampled bool) {
	sampledStr := "false"
	if sampled {
		sampledStr = "true"
	}
	m.TracesTotal.WithLabelValues(sampledStr).Inc()
}

// RecordSpan 记录 Span
func (m *Metrics) RecordSpan(operation string) {
	m.SpansTotal.WithLabelValues(operation).Inc()
}
