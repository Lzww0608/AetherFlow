package handler

import (
	"net/http"

	"github.com/aetherflow/aetherflow/internal/gateway/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// BreakerStatsHandler 获取熔断器统计信息
func BreakerStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.BreakerManager == nil {
			httpx.WriteJson(w, http.StatusOK, Response{
				Code:    0,
				Message: "Circuit breaker is disabled",
				Data:    nil,
			})
			return
		}

		stats := svcCtx.BreakerManager.GetStats()

		httpx.WriteJson(w, http.StatusOK, Response{
			Code:    0,
			Message: "success",
			Data:    stats,
		})
	}
}

// BreakerResetHandler 重置熔断器
func BreakerResetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.BreakerManager == nil {
			httpx.WriteJson(w, http.StatusOK, Response{
				Code:    -1,
				Message: "Circuit breaker is disabled",
				Data:    nil,
			})
			return
		}

		// 从query参数获取breaker名称
		name := r.URL.Query().Get("name")
		
		if name == "" {
			// 重置所有熔断器
			svcCtx.BreakerManager.ResetAll()
		} else {
			// 重置指定熔断器
			svcCtx.BreakerManager.Reset(name)
		}

		httpx.WriteJson(w, http.StatusOK, Response{
			Code:    0,
			Message: "Circuit breaker reset successfully",
			Data:    nil,
		})
	}
}
