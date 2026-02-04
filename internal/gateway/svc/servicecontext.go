package svc

import (
	"fmt"
	"time"

	"github.com/aetherflow/aetherflow/internal/gateway/config"
	"github.com/aetherflow/aetherflow/internal/gateway/discovery"
	"github.com/aetherflow/aetherflow/internal/gateway/grpcclient"
	"github.com/aetherflow/aetherflow/internal/gateway/jwt"
	"github.com/aetherflow/aetherflow/internal/gateway/websocket"
	"go.uber.org/zap"
)

// ServiceContext 服务上下文
type ServiceContext struct {
	Config          config.Config
	Logger          *zap.Logger
	WSServer        *websocket.Server
	JWTManager      *jwt.JWTManager
	
	// gRPC客户端
	GRPCManager     *grpcclient.Manager
	SessionClient   *grpcclient.SessionClient
	StateSyncClient *grpcclient.StateSyncClient
	
	// 服务发现
	EtcdClient      *discovery.EtcdClient
	ServiceResolver *discovery.ServiceResolver
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
	
	// 创建gRPC客户端管理器
	grpcManager := grpcclient.NewManager(logger)
	
	// 创建Quantum Dialer（如果需要）
	quantumDialer := grpcclient.NewQuantumDialer(logger)
	
	// 注册Session服务连接池
	sessionDialOpts := grpcclient.GetDialOptions(c.GRPC.Session.Transport, quantumDialer)
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
	
	// 注册StateSync服务连接池
	stateSyncDialOpts := grpcclient.GetDialOptions(c.GRPC.StateSync.Transport, quantumDialer)
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
	
	// 创建StateSync客户端
	stateSyncClient := grpcclient.NewStateSyncClient(
		grpcManager,
		"statesync",
		time.Duration(c.GRPC.StateSync.Timeout)*time.Millisecond,
		c.GRPC.StateSync.MaxRetries,
		logger,
	)

	return &ServiceContext{
		Config:          c,
		Logger:          logger,
		WSServer:        wsServer,
		JWTManager:      jwtManager,
		GRPCManager:     grpcManager,
		SessionClient:   sessionClient,
		StateSyncClient: stateSyncClient,
		EtcdClient:      etcdClient,
		ServiceResolver: serviceResolver,
	}
}

// Close 关闭服务上下文
func (ctx *ServiceContext) Close() {
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
