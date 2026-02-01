package middleware

import (
	"net/http"
	"strings"

	"github.com/aetherflow/aetherflow/internal/gateway/jwt"
)

// JWTMiddleware JWT认证中间件
func JWTMiddleware(jwtManager *jwt.JWTManager) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 从请求头获取Token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// 解析Bearer Token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// 验证Token
			claims, err := jwtManager.VerifyToken(tokenString)
			if err != nil {
				switch err {
				case jwt.ErrExpiredToken:
					http.Error(w, "Token has expired", http.StatusUnauthorized)
				case jwt.ErrInvalidSignature:
					http.Error(w, "Invalid token signature", http.StatusUnauthorized)
				case jwt.ErrMissingClaims:
					http.Error(w, "Missing required claims", http.StatusUnauthorized)
				default:
					http.Error(w, "Invalid token", http.StatusUnauthorized)
				}
				return
			}

			// 将用户信息添加到Context
			ctx := r.Context()
			ctx = UserIDToContext(ctx, claims.UserID)
			ctx = SessionIDToContext(ctx, claims.SessionID)

			// 调用下一个处理器
			next(w, r.WithContext(ctx))
		}
	}
}

// OptionalJWTMiddleware 可选JWT认证中间件（不强制要求认证）
func OptionalJWTMiddleware(jwtManager *jwt.JWTManager) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 没有Token，继续处理
				next(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				// Token格式错误，继续处理（不阻止请求）
				next(w, r)
				return
			}

			tokenString := parts[1]
			claims, err := jwtManager.VerifyToken(tokenString)
			if err != nil {
				// Token无效，继续处理（不阻止请求）
				next(w, r)
				return
			}

			// Token有效，将用户信息添加到Context
			ctx := r.Context()
			ctx = UserIDToContext(ctx, claims.UserID)
			ctx = SessionIDToContext(ctx, claims.SessionID)

			next(w, r.WithContext(ctx))
		}
	}
}

// ExtractTokenFromHeader 从请求头提取Token
func ExtractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}
