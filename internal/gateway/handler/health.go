package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aetherflow/aetherflow/internal/gateway/svc"
)

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
}

// HealthCheckHandler 健康检查
func HealthCheckHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := HealthResponse{
			Status:    "UP",
			Timestamp: time.Now(),
			Service:   "aetherflow-gateway",
			Version:   "0.3.0-alpha",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// PingHandler Ping处理器
func PingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	}
}

// VersionResponse 版本响应
type VersionResponse struct {
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	BuildTime string    `json:"build_time"`
	GoVersion string    `json:"go_version"`
	Timestamp time.Time `json:"timestamp"`
}

// VersionHandler 版本信息处理器
func VersionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := VersionResponse{
			Service:   "aetherflow-gateway",
			Version:   "0.3.0-alpha",
			BuildTime: "2026-01-15",
			GoVersion: "1.21",
			Timestamp: time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
