package tracing

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func TestNewTracer(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "disabled tracer",
			config: &Config{
				Enable: false,
			},
			wantErr: false,
		},
		{
			name: "jaeger exporter",
			config: &Config{
				Enable:      true,
				ServiceName: "test-service",
				Endpoint:    "http://localhost:14268/api/traces",
				Exporter:    "jaeger",
				SampleRate:  1.0,
			},
			wantErr: false,
		},
		{
			name: "invalid exporter",
			config: &Config{
				Enable:      true,
				ServiceName: "test-service",
				Exporter:    "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer, err := NewTracer(tt.config, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tracer != nil {
				defer func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = tracer.Shutdown(ctx)
				}()

				if tt.config.Enable && !tracer.IsEnabled() {
					t.Error("Tracer should be enabled")
				}
			}
		})
	}
}

func TestTracerOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 创建一个禁用的 tracer
	config := &Config{
		Enable: false,
	}
	tracer, err := NewTracer(config, logger)
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}

	ctx := context.Background()

	// 测试 Start
	newCtx, span := tracer.Start(ctx, "test-span")
	if newCtx == nil {
		t.Error("Start() returned nil context")
	}
	if span == nil {
		t.Error("Start() returned nil span")
	}
	span.End()

	// 测试 AddEvent
	tracer.AddEvent(ctx, "test-event", attribute.String("key", "value"))

	// 测试 SetAttributes
	tracer.SetAttributes(ctx, attribute.String("attr", "value"))

	// 测试 RecordError
	tracer.RecordError(ctx, nil)

	// 测试 GetTraceID
	traceID := tracer.GetTraceID(ctx)
	if traceID != "" {
		t.Error("GetTraceID() should return empty string for disabled tracer")
	}

	// 测试 GetSpanID
	spanID := tracer.GetSpanID(ctx)
	if spanID != "" {
		t.Error("GetSpanID() should return empty string for disabled tracer")
	}
}

func TestInjectExtractHeaders(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := &Config{
		Enable:      true,
		ServiceName: "test-service",
		Endpoint:    "http://localhost:14268/api/traces",
		Exporter:    "jaeger",
		SampleRate:  1.0,
	}

	tracer, err := NewTracer(config, logger)
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tracer.Shutdown(ctx)
	}()

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-span")
	defer span.End()

	// 测试注入
	headers := make(map[string]string)
	tracer.InjectHTTPHeaders(ctx, headers)

	if len(headers) == 0 {
		t.Error("InjectHTTPHeaders() should inject headers")
	}

	// 测试提取
	headersSlice := make(map[string][]string)
	for k, v := range headers {
		headersSlice[k] = []string{v}
	}
	newCtx := tracer.ExtractHTTPHeaders(context.Background(), headersSlice)

	if newCtx == nil {
		t.Error("ExtractHTTPHeaders() returned nil context")
	}
}

func TestSamplingRates(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name       string
		sampleRate float64
	}{
		{"always sample", 1.0},
		{"never sample", 0.0},
		{"50% sample", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Enable:      true,
				ServiceName: "test-service",
				Endpoint:    "http://localhost:14268/api/traces",
				Exporter:    "jaeger",
				SampleRate:  tt.sampleRate,
			}

			tracer, err := NewTracer(config, logger)
			if err != nil {
				t.Fatalf("Failed to create tracer: %v", err)
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = tracer.Shutdown(ctx)
			}()

			if !tracer.IsEnabled() {
				t.Error("Tracer should be enabled")
			}
		})
	}
}

func TestMapCarrier(t *testing.T) {
	headers := make(map[string]string)
	carrier := &mapCarrier{headers: headers}

	// 测试 Set
	carrier.Set("key1", "value1")
	carrier.Set("key2", "value2")

	// 测试 Get
	if carrier.Get("key1") != "value1" {
		t.Error("Get() returned wrong value")
	}

	// 测试 Keys
	keys := carrier.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() returned %d keys, want 2", len(keys))
	}
}

func TestSliceMapCarrier(t *testing.T) {
	headers := make(map[string][]string)
	carrier := &sliceMapCarrier{headers: headers}

	// 测试 Set
	carrier.Set("key1", "value1")
	carrier.Set("key2", "value2")

	// 测试 Get
	if carrier.Get("key1") != "value1" {
		t.Error("Get() returned wrong value")
	}

	// 测试空值
	if carrier.Get("nonexistent") != "" {
		t.Error("Get() should return empty string for nonexistent key")
	}

	// 测试 Keys
	keys := carrier.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() returned %d keys, want 2", len(keys))
	}
}
