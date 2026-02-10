package grpcclient

import (
	"github.com/aetherflow/aetherflow/internal/gateway/tracing"
	"google.golang.org/grpc"
)

// GetTracingDialOptions 获取追踪拦截器的 DialOptions
func GetTracingDialOptions(tracer *tracing.Tracer) []grpc.DialOption {
	if tracer == nil || !tracer.IsEnabled() {
		return nil
	}

	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(UnaryClientTracingInterceptor(tracer)),
		grpc.WithChainStreamInterceptor(StreamClientTracingInterceptor(tracer)),
	}
}
