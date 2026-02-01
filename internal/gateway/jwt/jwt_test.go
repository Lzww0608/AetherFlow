package jwt

import (
	"testing"
	"time"
)

func createTestManager() *JWTManager {
	return NewJWTManager("test-secret-key", 3600, 86400, "test-issuer")
}

func TestJWTManager_GenerateToken(t *testing.T) {
	manager := createTestManager()

	token, err := manager.GenerateToken("user123", "session456", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}
}

func TestJWTManager_VerifyToken(t *testing.T) {
	manager := createTestManager()

	// 生成token
	token, err := manager.GenerateToken("user123", "session456", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// 验证token
	claims, err := manager.VerifyToken(token)
	if err != nil {
		t.Fatalf("Failed to verify token: %v", err)
	}

	// 检查声明
	if claims.UserID != "user123" {
		t.Errorf("Expected UserID user123, got %s", claims.UserID)
	}

	if claims.SessionID != "session456" {
		t.Errorf("Expected SessionID session456, got %s", claims.SessionID)
	}

	if claims.Username != "testuser" {
		t.Errorf("Expected Username testuser, got %s", claims.Username)
	}

	if claims.Email != "test@example.com" {
		t.Errorf("Expected Email test@example.com, got %s", claims.Email)
	}

	if claims.Issuer != "test-issuer" {
		t.Errorf("Expected Issuer test-issuer, got %s", claims.Issuer)
	}
}

func TestJWTManager_VerifyToken_Invalid(t *testing.T) {
	manager := createTestManager()

	// 测试无效token
	_, err := manager.VerifyToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}

	// 测试空token
	_, err = manager.VerifyToken("")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

func TestJWTManager_VerifyToken_WrongSecret(t *testing.T) {
	manager1 := NewJWTManager("secret1", 3600, 86400, "issuer")
	manager2 := NewJWTManager("secret2", 3600, 86400, "issuer")

	// 用manager1生成token
	token, err := manager1.GenerateToken("user123", "session456", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// 用manager2验证token（密钥不同）
	_, err = manager2.VerifyToken(token)
	if err == nil {
		t.Error("Expected error when verifying with wrong secret")
	}
}

func TestJWTManager_VerifyToken_Expired(t *testing.T) {
	// 创建一个1秒过期的管理器
	manager := NewJWTManager("test-secret", 1, 86400, "test-issuer")

	// 生成token
	token, err := manager.GenerateToken("user123", "session456", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// 等待token过期
	time.Sleep(2 * time.Second)

	// 验证过期的token
	_, err = manager.VerifyToken(token)
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got %v", err)
	}
}

func TestJWTManager_GenerateRefreshToken(t *testing.T) {
	manager := createTestManager()

	refreshToken, err := manager.GenerateRefreshToken("user123", "session456")
	if err != nil {
		t.Fatalf("Failed to generate refresh token: %v", err)
	}

	if refreshToken == "" {
		t.Error("Refresh token should not be empty")
	}

	// 验证刷新token
	claims, err := manager.VerifyToken(refreshToken)
	if err != nil {
		t.Fatalf("Failed to verify refresh token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID user123, got %s", claims.UserID)
	}
}

func TestJWTManager_RefreshToken(t *testing.T) {
	manager := createTestManager()

	// 生成访问token和刷新token
	accessToken, _ := manager.GenerateToken("user123", "session456", "testuser", "test@example.com")
	refreshToken, _ := manager.GenerateRefreshToken("user123", "session456")

	// 等待一小段时间
	time.Sleep(100 * time.Millisecond)

	// 使用刷新token获取新的访问token
	newAccessToken, err := manager.RefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	// 新token应该不同
	if newAccessToken == accessToken {
		t.Error("New access token should be different from old one")
	}

	// 验证新token
	claims, err := manager.VerifyToken(newAccessToken)
	if err != nil {
		t.Fatalf("Failed to verify new token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID user123, got %s", claims.UserID)
	}
}

func TestJWTManager_ParseToken(t *testing.T) {
	// 生成一个1秒过期的token
	shortManager := NewJWTManager("test-secret", 1, 86400, "test-issuer")
	token, _ := shortManager.GenerateToken("user123", "session456", "testuser", "test@example.com")

	// 等待过期
	time.Sleep(2 * time.Second)

	// VerifyToken应该失败
	_, err := shortManager.VerifyToken(token)
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got %v", err)
	}

	// ParseToken应该成功（不验证过期）
	claims, err := shortManager.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken should succeed for expired token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID user123, got %s", claims.UserID)
	}
}

func TestJWTManager_MissingClaims(t *testing.T) {
	manager := createTestManager()

	// 生成token但缺少UserID
	token1, _ := manager.GenerateToken("", "session456", "testuser", "test@example.com")
	_, err := manager.VerifyToken(token1)
	if err != ErrMissingClaims {
		t.Errorf("Expected ErrMissingClaims for missing UserID, got %v", err)
	}

	// 生成token但缺少SessionID
	token2, _ := manager.GenerateToken("user123", "", "testuser", "test@example.com")
	_, err = manager.VerifyToken(token2)
	if err != ErrMissingClaims {
		t.Errorf("Expected ErrMissingClaims for missing SessionID, got %v", err)
	}
}

func TestJWTManager_GetExpire(t *testing.T) {
	manager := createTestManager()

	expire := manager.GetExpire()
	expected := 3600 * time.Second

	if expire != expected {
		t.Errorf("Expected expire %v, got %v", expected, expire)
	}
}

func TestJWTManager_GetRefreshExpire(t *testing.T) {
	manager := createTestManager()

	refreshExpire := manager.GetRefreshExpire()
	expected := 86400 * time.Second

	if refreshExpire != expected {
		t.Errorf("Expected refresh expire %v, got %v", expected, refreshExpire)
	}
}
