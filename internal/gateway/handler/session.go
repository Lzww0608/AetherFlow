package handler

import (
	"encoding/json"
	"net/http"

	pb "github.com/aetherflow/aetherflow/api/proto/session"
	"github.com/aetherflow/aetherflow/internal/gateway/middleware"
	"github.com/aetherflow/aetherflow/internal/gateway/svc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateSessionHandler 创建会话
func CreateSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())

		if userID == "" {
			UnauthorizedResponse(w, "Not authenticated", requestID)
			return
		}

		var req struct {
			ClientIP       string            `json:"client_ip"`
			ClientPort     uint32            `json:"client_port"`
			Metadata       map[string]string `json:"metadata"`
			TimeoutSeconds uint32            `json:"timeout_seconds"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.SessionClient.CreateSession(r.Context(), &pb.CreateSessionRequest{
			UserId:         userID,
			ClientIp:       req.ClientIP,
			ClientPort:     req.ClientPort,
			Metadata:       req.Metadata,
			TimeoutSeconds: req.TimeoutSeconds,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to create session: "+err.Error(), requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"session": resp.Session,
			"token":   resp.Token,
		}, requestID)
	}
}

// GetSessionHandler 获取会话
func GetSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		sessionID := r.URL.Query().Get("session_id")

		if sessionID == "" {
			BadRequestResponse(w, "session_id is required", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.SessionClient.GetSession(r.Context(), &pb.GetSessionRequest{
			SessionId: sessionID,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to get session: "+err.Error(), requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, resp.Session, requestID)
	}
}

// ListSessionsHandler 列出会话
func ListSessionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())

		// 调用gRPC服务
		resp, err := svcCtx.SessionClient.ListSessions(r.Context(), &pb.ListSessionsRequest{
			UserId: userID,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to list sessions: "+err.Error(), requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"sessions":  resp.Sessions,
			"total":     resp.Total,
			"page":      resp.Page,
			"page_size": resp.PageSize,
		}, requestID)
	}
}

// SessionHeartbeatHandler 会话心跳
func SessionHeartbeatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())

		var req struct {
			SessionID string `json:"session_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		if req.SessionID == "" {
			BadRequestResponse(w, "session_id is required", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.SessionClient.Heartbeat(r.Context(), &pb.HeartbeatRequest{
			SessionId:       req.SessionID,
			ClientTimestamp: timestamppb.Now(),
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to send heartbeat: "+err.Error(), requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"success":           resp.Success,
			"server_timestamp":  resp.ServerTimestamp,
			"remaining_seconds": resp.RemainingSeconds,
		}, requestID)
	}
}

// DeleteSessionHandler 删除会话
func DeleteSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		sessionID := r.URL.Query().Get("session_id")

		if sessionID == "" {
			BadRequestResponse(w, "session_id is required", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.SessionClient.DeleteSession(r.Context(), &pb.DeleteSessionRequest{
			SessionId: sessionID,
			Reason:    "User logout",
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to delete session: "+err.Error(), requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"success": resp.Success,
		}, requestID)
	}
}
