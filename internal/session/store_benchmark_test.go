package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/redis/go-redis/v9"
)

// BenchmarkMemoryStore_Create benchmarks MemoryStore Create operation
func BenchmarkMemoryStore_Create(b *testing.B) {
	store := NewMemoryStore()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()
		session := &Session{
			SessionID:    sessionID,
			UserID:       fmt.Sprintf("user-%d", i),
			ConnectionID: connID,
			ClientIP:     "127.0.0.1",
			ClientPort:   uint32(12345 + i),
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Metadata:     make(map[string]string),
			Stats:        &Stats{},
		}
		store.Create(ctx, session)
	}
}

// BenchmarkMemoryStore_Get benchmarks MemoryStore Get operation
func BenchmarkMemoryStore_Get(b *testing.B) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Setup: Create a session
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "benchmark-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{},
	}
	store.Create(ctx, session)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(ctx, sessionID)
	}
}

// BenchmarkMemoryStore_Update benchmarks MemoryStore Update operation
func BenchmarkMemoryStore_Update(b *testing.B) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Setup
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "benchmark-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{PacketsSent: 100},
	}
	store.Create(ctx, session)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.Stats.PacketsSent = uint64(100 + i)
		_ = store.Update(ctx, session)
	}
}

// BenchmarkMemoryStore_GetByUserID benchmarks MemoryStore GetByUserID operation
func BenchmarkMemoryStore_GetByUserID(b *testing.B) {
	store := NewMemoryStore()
	ctx := context.Background()
	userID := "benchmark-user"

	// Setup: Create 10 sessions for the same user
	for i := 0; i < 10; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()
		session := &Session{
			SessionID:    sessionID,
			UserID:       userID,
			ConnectionID: connID,
			ClientIP:     "127.0.0.1",
			ClientPort:   uint32(12345 + i),
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Metadata:     make(map[string]string),
			Stats:        &Stats{},
		}
		store.Create(ctx, session)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetByUserID(ctx, userID)
	}
}

// BenchmarkRedisStore_Create benchmarks RedisStore Create operation
func BenchmarkRedisStore_Create(b *testing.B) {
	if !isRedisAvailable() {
		b.Skip("Redis not available")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		TTL:    10 * time.Minute,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Clear(context.Background())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()
		session := &Session{
			SessionID:    sessionID,
			UserID:       fmt.Sprintf("user-%d", i),
			ConnectionID: connID,
			ClientIP:     "127.0.0.1",
			ClientPort:   uint32(12345 + i),
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Metadata:     make(map[string]string),
			Stats:        &Stats{},
		}
		store.Create(ctx, session)
	}
}

// BenchmarkRedisStore_Get benchmarks RedisStore Get operation
func BenchmarkRedisStore_Get(b *testing.B) {
	if !isRedisAvailable() {
		b.Skip("Redis not available")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		TTL:    10 * time.Minute,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Clear(context.Background())

	ctx := context.Background()

	// Setup
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "benchmark-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{},
	}
	store.Create(ctx, session)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(ctx, sessionID)
	}
}

// BenchmarkRedisStore_Update benchmarks RedisStore Update operation
func BenchmarkRedisStore_Update(b *testing.B) {
	if !isRedisAvailable() {
		b.Skip("Redis not available")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		TTL:    10 * time.Minute,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Clear(context.Background())

	ctx := context.Background()

	// Setup
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "benchmark-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		ClientPort:   12345,
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{PacketsSent: 100},
	}
	store.Create(ctx, session)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.Stats.PacketsSent = uint64(100 + i)
		_ = store.Update(ctx, session)
	}
}

// BenchmarkRedisStore_GetByUserID benchmarks RedisStore GetByUserID operation
func BenchmarkRedisStore_GetByUserID(b *testing.B) {
	if !isRedisAvailable() {
		b.Skip("Redis not available")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		TTL:    10 * time.Minute,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Clear(context.Background())

	ctx := context.Background()
	userID := "benchmark-user"

	// Setup: Create 10 sessions
	for i := 0; i < 10; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()
		session := &Session{
			SessionID:    sessionID,
			UserID:       userID,
			ConnectionID: connID,
			ClientIP:     "127.0.0.1",
			ClientPort:   uint32(12345 + i),
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Metadata:     make(map[string]string),
			Stats:        &Stats{},
		}
		store.Create(ctx, session)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetByUserID(ctx, userID)
	}
}

// BenchmarkComparison runs all benchmarks and compares results
func BenchmarkComparison(b *testing.B) {
	b.Run("MemoryStore", func(b *testing.B) {
		b.Run("Create", BenchmarkMemoryStore_Create)
		b.Run("Get", BenchmarkMemoryStore_Get)
		b.Run("Update", BenchmarkMemoryStore_Update)
		b.Run("GetByUserID", BenchmarkMemoryStore_GetByUserID)
	})

	b.Run("RedisStore", func(b *testing.B) {
		b.Run("Create", BenchmarkRedisStore_Create)
		b.Run("Get", BenchmarkRedisStore_Get)
		b.Run("Update", BenchmarkRedisStore_Update)
		b.Run("GetByUserID", BenchmarkRedisStore_GetByUserID)
	})
}
