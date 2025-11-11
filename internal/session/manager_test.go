// Package session implements session management service
package session

import (
	"context"
	"testing"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"go.uber.org/zap"
)

func TestManagerCreateSession(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:          store,
		Logger:         logger,
		DefaultTimeout: 1 * time.Minute,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create connection ID
	connID, err := guuid.NewV7()
	if err != nil {
		t.Fatalf("failed to generate connection ID: %v", err)
	}

	// Create session
	session, token, err := manager.CreateSession(ctx, "user123", "192.168.1.1", 12345, connID, map[string]string{
		"device": "mobile",
	})

	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify session
	if session.UserID != "user123" {
		t.Errorf("expected user_id 'user123', got '%s'", session.UserID)
	}
	if session.ClientIP != "192.168.1.1" {
		t.Errorf("expected client_ip '192.168.1.1', got '%s'", session.ClientIP)
	}
	if session.ClientPort != 12345 {
		t.Errorf("expected client_port 12345, got %d", session.ClientPort)
	}
	if session.ConnectionID != connID {
		t.Errorf("connection ID mismatch")
	}
	if session.State != StateConnecting {
		t.Errorf("expected state CONNECTING, got %v", session.State)
	}
	if len(token) == 0 {
		t.Error("expected non-empty token")
	}
	if session.Metadata["device"] != "mobile" {
		t.Error("metadata not set correctly")
	}
}

