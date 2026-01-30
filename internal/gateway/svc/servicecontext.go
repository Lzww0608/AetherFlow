package svc

import (
	"github.com/aetherflow/aetherflow/internal/gateway/config"
	"go.uber.org/zap"
)

// ServiceContext 服务上下文
type ServiceContext struct {
	Config config.Config
	Logger *zap.Logger

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

	return &ServiceContext{
		Config: c,
		Logger: logger,
	}
}

// Close 关闭服务上下文
func (ctx *ServiceContext) Close() {
	if ctx.Logger != nil {
		_ = ctx.Logger.Sync()
	}
}
