package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	pb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"github.com/aetherflow/aetherflow/cmd/statesync-service/config"
	"github.com/aetherflow/aetherflow/internal/gateway/tracing"
	"github.com/aetherflow/aetherflow/internal/statesync"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server StateSync Service gRPC Server
type Server struct {
	pb.UnimplementedStateSyncServiceServer

	config     *config.Config
	manager    *statesync.Manager
	logger     *zap.Logger
	tracer     *tracing.Tracer
	grpcServer *grpc.Server
	httpServer *http.Server
}

// New 创建新的 StateSync Service Server
func New(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	// 创建存储
	var store statesync.Store
	switch cfg.Store.Type {
	case "memory":
		store = statesync.NewMemoryStore()
		logger.Info("Using MemoryStore")
	case "postgres":
		// TODO: 实现 PostgresStore
		logger.Warn("PostgresStore not implemented yet, falling back to MemoryStore")
		store = statesync.NewMemoryStore()
	default:
		return nil, fmt.Errorf("unsupported store type: %s", cfg.Store.Type)
	}

	// 创建 Manager
	manager, err := statesync.NewManager(&statesync.ManagerConfig{
		Store:                store,
		Broadcaster:          nil, // 使用默认
		ConflictResolver:     nil, // 使用默认 LWW
		Logger:               logger,
		LockTimeout:          cfg.Manager.LockTimeout,
		CleanupInterval:      cfg.Manager.CleanupInterval,
		AutoResolveConflicts: cfg.Manager.AutoResolveConflicts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// 创建链路追踪器
	var tracer *tracing.Tracer
	if cfg.Tracing.Enable {
		tracingConfig := &tracing.Config{
			Enable:       cfg.Tracing.Enable,
			ServiceName:  cfg.Tracing.ServiceName,
			Endpoint:     cfg.Tracing.Endpoint,
			Exporter:     cfg.Tracing.Exporter,
			SampleRate:   cfg.Tracing.SampleRate,
			Environment:  cfg.Tracing.Environment,
			BatchTimeout: cfg.Tracing.BatchTimeout,
			MaxQueueSize: cfg.Tracing.MaxQueueSize,
		}
		var err error
		tracer, err = tracing.NewTracer(tracingConfig, logger)
		if err != nil {
			logger.Error("Failed to create tracer", zap.Error(err))
			return nil, fmt.Errorf("failed to create tracer: %w", err)
		}
	}

	s := &Server{
		config:  cfg,
		manager: manager,
		logger:  logger,
		tracer:  tracer,
	}

	return s, nil
}

// Start 启动服务
func (s *Server) Start() error {
	// 创建 gRPC Server
	var opts []grpc.ServerOption

	// 添加链路追踪拦截器
	if s.tracer != nil && s.tracer.IsEnabled() {
		opts = append(opts,
			grpc.ChainUnaryInterceptor(s.unaryTracingInterceptor()),
			grpc.ChainStreamInterceptor(s.streamTracingInterceptor()),
		)
	}

	s.grpcServer = grpc.NewServer(opts...)

	// 注册 StateSyncService
	pb.RegisterStateSyncServiceServer(s.grpcServer, s)

	// 注册健康检查
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s.grpcServer, healthServer)
	healthServer.SetServingStatus("aetherflow.statesync.StateSyncService", grpc_health_v1.HealthCheckResponse_SERVING)

	// 注册反射服务（用于 grpcurl 等工具）
	reflection.Register(s.grpcServer)

	// 启动 Prometheus 指标服务
	if s.config.Metrics.Enable {
		go s.startMetricsServer()
	}

	// 监听端口
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.logger.Info("StateSync Service started",
		zap.String("address", addr),
		zap.Bool("metrics_enabled", s.config.Metrics.Enable),
		zap.Bool("tracing_enabled", s.config.Tracing.Enable))

	// 启动 gRPC Server（阻塞）
	return s.grpcServer.Serve(listener)
}

// Stop 停止服务
func (s *Server) Stop() {
	s.logger.Info("Stopping StateSync Service...")

	// 停止 gRPC Server
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// 停止 HTTP Server（指标）
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5000)
		defer cancel()
		_ = s.httpServer.Shutdown(ctx)
	}

	// 关闭链路追踪器
	if s.tracer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5000)
		defer cancel()
		_ = s.tracer.Shutdown(ctx)
	}

	// 关闭 Manager
	if s.manager != nil {
		s.manager.Close()
	}

	s.logger.Info("StateSync Service stopped")
}

// startMetricsServer 启动指标服务器
func (s *Server) startMetricsServer() {
	addr := fmt.Sprintf("%s:%d", s.config.Metrics.Host, s.config.Metrics.Port)

	mux := http.NewServeMux()
	mux.Handle(s.config.Metrics.Path, promhttp.Handler())

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s.logger.Info("Metrics server started", zap.String("address", addr))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("Metrics server error", zap.Error(err))
	}
}

// unaryTracingInterceptor 一元调用追踪拦截器
func (s *Server) unaryTracingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 创建 span
		ctx, span := s.tracer.Start(ctx, info.FullMethod)
		defer span.End()

		// 执行处理
		resp, err := handler(ctx, req)

		// 记录错误
		if err != nil {
			s.tracer.RecordError(ctx, err)
		}

		return resp, err
	}
}

// streamTracingInterceptor 流式调用追踪拦截器
func (s *Server) streamTracingInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// 创建 span
		ctx, span := s.tracer.Start(ss.Context(), info.FullMethod)
		defer span.End()

		// 包装 ServerStream 以传递新的 context
		wrappedStream := &serverStreamWrapper{
			ServerStream: ss,
			ctx:          ctx,
		}

		// 执行处理
		err := handler(srv, wrappedStream)

		// 记录错误
		if err != nil {
			s.tracer.RecordError(ctx, err)
		}

		return err
	}
}

// serverStreamWrapper 包装 ServerStream 以替换 context
type serverStreamWrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *serverStreamWrapper) Context() context.Context {
	return w.ctx
}
