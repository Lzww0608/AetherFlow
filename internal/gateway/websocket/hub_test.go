package websocket

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func createTestHub() *Hub {
	logger := zap.NewNop()
	handler := NewDefaultHandler(nil, logger)
	return NewHub(logger, handler)
}

func createTestConnection(id string) *Connection {
	logger := zap.NewNop()
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建一个假的websocket连接（仅用于测试）
	return &Connection{
		ID:            id,
		send:          make(chan *Message, 256),
		authenticated: false,
		lastPing:      time.Now(),
		closed:        false,
		subscriptions: make(map[string]bool),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := createTestHub()
	defer hub.Close()
	
	conn := createTestConnection("conn1")
	
	// 注册连接
	hub.Register(conn)
	
	stats := hub.GetStats()
	totalConns := stats["total_connections"].(int)
	if totalConns != 1 {
		t.Errorf("Expected 1 connection, got %d", totalConns)
	}
	
	// 注销连接
	hub.Unregister("conn1")
	
	stats = hub.GetStats()
	totalConns = stats["total_connections"].(int)
	if totalConns != 0 {
		t.Errorf("Expected 0 connections, got %d", totalConns)
	}
}

func TestHub_GetConnection(t *testing.T) {
	hub := createTestHub()
	defer hub.Close()
	
	conn := createTestConnection("conn1")
	hub.Register(conn)
	
	// 获取存在的连接
	retrieved, err := hub.GetConnection("conn1")
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}
	
	if retrieved.ID != "conn1" {
		t.Errorf("Expected conn1, got %s", retrieved.ID)
	}
	
	// 获取不存在的连接
	_, err = hub.GetConnection("nonexistent")
	if err != ErrConnectionNotFound {
		t.Errorf("Expected ErrConnectionNotFound, got %v", err)
	}
}

func TestHub_SetUserID(t *testing.T) {
	hub := createTestHub()
	defer hub.Close()
	
	conn := createTestConnection("conn1")
	hub.Register(conn)
	
	// 设置用户ID
	err := hub.SetUserID("conn1", "user1", "session1")
	if err != nil {
		t.Fatalf("Failed to set user ID: %v", err)
	}
	
	// 验证连接已认证
	retrieved, _ := hub.GetConnection("conn1")
	if !retrieved.IsAuthenticated() {
		t.Error("Connection should be authenticated")
	}
	
	if retrieved.UserID != "user1" {
		t.Errorf("Expected user1, got %s", retrieved.UserID)
	}
	
	// 验证用户连接列表
	stats := hub.GetStats()
	authUsers := stats["authenticated_users"].(int)
	if authUsers != 1 {
		t.Errorf("Expected 1 authenticated user, got %d", authUsers)
	}
}

func TestHub_SubscribeUnsubscribe(t *testing.T) {
	hub := createTestHub()
	defer hub.Close()
	
	conn := createTestConnection("conn1")
	hub.Register(conn)
	
	// 订阅频道
	err := hub.SubscribeChannel("conn1", "channel1")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	
	// 验证订阅
	if !conn.IsSubscribed("channel1") {
		t.Error("Connection should be subscribed to channel1")
	}
	
	stats := hub.GetStats()
	totalChannels := stats["total_channels"].(int)
	if totalChannels != 1 {
		t.Errorf("Expected 1 channel, got %d", totalChannels)
	}
	
	// 取消订阅
	err = hub.UnsubscribeChannel("conn1", "channel1")
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}
	
	// 验证取消订阅
	if conn.IsSubscribed("channel1") {
		t.Error("Connection should not be subscribed to channel1")
	}
	
	stats = hub.GetStats()
	totalChannels = stats["total_channels"].(int)
	if totalChannels != 0 {
		t.Errorf("Expected 0 channels, got %d", totalChannels)
	}
}

