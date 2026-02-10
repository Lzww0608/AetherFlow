package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/aetherflow/aetherflow/internal/gateway/breaker"
	"github.com/aetherflow/aetherflow/internal/gateway/config"
	"github.com/aetherflow/aetherflow/internal/gateway/discovery"
	"github.com/aetherflow/aetherflow/internal/gateway/grpcclient"
	"github.com/aetherflow/aetherflow/internal/gateway/jwt"
	"github.com/aetherflow/aetherflow/internal/gateway/metrics"
	"github.com/aetherflow/aetherflow/internal/gateway/tracing"
	"github.com/aetherflow/aetherflow/internal/gateway/websocket"
	"go.uber.org/zap"
)

// ServiceContext 服务上下文
type ServiceContext struct {
	Config          config.Config
	Logger          *zap.Logger
	WSServer        *websocket.Server
	JWTManager      *jwt.JWTManager
	Tracer          *tracing.Tracer
	Metrics         *metrics.Metrics
	MetricsCollector *metrics.Collector
	
	// gRPC客户端
	GRPCManager     *grpcclient.Manager
	SessionClient   *grpcclient.SessionClient
	StateSyncClient *grpcclient.StateSyncClient
	
	// 服务发现
	EtcdClient      *discovery.EtcdClient
	ServiceResolver *discovery.ServiceResolver
	
	// 熔断器
	BreakerManager  *breaker.Manager
}

