package handler

import (
	"encoding/json"
	"net/http"

	"github.com/aetherflow/aetherflow/internal/gateway/middleware"
	"github.com/aetherflow/aetherflow/internal/gateway/svc"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
}

// RefreshRequest 刷新令牌请求
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshResponse 刷新令牌响应
type RefreshResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
}

// LoginHandler 登录处理器
func LoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		// 验证用户名和密码（这里是示例，实际应该查询数据库）
		if req.Username == "" || req.Password == "" {
			BadRequestResponse(w, "Username and password are required", requestID)
			return
		}

		// 模拟用户认证（实际应该查询数据库）
		// 这里简单验证，用户名密码都是"test"
		if req.Username != "test" || req.Password != "test" {
			UnauthorizedResponse(w, "Invalid username or password", requestID)
			return
		}

		// 模拟用户信息
		userID := "user-123"
		sessionID := "session-456"
		email := "test@example.com"

		// 生成访问令牌
		token, err := svcCtx.JWTManager.GenerateToken(userID, sessionID, req.Username, email)
		if err != nil {
			InternalServerErrorResponse(w, "Failed to generate token", requestID)
			return
		}

		// 生成刷新令牌
		refreshToken, err := svcCtx.JWTManager.GenerateRefreshToken(userID, sessionID)
		if err != nil {
			InternalServerErrorResponse(w, "Failed to generate refresh token", requestID)
			return
		}

		// 返回令牌
		SuccessResponse(w, LoginResponse{
			Token:        token,
			RefreshToken: refreshToken,
			ExpiresIn:    int64(svcCtx.JWTManager.GetExpire().Seconds()),
			UserID:       userID,
			Username:     req.Username,
		}, requestID)
	}
}

// RefreshTokenHandler 刷新令牌处理器
func RefreshTokenHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())

		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		if req.RefreshToken == "" {
			BadRequestResponse(w, "Refresh token is required", requestID)
			return
		}

		// 刷新访问令牌
		newToken, err := svcCtx.JWTManager.RefreshToken(req.RefreshToken)
		if err != nil {
			UnauthorizedResponse(w, "Invalid or expired refresh token", requestID)
			return
		}

		// 返回新令牌
		SuccessResponse(w, RefreshResponse{
			Token:     newToken,
			ExpiresIn: int64(svcCtx.JWTManager.GetExpire().Seconds()),
		}, requestID)
	}
}

// MeHandler 获取当前用户信息
func MeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())
		sessionID := middleware.SessionIDFromContext(r.Context())

		if userID == "" {
			UnauthorizedResponse(w, "Not authenticated", requestID)
			return
		}

		// 返回用户信息（实际应该从数据库查询）
		SuccessResponse(w, map[string]interface{}{
			"user_id":    userID,
			"session_id": sessionID,
			"username":   "test",
			"email":      "test@example.com",
		}, requestID)
	}
}
