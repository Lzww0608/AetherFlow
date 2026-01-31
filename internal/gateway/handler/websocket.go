package handler

import (
	"encoding/json"
	"net/http"

	"github.com/aetherflow/aetherflow/internal/gateway/svc"
)

// WebSocketHandler WebSocket连接处理器
func WebSocketHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return svcCtx.WSServer.HandleWebSocket()
}

// WebSocketStatsHandler WebSocket统计信息处理器
func WebSocketStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := svcCtx.WSServer.GetStats()
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response{
			Code:    0,
			Message: "success",
			Data:    stats,
		})
	}
}