func TestHub_BroadcastToChannel(t *testing.T) {
	hub := createTestHub()
	defer hub.Close()
	
	// 创建两个连接
	conn1 := createTestConnection("conn1")
	conn2 := createTestConnection("conn2")
	
	hub.Register(conn1)
	hub.Register(conn2)
	
	// 设置认证状态
	hub.SetUserID("conn1", "user1", "session1")
	hub.SetUserID("conn2", "user2", "session2")
	
	// 订阅频道
	hub.SubscribeChannel("conn1", "channel1")
	hub.SubscribeChannel("conn2", "channel1")
	
	// 广播消息
	msg := NewMessage(MessageTypeNotify, map[string]string{"test": "data"})
	count := hub.BroadcastToChannel("channel1", msg)
	
	if count != 2 {
		t.Errorf("Expected 2 recipients, got %d", count)
	}
	
	// 验证消息已发送到两个连接
	select {
	case received := <-conn1.send:
		if received.Type != MessageTypeNotify {
			t.Errorf("Expected notify message, got %s", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message on conn1")
	}
	
	select {
	case received := <-conn2.send:
		if received.Type != MessageTypeNotify {
			t.Errorf("Expected notify message, got %s", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message on conn2")
	}
}

func TestHub_SendToUser(t *testing.T) {
	hub := createTestHub()
	defer hub.Close()
	
	// 创建两个连接，属于同一用户
	conn1 := createTestConnection("conn1")
	conn2 := createTestConnection("conn2")
	
	hub.Register(conn1)
	hub.Register(conn2)
	
	// 设置相同的用户ID
	hub.SetUserID("conn1", "user1", "session1")
	hub.SetUserID("conn2", "user1", "session2")
	
	// 发送消息给用户
	msg := NewMessage(MessageTypeNotify, map[string]string{"test": "data"})
	count := hub.SendToUser("user1", msg)
	
	if count != 2 {
		t.Errorf("Expected 2 recipients, got %d", count)
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := createTestHub()
	defer hub.Close()
	
	// 创建多个连接
	conn1 := createTestConnection("conn1")
	conn2 := createTestConnection("conn2")
	conn3 := createTestConnection("conn3") // 未认证
	
	hub.Register(conn1)
	hub.Register(conn2)
	hub.Register(conn3)
	
	// 只认证前两个
	hub.SetUserID("conn1", "user1", "session1")
	hub.SetUserID("conn2", "user2", "session2")
	
	// 广播消息（只发送给已认证的连接）
	msg := NewMessage(MessageTypeNotify, map[string]string{"test": "broadcast"})
	count := hub.Broadcast(msg)
	
	if count != 2 {
		t.Errorf("Expected 2 recipients (only authenticated), got %d", count)
	}
}

func TestConnection_Send(t *testing.T) {
	conn := createTestConnection("conn1")
	
	msg := NewMessage(MessageTypePing, nil)
	err := conn.Send(msg)
	
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	// 验证消息在通道中
	select {
	case received := <-conn.send:
		if received.Type != MessageTypePing {
			t.Errorf("Expected ping message, got %s", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message")
	}
}

func TestConnection_IsClosed(t *testing.T) {
	conn := createTestConnection("conn1")
	
	if conn.IsClosed() {
		t.Error("Connection should not be closed initially")
	}
	
	// 关闭连接（会因为没有真实的websocket.Conn而失败，但状态会更新）
	conn.mu.Lock()
	conn.closed = true
	conn.mu.Unlock()
	
	if !conn.IsClosed() {
		t.Error("Connection should be closed")
	}
}

func TestConnection_SubscribeUnsubscribe(t *testing.T) {
	conn := createTestConnection("conn1")
	
	// 订阅
	conn.Subscribe("channel1")
	if !conn.IsSubscribed("channel1") {
		t.Error("Should be subscribed to channel1")
	}
	
	subs := conn.GetSubscriptions()
	if len(subs) != 1 {
		t.Errorf("Expected 1 subscription, got %d", len(subs))
	}
	
	// 取消订阅
	conn.Unsubscribe("channel1")
	if conn.IsSubscribed("channel1") {
		t.Error("Should not be subscribed to channel1")
	}
	
	subs = conn.GetSubscriptions()
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", len(subs))
	}
}

func TestConnection_Ping(t *testing.T) {
	conn := createTestConnection("conn1")
	
	firstPing := conn.LastPing()
	time.Sleep(10 * time.Millisecond)
	
	conn.UpdatePing()
	secondPing := conn.LastPing()
	
	if !secondPing.After(firstPing) {
		t.Error("Second ping should be after first ping")
	}
}
