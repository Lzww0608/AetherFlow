package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Config 链路追踪配置
type Config struct {
	Enable       bool    `json:",default=false"`                          // 是否启用追踪
	ServiceName  string  `json:",default=aetherflow-gateway"`             // 服务名称
	Endpoint     string  `json:",default=http://localhost:14268/api/traces"` // Jaeger endpoint
	Exporter     string  `json:",default=jaeger,options=jaeger|zipkin"`   // 导出器类型
	SampleRate   float64 `json:",default=1.0"`                            // 采样率 (0.0-1.0)
	Environment  string  `json:",default=development"`                     // 环境
	BatchTimeout int     `json:",default=5"`                              // 批量发送超时（秒）
	MaxQueueSize int     `json:",default=2048"`                           // 最大队列大小
}

// Tracer 链路追踪管理器
type Tracer struct {
	config   *Config
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	logger   *zap.Logger
}

// NewTracer 创建链路追踪管理器
func NewTracer(cfg *Config, logger *zap.Logger) (*Tracer, error) {
	if !cfg.Enable {
		logger.Info("Tracing is disabled")
		return &Tracer{
			config: cfg,
			logger: logger,
		}, nil
	}

	// 创建资源
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 创建导出器
	var exporter sdktrace.SpanExporter
	switch cfg.Exporter {
	case "jaeger":
		exporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.Endpoint)))
		if err != nil {
			return nil, fmt.Errorf("failed to create jaeger exporter: %w", err)
		}
		logger.Info("Created Jaeger exporter", zap.String("endpoint", cfg.Endpoint))
	case "zipkin":
		exporter, err = zipkin.New(cfg.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to create zipkin exporter: %w", err)
		}
		logger.Info("Created Zipkin exporter", zap.String("endpoint", cfg.Endpoint))
	default:
		return nil, fmt.Errorf("unsupported exporter: %s", cfg.Exporter)
	}

	// 创建采样器
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// 创建批量处理器
	batcher := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithBatchTimeout(time.Duration(cfg.BatchTimeout)*time.Second),
		sdktrace.WithMaxQueueSize(cfg.MaxQueueSize),
	)

	// 创建 TracerProvider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(batcher),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(provider)

	// 设置全局 Propagator (支持 W3C Trace Context 和 Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := provider.Tracer(cfg.ServiceName)

	logger.Info("Tracing initialized",
		zap.String("service", cfg.ServiceName),
		zap.String("exporter", cfg.Exporter),
		zap.Float64("sample_rate", cfg.SampleRate),
	)

	return &Tracer{
		config:   cfg,
		provider: provider,
		tracer:   tracer,
		logger:   logger,
	}, nil
}

// Shutdown 关闭追踪器
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}

	t.logger.Info("Shutting down tracer")
	return t.provider.Shutdown(ctx)
}

// Start 开始一个新的 span
func (t *Tracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !t.config.Enable || t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, spanName, opts...)
}

// GetTracer 获取 tracer
func (t *Tracer) GetTracer() trace.Tracer {
	return t.tracer
}

// IsEnabled 是否启用追踪
func (t *Tracer) IsEnabled() bool {
	return t.config.Enable
}

// AddEvent 添加事件到当前 span
func (t *Tracer) AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if !t.config.Enable {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes 设置 span 属性
func (t *Tracer) SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	if !t.config.Enable {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError 记录错误到 span
func (t *Tracer) RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	if !t.config.Enable || err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, trace.WithAttributes(attrs...))
}

// GetTraceID 获取当前 trace ID
func (t *Tracer) GetTraceID(ctx context.Context) string {
	if !t.config.Enable {
		return ""
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// GetSpanID 获取当前 span ID
func (t *Tracer) GetSpanID(ctx context.Context) string {
	if !t.config.Enable {
		return ""
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().SpanID().String()
}

// InjectHTTPHeaders 将追踪信息注入 HTTP 头
func (t *Tracer) InjectHTTPHeaders(ctx context.Context, headers map[string]string) {
	if !t.config.Enable {
		return
	}
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, &mapCarrier{headers: headers})
}

// ExtractHTTPHeaders 从 HTTP 头提取追踪信息
func (t *Tracer) ExtractHTTPHeaders(ctx context.Context, headers map[string][]string) context.Context {
	if !t.config.Enable {
		return ctx
	}
	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, &sliceMapCarrier{headers: headers})
}

// mapCarrier 实现 TextMapCarrier 接口（用于注入）
type mapCarrier struct {
	headers map[string]string
}

func (c *mapCarrier) Get(key string) string {
	return c.headers[key]
}

func (c *mapCarrier) Set(key, value string) {
	c.headers[key] = value
}

func (c *mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

// sliceMapCarrier 实现 TextMapCarrier 接口（用于提取）
type sliceMapCarrier struct {
	headers map[string][]string
}

func (c *sliceMapCarrier) Get(key string) string {
	values := c.headers[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c *sliceMapCarrier) Set(key, value string) {
	c.headers[key] = []string{value}
}

func (c *sliceMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}
