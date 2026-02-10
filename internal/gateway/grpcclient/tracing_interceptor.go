package grpcclient

import (
	"context"

	"github.com/aetherflow/aetherflow/internal/gateway/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryClientTracingInterceptor 创建一元调用追踪拦截器
func UnaryClientTracingInterceptor(tracer *tracing.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !tracer.IsEnabled() {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		// 创建新的 span
		ctx, span := tracer.Start(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", extractService(method)),
				attribute.String("rpc.method", extractMethod(method)),
				attribute.String("rpc.grpc.target", cc.Target()),
			),
		)
		defer span.End()

		// 将追踪信息注入到 gRPC metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		
		// 注入追踪头
		headers := make(map[string]string)
		tracer.InjectHTTPHeaders(ctx, headers)
		for k, v := range headers {
			md.Set(k, v)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)

		// 执行 RPC 调用
		err := invoker(ctx, method, req, reply, cc, opts...)

		// 记录错误
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			
			// 记录 gRPC 状态码
			if st, ok := status.FromError(err); ok {
				span.SetAttributes(
					attribute.String("rpc.grpc.status_code", st.Code().String()),
					attribute.String("rpc.grpc.message", st.Message()),
				)
			}
		} else {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(attribute.String("rpc.grpc.status_code", "OK"))
		}

		return err
	}
}

// StreamClientTracingInterceptor 创建流式调用追踪拦截器
func StreamClientTracingInterceptor(tracer *tracing.Tracer) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if !tracer.IsEnabled() {
			return streamer(ctx, desc, cc, method, opts...)
		}

		// 创建新的 span
		ctx, span := tracer.Start(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", extractService(method)),
				attribute.String("rpc.method", extractMethod(method)),
				attribute.String("rpc.grpc.target", cc.Target()),
				attribute.Bool("rpc.grpc.stream", true),
			),
		)

		// 将追踪信息注入到 gRPC metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		
		headers := make(map[string]string)
		tracer.InjectHTTPHeaders(ctx, headers)
		for k, v := range headers {
			md.Set(k, v)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)

		// 执行流式调用
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return nil, err
		}

		return &tracingClientStream{
			ClientStream: stream,
			span:         span,
			tracer:       tracer,
		}, nil
	}
}

// tracingClientStream 包装 gRPC ClientStream 以追踪流式调用
type tracingClientStream struct {
	grpc.ClientStream
	span   trace.Span
	tracer *tracing.Tracer
}

func (s *tracingClientStream) SendMsg(m interface{}) error {
	s.tracer.AddEvent(s.Context(), "message.sent")
	err := s.ClientStream.SendMsg(m)
	if err != nil {
		s.span.RecordError(err)
	}
	return err
}

func (s *tracingClientStream) RecvMsg(m interface{}) error {
	s.tracer.AddEvent(s.Context(), "message.received")
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		s.span.RecordError(err)
	}
	return err
}

func (s *tracingClientStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	s.span.End()
	return err
}

// 从方法名中提取服务名
func extractService(fullMethod string) string {
	// fullMethod 格式: /package.Service/Method
	if len(fullMethod) < 2 {
		return ""
	}
	parts := fullMethod[1:] // 去掉开头的 /
	for i, c := range parts {
		if c == '/' {
			return parts[:i]
		}
	}
	return parts
}

// 从方法名中提取方法名
func extractMethod(fullMethod string) string {
	// fullMethod 格式: /package.Service/Method
	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '/' {
			return fullMethod[i+1:]
		}
	}
	return fullMethod
}
