/*
@Author: Lzww
@LastEditTime: 2025-11-11 21:14:59
@Description: Session manager
@Language: Go
*/
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"go.uber.org/zap"
)

const (
	// DefaultSessionTimeout is the default session timeout duration
	DefaultSessionTimeout = 30 * time.Minute

	// DefaultCleanupInterval is the default cleanup interval
	DefaultCleanupInterval = 5 * time.Minute

	// TokenLength is the length of session tokens in bytes
	TokenLength = 32
)

// Manager manages user sessions
type Manager struct {
	store  Store
	logger *zap.Logger

	// Configuration
	defaultTimeout  time.Duration
	cleanupInterval time.Duration

	// Cleanup
	stopCleanup chan struct{}
	wg          sync.WaitGroup
}

// ManagerConfig contains configuration for session manager
type ManagerConfig struct {
	Store           Store
	Logger          *zap.Logger
	DefaultTimeout  time.Duration
	CleanupInterval time.Duration
}

// NewManager creates a new session manager
func NewManager(config *ManagerConfig) *Manager {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = DefaultSessionTimeout
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = DefaultCleanupInterval
	}
	if config.Logger == nil {
		// to be redirected in production environment
		config.Logger, _ = zap.NewProduction()
	}

	m := &Manager{
		store:           config.Store,
		logger:          config.Logger,
		defaultTimeout:  config.DefaultTimeout,
		cleanupInterval: config.CleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
	m.wg.Add(1)
	go m.cleanupLoop()

	return m
}

// CreateSession creates a new user session
func (m *Manager) CreateSession(ctx context.Context, userID, clientIP string, clientPort uint32, connID guuid.UUID, metadata map[string]string) (*Session, string, error) {
	// Generate session ID
	sessionID, err := guuid.NewV7()
	if err != nil {
		m.logger.Error("failed to generate session ID",
			zap.Error(err))
		return nil, "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Generate session token
	token, err := m.generateToken()
	if err != nil {
		m.logger.Error("failed to generate token",
			zap.Error(err))
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session
	now := time.Now()
	session := &Session{
		SessionID:    sessionID,
		UserID:       userID,
		ConnectionID: connID,
		ClientIP:     clientIP,
		ClientPort:   clientPort,
		State:        StateConnecting,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now.Add(m.defaultTimeout),
		Metadata:     metadata,
		Stats:        &Stats{},
	}

	// Store session
	if err := m.store.Create(ctx, session); err != nil {
		m.logger.Error("failed to create session",
			zap.String("session_id", sessionID.String()),
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	m.logger.Info("session created",
		zap.String("session_id", sessionID.String()),
		zap.String("user_id", userID),
		zap.String("connection_id", connID.String()),
		zap.String("client_ip", clientIP))

	return session, token, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(ctx context.Context, sessionID guuid.UUID) (*Session, error) {
	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Check if expired
	if session.IsExpired() {
		m.logger.Warn("session expired",
			zap.String("session_id", sessionID.String()),
			zap.Time("expires_at", session.ExpiresAt))
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// GetSessionByConnection retrieves a session by connection ID
func (m *Manager) GetSessionByConnection(ctx context.Context, connID guuid.UUID) (*Session, error) {
	return m.store.GetByConnectionID(ctx, connID)
}

// UpdateSession updates session information
func (m *Manager) UpdateSession(ctx context.Context, sessionID guuid.UUID, state *State, metadata map[string]string, stats *Stats) (*Session, error) {
	// Get current session
	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if state != nil {
		session.State = *state
		m.logger.Info("session state updated",
			zap.String("session_id", sessionID.String()),
			zap.String("new_state", state.String()))
	}

	if metadata != nil {
		if session.Metadata == nil {
			session.Metadata = make(map[string]string)
		}
		for k, v := range metadata {
			session.Metadata[k] = v
		}
	}

	if stats != nil {
		session.Stats = stats
	}

	session.LastActiveAt = time.Now()

	// Save updated session
	if err := m.store.Update(ctx, session); err != nil {
		m.logger.Error("failed to update session",
			zap.String("session_id", sessionID.String()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return session, nil
}

// DeleteSession deletes a session
func (m *Manager) DeleteSession(ctx context.Context, sessionID guuid.UUID, reason string) error {
	// Get session info for logging
	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	// Delete from store
	if err := m.store.Delete(ctx, sessionID); err != nil {
		m.logger.Error("failed to delete session",
			zap.String("session_id", sessionID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to delete session: %w", err)
	}

	m.logger.Info("session deleted",
		zap.String("session_id", sessionID.String()),
		zap.String("user_id", session.UserID),
		zap.String("reason", reason))

	return nil
}

// ListSessions lists sessions based on filter
func (m *Manager) ListSessions(ctx context.Context, filter *SessionFilter) ([]*Session, int, error) {
	return m.store.List(ctx, filter)
}

// Heartbeat updates session activity and extends expiration
func (m *Manager) Heartbeat(ctx context.Context, sessionID guuid.UUID) (time.Duration, error) {
	// Get current session
	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	// Check if expired
	if session.IsExpired() {
		m.logger.Warn("heartbeat for expired session",
			zap.String("session_id", sessionID.String()))
		return 0, fmt.Errorf("session expired")
	}

	// Update activity and extend expiration
	now := time.Now()
	session.LastActiveAt = now
	session.ExpiresAt = now.Add(m.defaultTimeout)

	// Mark as active if not already
	if session.State == StateIdle || session.State == StateConnecting {
		session.State = StateActive
	}

	// Save updated session
	if err := m.store.Update(ctx, session); err != nil {
		m.logger.Error("failed to update session heartbeat",
			zap.String("session_id", sessionID.String()),
			zap.Error(err))
		return 0, fmt.Errorf("failed to update session: %w", err)
	}

	remaining := session.ExpiresAt.Sub(now)

	m.logger.Debug("session heartbeat",
		zap.String("session_id", sessionID.String()),
		zap.Duration("remaining", remaining))

	return remaining, nil
}

// GetStats returns overall session statistics
func (m *Manager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	total, err := m.store.Count(ctx)
	if err != nil {
		return nil, err
	}

	// Count by state
	activeFilter := SessionFilter{State: ptrState(StateActive)}
	activeSessions, _, err := m.store.List(ctx, &activeFilter)
	if err != nil {
		return nil, err
	}

	idleFilter := SessionFilter{State: ptrState(StateIdle)}
	idleSessions, _, err := m.store.List(ctx, &idleFilter)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":  total,
		"active": len(activeSessions),
		"idle":   len(idleSessions),
	}, nil
}

// Close stops the manager and cleanup goroutine
func (m *Manager) Close() error {
	close(m.stopCleanup)
	m.wg.Wait()
	m.logger.Info("session manager stopped")
	return nil
}

// cleanupLoop periodically cleans up expired sessions
func (m *Manager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredSessions()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanupExpiredSessions removes all expired sessions
func (m *Manager) cleanupExpiredSessions() {
	ctx := context.Background()

	count, err := m.store.DeleteExpired(ctx)
	if err != nil {
		m.logger.Error("failed to cleanup expired sessions",
			zap.Error(err))
		return
	}

	if count > 0 {
		m.logger.Info("expired sessions cleaned up",
			zap.Int("count", count))
	}
}

// generateToken generates a random session token
func (m *Manager) generateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// Helper function to create state pointer
func ptrState(s State) *State {
	return &s
}
