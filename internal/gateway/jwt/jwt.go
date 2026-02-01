package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrMissingClaims    = errors.New("missing required claims")
)

// Claims JWT声明
type Claims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Username  string `json:"username,omitempty"`
	Email     string `json:"email,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager JWT管理器
type JWTManager struct {
	secret        []byte
	expire        time.Duration
	refreshExpire time.Duration
	issuer        string
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(secret string, expire, refreshExpire int64, issuer string) *JWTManager {
	return &JWTManager{
		secret:        []byte(secret),
		expire:        time.Duration(expire) * time.Second,
		refreshExpire: time.Duration(refreshExpire) * time.Second,
		issuer:        issuer,
	}
}

// GenerateToken 生成访问令牌
func (m *JWTManager) GenerateToken(userID, sessionID, username, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		SessionID: sessionID,
		Username:  username,
		Email:     email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expire)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefreshToken 生成刷新令牌
func (m *JWTManager) GenerateRefreshToken(userID, sessionID string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpire)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// VerifyToken 验证令牌
func (m *JWTManager) VerifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSignature
		}
		return m.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// 验证必需的声明
	if claims.UserID == "" || claims.SessionID == "" {
		return nil, ErrMissingClaims
	}

	return claims, nil
}

// RefreshToken 刷新令牌
func (m *JWTManager) RefreshToken(refreshToken string) (string, error) {
	// 验证刷新令牌
	claims, err := m.VerifyToken(refreshToken)
	if err != nil {
		return "", err
	}

	// 生成新的访问令牌
	return m.GenerateToken(claims.UserID, claims.SessionID, claims.Username, claims.Email)
}

// ParseToken 解析令牌（不验证过期）
func (m *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSignature
		}
		return m.secret, nil
	}, jwt.WithoutClaimsValidation())

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GetExpire 获取过期时间
func (m *JWTManager) GetExpire() time.Duration {
	return m.expire
}

// GetRefreshExpire 获取刷新令牌过期时间
func (m *JWTManager) GetRefreshExpire() time.Duration {
	return m.refreshExpire
}
