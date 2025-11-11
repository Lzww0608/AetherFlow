// Package session implements session management service
package session

import (
	"context"
	"testing"
	"time"

	guuid "github.com/Lzww0608/GUUID"
)

func TestMemoryStoreCreate(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()

	session := &Session{
		SessionID:    sessionID,
		UserID:       "user123",
		ConnectionID: connID,
		ClientIP:     "192.168.1.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     map[string]string{"device": "mobile"},
		Stats:        &Stats{},
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify duplicate creation fails
	err = store.Create(ctx, session)
	if err == nil {
		t.Error("expected error when creating duplicate session")
	}
}

func TestMemoryStoreGet(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()

	session := &Session{
		SessionID:    sessionID,
		UserID:       "user123",
		ConnectionID: connID,
		ClientIP:     "192.168.1.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Stats:        &Stats{},
	}

	store.Create(ctx, session)

	retrieved, err := store.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.UserID != "user123" {
		t.Errorf("expected user_id 'user123', got '%s'", retrieved.UserID)
	}

	// Test non-existent session
	nonExistent, _ := guuid.NewV7()
	_, err = store.Get(ctx, nonExistent)
	if err == nil {
		t.Error("expected error when getting non-existent session")
	}
}

func TestMemoryStoreUpdate(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()

	session := &Session{
		SessionID:    sessionID,
		UserID:       "user123",
		ConnectionID: connID,
		ClientIP:     "192.168.1.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Stats:        &Stats{},
	}

	store.Create(ctx, session)

	// Update session
	session.State = StateIdle
	session.Stats.PacketsSent = 100

	err := store.Update(ctx, session)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, _ := store.Get(ctx, sessionID)
	if retrieved.State != StateIdle {
		t.Errorf("expected state IDLE, got %v", retrieved.State)
	}
	if retrieved.Stats.PacketsSent != 100 {
		t.Errorf("expected packets_sent 100, got %d", retrieved.Stats.PacketsSent)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()

	session := &Session{
		SessionID:    sessionID,
		UserID:       "user123",
		ConnectionID: connID,
		ClientIP:     "192.168.1.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Stats:        &Stats{},
	}

	store.Create(ctx, session)

	err := store.Delete(ctx, sessionID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = store.Get(ctx, sessionID)
	if err == nil {
		t.Error("expected error when getting deleted session")
	}

	// Verify indexes are cleaned up
	_, err = store.GetByConnectionID(ctx, connID)
	if err == nil {
		t.Error("expected error when getting by deleted connection ID")
	}
}

func TestMemoryStoreList(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()

		session := &Session{
			SessionID:    sessionID,
			UserID:       "user123",
			ConnectionID: connID,
			ClientIP:     "192.168.1.1",
			ClientPort:   uint32(12345 + i),
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Stats:        &Stats{},
		}
		store.Create(ctx, session)
	}

	// Create session with different state
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "user456",
		ConnectionID: connID,
		ClientIP:     "192.168.1.2",
		ClientPort:   54321,
		State:        StateIdle,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Stats:        &Stats{},
	}
	store.Create(ctx, session)

	// List all
	sessions, total, err := store.List(ctx, &SessionFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 6 {
		t.Errorf("expected 6 sessions, got %d", total)
	}

	// Filter by user
	sessions, total, err = store.List(ctx, &SessionFilter{UserID: "user123"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 sessions for user123, got %d", total)
	}

	// Filter by state
	activeState := StateActive
	sessions, total, err = store.List(ctx, &SessionFilter{State: &activeState})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 active sessions, got %d", total)
	}

	// Test pagination
	sessions, total, err = store.List(ctx, &SessionFilter{
		Limit:  2,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions in page, got %d", len(sessions))
	}
	if total != 6 {
		t.Errorf("expected total 6, got %d", total)
	}
}

func TestMemoryStoreGetByConnectionID(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()

	session := &Session{
		SessionID:    sessionID,
		UserID:       "user123",
		ConnectionID: connID,
		ClientIP:     "192.168.1.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Stats:        &Stats{},
	}

	store.Create(ctx, session)

	retrieved, err := store.GetByConnectionID(ctx, connID)
	if err != nil {
		t.Fatalf("GetByConnectionID failed: %v", err)
	}

	if retrieved.SessionID != sessionID {
		t.Error("session ID mismatch")
	}
}

func TestMemoryStoreGetByUserID(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create multiple sessions for same user
	for i := 0; i < 3; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()

		session := &Session{
			SessionID:    sessionID,
			UserID:       "user123",
			ConnectionID: connID,
			ClientIP:     "192.168.1.1",
			ClientPort:   uint32(12345 + i),
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Stats:        &Stats{},
		}
		store.Create(ctx, session)
	}

	sessions, err := store.GetByUserID(ctx, "user123")
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	// Test non-existent user
	sessions, err = store.GetByUserID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestMemoryStoreDeleteExpired(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	now := time.Now()

	// Create expired session
	sessionID1, _ := guuid.NewV7()
	connID1, _ := guuid.NewV7()
	expiredSession := &Session{
		SessionID:    sessionID1,
		UserID:       "user123",
		ConnectionID: connID1,
		ClientIP:     "192.168.1.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    now.Add(-1 * time.Hour),
		LastActiveAt: now.Add(-1 * time.Hour),
		ExpiresAt:    now.Add(-1 * time.Minute), // Expired
		Stats:        &Stats{},
	}
	store.Create(ctx, expiredSession)

	// Create active session
	sessionID2, _ := guuid.NewV7()
	connID2, _ := guuid.NewV7()
	activeSession := &Session{
		SessionID:    sessionID2,
		UserID:       "user456",
		ConnectionID: connID2,
		ClientIP:     "192.168.1.2",
		ClientPort:   54321,
		State:        StateActive,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now.Add(30 * time.Minute), // Not expired
		Stats:        &Stats{},
	}
	store.Create(ctx, activeSession)

	// Delete expired
	count, err := store.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 expired session, got %d", count)
	}

	// Verify only active session remains
	totalCount, _ := store.Count(ctx)
	if totalCount != 1 {
		t.Errorf("expected 1 remaining session, got %d", totalCount)
	}

	// Verify active session still exists
	_, err = store.Get(ctx, sessionID2)
	if err != nil {
		t.Error("active session should still exist")
	}
}

func TestMemoryStoreCount(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Initially empty
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 sessions, got %d", count)
	}

	// Create sessions
	for i := 0; i < 5; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()

		session := &Session{
			SessionID:    sessionID,
			UserID:       "user123",
			ConnectionID: connID,
			ClientIP:     "192.168.1.1",
			ClientPort:   uint32(12345 + i),
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Stats:        &Stats{},
		}
		store.Create(ctx, session)
	}

	count, err = store.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 sessions, got %d", count)
	}
}
