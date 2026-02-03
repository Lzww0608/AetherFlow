package svc

import (
	"time"

	"github.com/aetherflow/aetherflow/internal/gateway/config"
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
	}
}

// Close 关闭服务上下文
func (ctx *ServiceContext) Close() {
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
