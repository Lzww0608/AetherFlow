package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	pb "github.com/aetherflow/aetherflow/api/proto/session"
	"github.com/aetherflow/aetherflow/cmd/session-service/config"
	"github.com/aetherflow/aetherflow/internal/gateway/tracing"
	"github.com/aetherflow/aetherflow/internal/session"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server Session Service gRPC Server
type Server struct {
	pb.UnimplementedSessionServiceServer
	
	config     *config.Config
	manager    *session.Manager
	logger     *zap.Logger
	tracer     *tracing.Tracer
	grpcServer *grpc.Server
	httpServer *http.Server
}

// New 创建新的 Session Service Server
func New(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	// 创建存储
	var store session.Store
	switch cfg.Store.Type {
	case "memory":
		store = session.NewMemoryStore()
		logger.Info("Using MemoryStore")
	case "redis":
		// 创建 Redis 客户端
		redisClient := redis.NewClient(&redis.Options{
			Addr:         cfg.Store.Redis.Addr,
			Password:     cfg.Store.Redis.Password,
			DB:           cfg.Store.Redis.DB,
			PoolSize:     cfg.Store.Redis.PoolSize,
			MinIdleConns: cfg.Store.Redis.MinIdleConns,
			MaxRetries:   cfg.Store.Redis.MaxRetries,
			DialTimeout:  cfg.Store.Redis.DialTimeout,
			ReadTimeout:  cfg.Store.Redis.ReadTimeout,
			WriteTimeout: cfg.Store.Redis.WriteTimeout,
		})
		
		// 测试连接
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Error("Failed to connect to Redis", zap.Error(err))
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}
		
		// 创建 RedisStore
		var err error
		store, err = session.NewRedisStore(&session.RedisStoreConfig{
			Client: redisClient,
			Logger: logger,
			TTL:    30 * time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create RedisStore: %w", err)
		}
		logger.Info("Using RedisStore", zap.String("addr", cfg.Store.Redis.Addr))
	default:
		return nil, fmt.Errorf("unsupported store type: %s", cfg.Store.Type)
	}

	// 创建 Manager
	manager := session.NewManager(&session.ManagerConfig{
		Store:           store,
		Logger:          logger,
		DefaultTimeout:  30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	})

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
		)
	}

	s.grpcServer = grpc.NewServer(opts...)

	// 注册 SessionService
	pb.RegisterSessionServiceServer(s.grpcServer, s)

	// 注册健康检查
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s.grpcServer, healthServer)
	healthServer.SetServingStatus("session.SessionService", grpc_health_v1.HealthCheckResponse_SERVING)

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

	s.logger.Info("Session Service started",
		zap.String("address", addr),
		zap.Bool("metrics_enabled", s.config.Metrics.Enable),
		zap.Bool("tracing_enabled", s.config.Tracing.Enable))

	// 启动 gRPC Server（阻塞）
	return s.grpcServer.Serve(listener)
}

// Stop 停止服务
func (s *Server) Stop() {
	s.logger.Info("Stopping Session Service...")

	// 停止 gRPC Server
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// 停止 HTTP Server（指标）
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(ctx)
	}

	// 关闭链路追踪器
	if s.tracer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.tracer.Shutdown(ctx)
	}

	// 关闭 Manager
	if s.manager != nil {
		s.manager.Close()
	}

	s.logger.Info("Session Service stopped")
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
