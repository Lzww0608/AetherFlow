package handler

import (
	"github.com/aetherflow/aetherflow/internal/gateway/svc"
	"github.com/zeromicro/go-zero/rest"
)

// RegisterHandlers 注册所有路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	// 健康检查和监控
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  "GET",
				Path:    "/health",
				Handler: HealthCheckHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/ping",
				Handler: PingHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/version",
				Handler: VersionHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/ws",
				Handler: WebSocketHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/ws/stats",
				Handler: WebSocketStatsHandler(svcCtx),
			},
		},
	)

	// Session相关路由 (将来实现)
	server.AddRoutes(
		[]rest.Route{
			// {
			// 	Method:  "POST",
			// 	Path:    "/sessions",
			// 	Handler: CreateSessionHandler(svcCtx),
			// },
			// {
			// 	Method:  "GET",
			// 	Path:    "/sessions/:id",
			// 	Handler: GetSessionHandler(svcCtx),
			// },
		},
		rest.WithPrefix("/api/v1"),
	)

	// StateSync相关路由 (将来实现)
	server.AddRoutes(
		[]rest.Route{
			// {
			// 	Method:  "POST",
			// 	Path:    "/documents",
			// 	Handler: CreateDocumentHandler(svcCtx),
			// },
			// {
			// 	Method:  "GET",
			// 	Path:    "/documents/:id",
			// 	Handler: GetDocumentHandler(svcCtx),
			// },
		},
		rest.WithPrefix("/api/v1"),
	)
}