// NewServiceContext 创建服务上下文
func NewServiceContext(c config.Config) *ServiceContext {
	// 创建Zap Logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	// 创建WebSocket服务器
	wsServer := websocket.NewServer(logger)
	
	// 创建JWT管理器
	jwtManager := jwt.NewJWTManager(
		c.JWT.Secret,
		c.JWT.Expire,
		c.JWT.RefreshExpire,
		c.JWT.Issuer,
	)
	
	// 创建链路追踪器
	tracingConfig := &tracing.Config{
		Enable:       c.Tracing.Enable,
		ServiceName:  c.Tracing.ServiceName,
		Endpoint:     c.Tracing.Endpoint,
		Exporter:     c.Tracing.Exporter,
		SampleRate:   c.Tracing.SampleRate,
		Environment:  c.Tracing.Environment,
		BatchTimeout: c.Tracing.BatchTimeout,
		MaxQueueSize: c.Tracing.MaxQueueSize,
	}
	tracer, err := tracing.NewTracer(tracingConfig, logger)
	if err != nil {
		logger.Error("Failed to create tracer", zap.Error(err))
		panic(fmt.Sprintf("Tracing initialization failed: %v", err))
	}
	
	// 创建指标收集器
	metricsCollector := metrics.NewMetrics("aetherflow", "gateway")
	collector := metrics.NewCollector(metricsCollector, logger)
	collector.Start()
	logger.Info("Metrics collector started")

	// 初始化Etcd和服务发现（如果启用）
	var etcdClient *discovery.EtcdClient
	var serviceResolver *discovery.ServiceResolver
	
	if c.Etcd.Enable {
		etcdConfig := &discovery.Config{
			Endpoints:   c.Etcd.Endpoints,
			DialTimeout: time.Duration(c.Etcd.DialTimeout) * time.Second,
			Username:    c.Etcd.Username,
			Password:    c.Etcd.Password,
		}
		
		var err error
		etcdClient, err = discovery.NewEtcdClient(etcdConfig, logger)
		if err != nil {
			logger.Error("Failed to create Etcd client", zap.Error(err))
			panic(fmt.Sprintf("Etcd initialization failed: %v", err))
		}
		
		// 注册网关服务
		serviceKey := fmt.Sprintf("/services/%s/%s", c.Etcd.ServiceName, c.Etcd.ServiceAddr)
		err = etcdClient.Register(serviceKey, c.Etcd.ServiceAddr, c.Etcd.ServiceTTL)
		if err != nil {
			logger.Error("Failed to register service", zap.Error(err))
			panic(fmt.Sprintf("Service registration failed: %v", err))
		}
		
		// 创建服务解析器
		serviceResolver = discovery.NewServiceResolver(etcdClient, logger)
		
		logger.Info("Etcd service discovery initialized",
			zap.Strings("endpoints", c.Etcd.Endpoints),
			zap.String("service", c.Etcd.ServiceName),
		)
	}
	
	// 创建熔断器管理器
	var breakerManager *breaker.Manager
	if c.Breaker.Enable {
		breakerManager = breaker.NewManager(logger)
		logger.Info("Circuit breaker enabled",
			zap.Float64("threshold", c.Breaker.Threshold),
			zap.Uint32("min_requests", c.Breaker.MinRequests),
		)
	}
	
	// 创建gRPC客户端管理器
	grpcManager := grpcclient.NewManager(logger)
	
	// 创建Quantum Dialer（如果需要）
	quantumDialer := grpcclient.NewQuantumDialer(logger)
	
	// 注册Session服务连接池（添加追踪拦截器）
	sessionDialOpts := grpcclient.GetDialOptions(c.GRPC.Session.Transport, quantumDialer)
	sessionDialOpts = append(sessionDialOpts, grpcclient.GetTracingDialOptions(tracer)...)
	grpcManager.RegisterPool(
		"session",
		c.GRPC.Session.Target,
		c.GRPC.Pool.MaxIdle,
		c.GRPC.Pool.MaxActive,
		time.Duration(c.GRPC.Pool.IdleTimeout)*time.Second,
		sessionDialOpts...,
	)
	
	// 如果启用服务发现，注册监听器
	if c.GRPC.Session.UseDiscovery && serviceResolver != nil {
		serviceName := c.GRPC.Session.DiscoveryName
		if serviceName == "" {
			serviceName = "session"
		}
		
		// 开始发现服务
		if err := serviceResolver.Discover(serviceName); err != nil {
			logger.Error("Failed to discover session service", zap.Error(err))
		} else {
			// 添加地址更新监听器
			sessionPool := grpcManager.GetPool("session")
			if sessionPool != nil {
				serviceResolver.AddUpdateListener(serviceName, func(svcName string, addresses []string) {
					logger.Info("Session service addresses updated",
						zap.String("service", svcName),
						zap.Strings("addresses", addresses),
					)
					sessionPool.UpdateAddresses(addresses)
				})
			}
		}
	}
	
	// 注册StateSync服务连接池（添加追踪拦截器）
	stateSyncDialOpts := grpcclient.GetDialOptions(c.GRPC.StateSync.Transport, quantumDialer)
	stateSyncDialOpts = append(stateSyncDialOpts, grpcclient.GetTracingDialOptions(tracer)...)
	grpcManager.RegisterPool(
		"statesync",
		c.GRPC.StateSync.Target,
		c.GRPC.Pool.MaxIdle,
		c.GRPC.Pool.MaxActive,
		time.Duration(c.GRPC.Pool.IdleTimeout)*time.Second,
		stateSyncDialOpts...,
	)
	
	// 如果启用服务发现，注册监听器
	if c.GRPC.StateSync.UseDiscovery && serviceResolver != nil {
		serviceName := c.GRPC.StateSync.DiscoveryName
		if serviceName == "" {
			serviceName = "statesync"
		}
		
		// 开始发现服务
		if err := serviceResolver.Discover(serviceName); err != nil {
			logger.Error("Failed to discover statesync service", zap.Error(err))
		} else {
			// 添加地址更新监听器
			stateSyncPool := grpcManager.GetPool("statesync")
			if stateSyncPool != nil {
				serviceResolver.AddUpdateListener(serviceName, func(svcName string, addresses []string) {
					logger.Info("StateSync service addresses updated",
						zap.String("service", svcName),
						zap.Strings("addresses", addresses),
					)
					stateSyncPool.UpdateAddresses(addresses)
				})
			}
		}
	}

	// 创建Session客户端
	sessionClient := grpcclient.NewSessionClient(
		grpcManager,
		"session",
		time.Duration(c.GRPC.Session.Timeout)*time.Millisecond,
		c.GRPC.Session.MaxRetries,
		logger,
	)
	
	// 如果启用熔断，包装Session客户端
	if breakerManager != nil {
		sessionBreakerConfig := breaker.Config{
			MaxRequests: c.Breaker.HalfOpenRequests,
			Interval:    10 * time.Second,
			Timeout:     time.Duration(c.Breaker.Timeout) * time.Second,
			ReadyToTrip: func(counts breaker.Counts) bool {
				return counts.Requests >= c.Breaker.MinRequests && 
					(counts.ErrorRate() >= c.Breaker.Threshold || 
					 counts.ConsecutiveFailures >= c.Breaker.ConsecutiveFailures)
			},
		}
		sessionBreaker := breakerManager.GetOrCreate("session", sessionBreakerConfig)
		_ = sessionBreaker // 熔断器已注册，可以在handler中使用
	}
	
	// 创建StateSync客户端
	stateSyncClient := grpcclient.NewStateSyncClient(
		grpcManager,
		"statesync",
		time.Duration(c.GRPC.StateSync.Timeout)*time.Millisecond,
		c.GRPC.StateSync.MaxRetries,
		logger,
	)
	
	// 如果启用熔断，包装StateSync客户端
	if breakerManager != nil {
		stateSyncBreakerConfig := breaker.Config{
			MaxRequests: c.Breaker.HalfOpenRequests,
			Interval:    10 * time.Second,
			Timeout:     time.Duration(c.Breaker.Timeout) * time.Second,
			ReadyToTrip: func(counts breaker.Counts) bool {
				return counts.Requests >= c.Breaker.MinRequests && 
					(counts.ErrorRate() >= c.Breaker.Threshold || 
					 counts.ConsecutiveFailures >= c.Breaker.ConsecutiveFailures)
			},
		}
		stateSyncBreaker := breakerManager.GetOrCreate("statesync", stateSyncBreakerConfig)
		_ = stateSyncBreaker // 熔断器已注册，可以在handler中使用
	}

	return &ServiceContext{
		Config:           c,
		Logger:           logger,
		WSServer:         wsServer,
		JWTManager:       jwtManager,
		Tracer:           tracer,
		Metrics:          metricsCollector,
		MetricsCollector: collector,
		GRPCManager:      grpcManager,
		SessionClient:    sessionClient,
		StateSyncClient:  stateSyncClient,
		EtcdClient:       etcdClient,
		ServiceResolver:  serviceResolver,
		BreakerManager:   breakerManager,
	}
}

// Close 关闭服务上下文
func (ctx *ServiceContext) Close() {
	// 关闭指标收集器
	if ctx.MetricsCollector != nil {
		ctx.MetricsCollector.Stop()
	}
	
	// 关闭链路追踪器
	if ctx.Tracer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ctx.Tracer.Shutdown(shutdownCtx); err != nil {
			ctx.Logger.Error("Failed to shutdown tracer", zap.Error(err))
		}
	}
	
	// 关闭Etcd客户端（先注销服务）
	if ctx.EtcdClient != nil {
		if err := ctx.EtcdClient.Unregister(); err != nil {
			ctx.Logger.Error("Failed to unregister service", zap.Error(err))
		}
		if err := ctx.EtcdClient.Close(); err != nil {
			ctx.Logger.Error("Failed to close Etcd client", zap.Error(err))
		}
	}
	
	if ctx.WSServer != nil {
		ctx.WSServer.Close()
	}
	
	if ctx.GRPCManager != nil {
		if err := ctx.GRPCManager.Close(); err != nil {
			ctx.Logger.Error("Failed to close gRPC manager", zap.Error(err))
		}
	}
	
	if ctx.Logger != nil {
		_ = ctx.Logger.Sync()
	}
}
