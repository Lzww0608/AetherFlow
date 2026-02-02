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

	// 认证路由（无需JWT）
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  "POST",
				Path:    "/auth/login",
				Handler: LoginHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/auth/refresh",
				Handler: RefreshTokenHandler(svcCtx),
			},
		},
		rest.WithPrefix("/api/v1"),
	)

	// 需要JWT认证的路由
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  "GET",
				Path:    "/auth/me",
				Handler: MeHandler(svcCtx),
			},
		},
		rest.WithPrefix("/api/v1"),
		rest.WithJwt(svcCtx.Config.JWT.Secret),
	)

	// Session API（需要JWT认证）
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  "POST",
				Path:    "/session",
				Handler: CreateSessionHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/session",
				Handler: GetSessionHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/sessions",
				Handler: ListSessionsHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/session/heartbeat",
				Handler: SessionHeartbeatHandler(svcCtx),
			},
			{
				Method:  "DELETE",
				Path:    "/session",
				Handler: DeleteSessionHandler(svcCtx),
			},
		},
		rest.WithPrefix("/api/v1"),
		rest.WithJwt(svcCtx.Config.JWT.Secret),
	)

	// StateSync API（需要JWT认证）
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  "POST",
				Path:    "/document",
				Handler: CreateDocumentHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/document",
				Handler: GetDocumentHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/documents",
				Handler: ListDocumentsHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/document/operation",
				Handler: ApplyOperationHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/document/operations",
				Handler: GetOperationHistoryHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/document/lock",
				Handler: AcquireLockHandler(svcCtx),
			},
			{
				Method:  "DELETE",
				Path:    "/document/lock",
				Handler: ReleaseLockHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/stats",
				Handler: GetStatsHandler(svcCtx),
			},
		},
		rest.WithPrefix("/api/v1"),
		rest.WithJwt(svcCtx.Config.JWT.Secret),
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