func TestManagerGetSession(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:  store,
		Logger: logger,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create session
	connID, _ := guuid.NewV7()
	session, _, err := manager.CreateSession(ctx, "user123", "192.168.1.1", 12345, connID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get session
	retrieved, err := manager.GetSession(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	// Verify
	if retrieved.SessionID != session.SessionID {
		t.Error("session ID mismatch")
	}
	if retrieved.UserID != "user123" {
		t.Errorf("expected user_id 'user123', got '%s'", retrieved.UserID)
	}
}

func TestManagerUpdateSession(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:  store,
		Logger: logger,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create session
	connID, _ := guuid.NewV7()
	session, _, err := manager.CreateSession(ctx, "user123", "192.168.1.1", 12345, connID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Update state
	newState := StateActive
	updated, err := manager.UpdateSession(ctx, session.SessionID, &newState, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	if updated.State != StateActive {
		t.Errorf("expected state ACTIVE, got %v", updated.State)
	}

	// Update metadata
	metadata := map[string]string{"version": "1.0"}
	updated, err = manager.UpdateSession(ctx, session.SessionID, nil, metadata, nil)
	if err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	if updated.Metadata["version"] != "1.0" {
		t.Error("metadata not updated correctly")
	}

	// Update stats
	stats := &Stats{
		PacketsSent:     100,
		PacketsReceived: 95,
		BytesSent:       10240,
		BytesReceived:   9800,
	}
	updated, err = manager.UpdateSession(ctx, session.SessionID, nil, nil, stats)
	if err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	if updated.Stats.PacketsSent != 100 {
		t.Errorf("expected packets_sent 100, got %d", updated.Stats.PacketsSent)
	}
}

func TestManagerDeleteSession(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:  store,
		Logger: logger,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create session
	connID, _ := guuid.NewV7()
	session, _, err := manager.CreateSession(ctx, "user123", "192.168.1.1", 12345, connID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Delete session
	err = manager.DeleteSession(ctx, session.SessionID, "test cleanup")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify deletion
	_, err = manager.GetSession(ctx, session.SessionID)
	if err == nil {
		t.Error("expected error when getting deleted session")
	}
}

func TestManagerHeartbeat(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:          store,
		Logger:         logger,
		DefaultTimeout: 1 * time.Minute,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create session
	connID, _ := guuid.NewV7()
	session, _, err := manager.CreateSession(ctx, "user123", "192.168.1.1", 12345, connID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	originalExpiry := session.ExpiresAt

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Send heartbeat
	remaining, err := manager.Heartbeat(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	// Verify remaining time
	if remaining <= 0 {
		t.Error("expected positive remaining time")
	}

	// Verify expiry was extended
	updated, _ := manager.GetSession(ctx, session.SessionID)
	if !updated.ExpiresAt.After(originalExpiry) {
		t.Error("expiry time should be extended")
	}

	// Verify state transition
	if updated.State != StateActive {
		t.Errorf("expected state ACTIVE after heartbeat, got %v", updated.State)
	}
}

func TestManagerListSessions(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:  store,
		Logger: logger,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		connID, _ := guuid.NewV7()
		_, _, err := manager.CreateSession(ctx, "user123", "192.168.1.1", uint32(12345+i), connID, nil)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	// Create session for different user
	connID, _ := guuid.NewV7()
	_, _, err := manager.CreateSession(ctx, "user456", "192.168.1.2", 54321, connID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// List all sessions
	sessions, total, err := manager.ListSessions(ctx, &SessionFilter{})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if total != 6 {
		t.Errorf("expected 6 total sessions, got %d", total)
	}
	if len(sessions) != 6 {
		t.Errorf("expected 6 sessions, got %d", len(sessions))
	}

	// Filter by user
	sessions, total, err = manager.ListSessions(ctx, &SessionFilter{UserID: "user123"})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if total != 5 {
		t.Errorf("expected 5 sessions for user123, got %d", total)
	}

	// Filter by state
	state := StateConnecting
	sessions, total, err = manager.ListSessions(ctx, &SessionFilter{State: &state})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if total != 6 {
		t.Errorf("expected 6 connecting sessions, got %d", total)
	}

	// Test pagination
	sessions, total, err = manager.ListSessions(ctx, &SessionFilter{
		Limit:  2,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if total != 6 {
		t.Errorf("expected 6 total, got %d", total)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions in page, got %d", len(sessions))
	}
}

func TestManagerExpiredSessionCleanup(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:           store,
		Logger:          logger,
		DefaultTimeout:  100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create session with short timeout
	connID, _ := guuid.NewV7()
	session, _, err := manager.CreateSession(ctx, "user123", "192.168.1.1", 12345, connID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify session exists
	_, err = manager.GetSession(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	// Wait for expiration and cleanup
	time.Sleep(300 * time.Millisecond)

	// Verify session is cleaned up
	count, _ := store.Count(ctx)
	if count != 0 {
		t.Errorf("expected 0 sessions after cleanup, got %d", count)
	}
}

func TestManagerGetSessionByConnection(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:  store,
		Logger: logger,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create session
	connID, _ := guuid.NewV7()
	session, _, err := manager.CreateSession(ctx, "user123", "192.168.1.1", 12345, connID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get by connection ID
	retrieved, err := manager.GetSessionByConnection(ctx, connID)
	if err != nil {
		t.Fatalf("GetSessionByConnection failed: %v", err)
	}

	if retrieved.SessionID != session.SessionID {
		t.Error("session ID mismatch")
	}
	if retrieved.ConnectionID != connID {
		t.Error("connection ID mismatch")
	}
}

func TestManagerGetStats(t *testing.T) {
	store := NewMemoryStore()
	logger, _ := zap.NewDevelopment()

	manager := NewManager(&ManagerConfig{
		Store:  store,
		Logger: logger,
	})
	defer manager.Close()

	ctx := context.Background()

	// Create sessions with different states
	connID1, _ := guuid.NewV7()
	session1, _, _ := manager.CreateSession(ctx, "user1", "192.168.1.1", 12345, connID1, nil)
	activeState := StateActive
	manager.UpdateSession(ctx, session1.SessionID, &activeState, nil, nil)

	connID2, _ := guuid.NewV7()
	session2, _, _ := manager.CreateSession(ctx, "user2", "192.168.1.2", 12346, connID2, nil)
	idleState := StateIdle
	manager.UpdateSession(ctx, session2.SessionID, &idleState, nil, nil)

	// Get stats
	stats, err := manager.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["total"].(int) != 2 {
		t.Errorf("expected 2 total sessions, got %v", stats["total"])
	}
	if stats["active"].(int) != 1 {
		t.Errorf("expected 1 active session, got %v", stats["active"])
	}
	if stats["idle"].(int) != 1 {
		t.Errorf("expected 1 idle session, got %v", stats["idle"])
	}
}
