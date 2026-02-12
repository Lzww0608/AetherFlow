package session

import (
	"context"
	"testing"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// 创建测试用的 Redis 客户端
func newTestRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       15, // 使用 DB 15 作为测试数据库
	})
}

// 检查 Redis 是否可用
func isRedisAvailable() bool {
	client := newTestRedisClient()
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err := client.Ping(ctx).Err()
	return err == nil
}

func TestRedisStore_CreateAndGet(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	// 创建测试会话
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "test-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		ClientPort:   12345,
		ServerAddr:   "localhost:8080",
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{},
	}

	// 测试创建
	err = store.Create(ctx, session)
	assert.NoError(t, err)

	// 测试获取
	retrieved, err := store.Get(ctx, sessionID)
	assert.NoError(t, err)
	assert.Equal(t, session.SessionID, retrieved.SessionID)
	assert.Equal(t, session.UserID, retrieved.UserID)
	assert.Equal(t, session.ClientIP, retrieved.ClientIP)
}

func TestRedisStore_Update(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	// 创建会话
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "test-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{PacketsSent: 10},
	}

	err = store.Create(ctx, session)
	require.NoError(t, err)

	// 更新会话
	session.State = StateIdle
	session.Stats.PacketsSent = 20
	err = store.Update(ctx, session)
	assert.NoError(t, err)

	// 验证更新
	retrieved, err := store.Get(ctx, sessionID)
	assert.NoError(t, err)
	assert.Equal(t, StateIdle, retrieved.State)
	assert.Equal(t, uint64(20), retrieved.Stats.PacketsSent)
}

func TestRedisStore_Delete(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	// 创建会话
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "test-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{},
	}

	err = store.Create(ctx, session)
	require.NoError(t, err)

	// 删除会话
	err = store.Delete(ctx, sessionID)
	assert.NoError(t, err)

	// 验证删除
	_, err = store.Get(ctx, sessionID)
	assert.Error(t, err)

	// 验证计数
	count, err := store.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRedisStore_GetByConnectionID(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	// 创建会话
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "test-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		Metadata:     make(map[string]string),
		Stats:        &Stats{},
	}

	err = store.Create(ctx, session)
	require.NoError(t, err)

	// 通过连接 ID 获取
	retrieved, err := store.GetByConnectionID(ctx, connID)
	assert.NoError(t, err)
	assert.Equal(t, sessionID, retrieved.SessionID)
}

func TestRedisStore_GetByUserID(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	userID := "test-user"

	// 创建多个会话
	for i := 0; i < 3; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()
		session := &Session{
			SessionID:    sessionID,
			UserID:       userID,
			ConnectionID: connID,
			ClientIP:     "127.0.0.1",
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Metadata:     make(map[string]string),
			Stats:        &Stats{},
		}
		err = store.Create(ctx, session)
		require.NoError(t, err)
	}

	// 通过用户 ID 获取所有会话
	sessions, err := store.GetByUserID(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(sessions))
}

func TestRedisStore_List(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	// 创建测试会话
	for i := 0; i < 5; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()
		session := &Session{
			SessionID:    sessionID,
			UserID:       "test-user",
			ConnectionID: connID,
			ClientIP:     "127.0.0.1",
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Metadata:     make(map[string]string),
			Stats:        &Stats{},
		}
		err = store.Create(ctx, session)
		require.NoError(t, err)
	}

	// 测试列表（无过滤）
	sessions, total, err := store.List(ctx, &SessionFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Equal(t, 5, len(sessions))

	// 测试分页
	sessions, total, err = store.List(ctx, &SessionFilter{
		Limit:  2,
		Offset: 0,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Equal(t, 2, len(sessions))
}

func TestRedisStore_Count(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	// 初始计数应为 0
	count, err := store.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// 创建会话
	for i := 0; i < 3; i++ {
		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()
		session := &Session{
			SessionID:    sessionID,
			UserID:       "test-user",
			ConnectionID: connID,
			ClientIP:     "127.0.0.1",
			State:        StateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			ExpiresAt:    time.Now().Add(30 * time.Minute),
			Metadata:     make(map[string]string),
			Stats:        &Stats{},
		}
		err = store.Create(ctx, session)
		require.NoError(t, err)
	}

	// 验证计数
	count, err = store.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestRedisStore_TTL(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    2 * time.Second, // 短 TTL 用于测试
	})
	require.NoError(t, err)
	defer store.Clear(context.Background())

	ctx := context.Background()

	// 创建会话
	sessionID, _ := guuid.NewV7()
	connID, _ := guuid.NewV7()
	session := &Session{
		SessionID:    sessionID,
		UserID:       "test-user",
		ConnectionID: connID,
		ClientIP:     "127.0.0.1",
		State:        StateActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(2 * time.Second),
		Metadata:     make(map[string]string),
		Stats:        &Stats{},
	}

	err = store.Create(ctx, session)
	require.NoError(t, err)

	// 立即获取应该成功
	_, err = store.Get(ctx, sessionID)
	assert.NoError(t, err)

	// 等待 TTL 过期
	time.Sleep(3 * time.Second)

	// 应该已经过期
	_, err = store.Get(ctx, sessionID)
	assert.Error(t, err)
}

func TestRedisStore_Ping(t *testing.T) {
	if !isRedisAvailable() {
		t.Skip("Redis not available, skipping test")
	}

	client := newTestRedisClient()
	defer client.Close()

	store, err := NewRedisStore(&RedisStoreConfig{
		Client: client,
		Logger: zaptest.NewLogger(t),
		TTL:    10 * time.Minute,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 测试 Ping
	err = store.Ping(ctx)
	assert.NoError(t, err)
}
