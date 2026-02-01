package svc

import (
	"github.com/aetherflow/aetherflow/internal/gateway/config"
	"github.com/aetherflow/aetherflow/internal/gateway/jwt"
	"github.com/aetherflow/aetherflow/internal/gateway/websocket"
	"go.uber.org/zap"
)

// ServiceContext 服务上下文
type ServiceContext struct {
	Config     config.Config
	Logger     *zap.Logger
	WSServer   *websocket.Server
	JWTManager *jwt.JWTManager
	
	// 将来添加: gRPC客户端连接池
	// SessionClient  session.SessionServiceClient
	// StateSyncClient statesync.StateSyncServiceClient
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

	return &ServiceContext{
		Config:     c,
		Logger:     logger,
		WSServer:   wsServer,
		JWTManager: jwtManager,
	}
}

// Close 关闭服务上下文
func (ctx *ServiceContext) Close() {
	if ctx.WSServer != nil {
		ctx.WSServer.Close()
	}
	if ctx.Logger != nil {
		_ = ctx.Logger.Sync()
	}
}
