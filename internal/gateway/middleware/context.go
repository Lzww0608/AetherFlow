package middleware

import "context"

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	sessionIDKey contextKey = "session_id"
	userIDKey    contextKey = "user_id"
)

// requestIDToContext 将请求ID添加到context
func requestIDToContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext 从context获取请求ID
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// SessionIDToContext 将会话ID添加到context
func SessionIDToContext(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// SessionIDFromContext 从context获取会话ID
func SessionIDFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value(sessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}

// UserIDToContext 将用户ID添加到context
func UserIDToContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext 从context获取用户ID
func UserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}
