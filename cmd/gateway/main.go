package main

import (
	"flag"
	"fmt"

	"github.com/aetherflow/aetherflow/internal/gateway/config"
	"github.com/aetherflow/aetherflow/internal/gateway/handler"
	"github.com/aetherflow/aetherflow/internal/gateway/middleware"
	"github.com/aetherflow/aetherflow/internal/gateway/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "configs/gateway.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 初始化日志
	logx.MustSetup(logx.LogConf{
		ServiceName:         c.Log.ServiceName,
		Mode:                c.Log.Mode,
		Path:                c.Log.Path,
		Level:               c.Log.Level,
		Compress:            c.Log.Compress,
		KeepDays:            c.Log.KeepDays,
		StackCooldownMillis: c.Log.StackCooldownMillis,
	})

	// 创建REST服务器
	server := rest.MustNewServer(c.RestConf, rest.WithCors())
	defer server.Stop()

	// 创建服务上下文
	ctx := svc.NewServiceContext(c)

	// 设置WebSocket JWT认证
	ctx.WSServer.SetAuthFunc(func(token string) (userID, sessionID, username, email string, err error) {
		claims, err := ctx.JWTManager.VerifyToken(token)
		if err != nil {
			return "", "", "", "", err
		}
		return claims.UserID, claims.SessionID, claims.Username, claims.Email, nil
	})

	// 注册全局中间件
	server.Use(middleware.RequestIDMiddleware)
	server.Use(middleware.LoggerMiddleware(ctx))

	// 可选：限流中间件
	if c.RateLimit.Enable {
		server.Use(middleware.RateLimitMiddleware(c.RateLimit.Rate, c.RateLimit.Burst))
	}

	// 注册路由
	handler.RegisterHandlers(server, ctx)

	// 启动服务
	fmt.Printf("Starting API Gateway at %s:%d...\n", c.Host, c.Port)
	logx.Infof("API Gateway started at %s:%d", c.Host, c.Port)

	server.Start()
}
