package websocket

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrConnectionClosed  = errors.New("connection closed")
	ErrConnectionNotFound = errors.New("connection not found")
	ErrSendChannelFull   = errors.New("send channel full")
	ErrNotAuthenticated  = errors.New("not authenticated")
)

// 超时配置
const (
	writeWait      = 10 * time.Second    // 写入超时
	pongWait       = 60 * time.Second    // Pong超时
	pingPeriod     = (pongWait * 9) / 10 // Ping间隔 (必须小于pongWait)
	maxMessageSize = 512 * 1024          // 最大消息大小 (512KB)
)

// Hub 连接管理中心
type Hub struct {
	// 连接管理
	connections map[string]*Connection // connID -> Connection
	userConns   map[string][]string    // userID -> []connID
	
	// 频道订阅
	channels map[string]map[string]bool // channel -> set of connID
	
	// 同步
	mu     sync.RWMutex
	logger *zap.Logger
	
	// 消息处理器
	handler MessageHandler
	
	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHub 创建连接中心
func NewHub(logger *zap.Logger, handler MessageHandler) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	
	hub := &Hub{
		connections: make(map[string]*Connection),
		userConns:   make(map[string][]string),
		channels:    make(map[string]map[string]bool),
		logger:      logger,
		handler:     handler,
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// 启动清理任务
	go hub.cleanupTask()
	
	return hub
}

// Register 注册连接
func (h *Hub) Register(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.connections[conn.ID] = conn
	
	h.logger.Info("Connection registered",
		zap.String("conn_id", conn.ID),
		zap.Int("total_connections", len(h.connections)),
	)
}

// Unregister 注销连接
func (h *Hub) Unregister(connID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	conn, exists := h.connections[connID]
	if !exists {
		return
	}
	
	// 从用户连接列表中移除
	if conn.UserID != "" {
		h.removeUserConn(conn.UserID, connID)
	}
	
	// 从所有频道中移除
	for channel := range conn.subscriptions {
		h.removeFromChannel(channel, connID)
	}
	
	// 删除连接
	delete(h.connections, connID)
	
	h.logger.Info("Connection unregistered",
		zap.String("conn_id", connID),
		zap.String("user_id", conn.UserID),
		zap.Int("total_connections", len(h.connections)),
	)
}

// GetConnection 获取连接
func (h *Hub) GetConnection(connID string) (*Connection, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	conn, exists := h.connections[connID]
	if !exists {
		return nil, ErrConnectionNotFound
	}
	
	return conn, nil
}

// Broadcast 广播消息到所有连接
func (h *Hub) Broadcast(msg *Message) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	count := 0
	for _, conn := range h.connections {
		if !conn.authenticated {
			continue
		}
		
		if err := conn.Send(msg); err == nil {
			count++
		}
	}
	
	return count
}

// BroadcastToChannel 广播到频道
func (h *Hub) BroadcastToChannel(channel string, msg *Message) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	connIDs, exists := h.channels[channel]
	if !exists {
		return 0
	}
	
	count := 0
	for connID := range connIDs {
		if conn, exists := h.connections[connID]; exists {
			if err := conn.Send(msg); err == nil {
				count++
			}
		}
	}
	
	return count
}

// SendToUser 发送消息给用户的所有连接
func (h *Hub) SendToUser(userID string, msg *Message) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	connIDs, exists := h.userConns[userID]
	if !exists {
		return 0
	}
	
	count := 0
	for _, connID := range connIDs {
		if conn, exists := h.connections[connID]; exists {
			if err := conn.Send(msg); err == nil {
				count++
			}
		}
	}
	
	return count
}

// SubscribeChannel 订阅频道
func (h *Hub) SubscribeChannel(connID, channel string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	conn, exists := h.connections[connID]
	if !exists {
		return ErrConnectionNotFound
	}
	
	// 添加到连接的订阅列表
	conn.Subscribe(channel)
	
	// 添加到频道的连接列表
	if h.channels[channel] == nil {
		h.channels[channel] = make(map[string]bool)
	}
	h.channels[channel][connID] = true
	
	h.logger.Debug("Subscribed to channel",
		zap.String("conn_id", connID),
		zap.String("channel", channel),
		zap.Int("channel_subscribers", len(h.channels[channel])),
	)
	
	return nil
}

// UnsubscribeChannel 取消订阅频道
func (h *Hub) UnsubscribeChannel(connID, channel string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	conn, exists := h.connections[connID]
	if !exists {
		return ErrConnectionNotFound
	}
	
	// 从连接的订阅列表中移除
	conn.Unsubscribe(channel)
	
	// 从频道的连接列表中移除
	h.removeFromChannel(channel, connID)
	
	return nil
}

// SetUserID 设置连接的用户ID
func (h *Hub) SetUserID(connID, userID, sessionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	conn, exists := h.connections[connID]
	if !exists {
		return ErrConnectionNotFound
	}
	
	// 设置认证状态
	conn.SetAuthenticated(userID, sessionID)
	
	// 添加到用户连接列表
	h.userConns[userID] = append(h.userConns[userID], connID)
	
	h.logger.Info("Connection authenticated",
		zap.String("conn_id", connID),
		zap.String("user_id", userID),
		zap.String("session_id", sessionID),
	)
	
	return nil
}

// GetStats 获取统计信息
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return map[string]interface{}{
		"total_connections":      len(h.connections),
		"authenticated_users":    len(h.userConns),
		"total_channels":         len(h.channels),
	}
}

// Close 关闭Hub
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.cancel()
	
	// 关闭所有连接
	for _, conn := range h.connections {
		conn.Close()
	}
	
	h.logger.Info("Hub closed")
}

// removeUserConn 从用户连接列表中移除
func (h *Hub) removeUserConn(userID, connID string) {
	connList := h.userConns[userID]
	for i, id := range connList {
		if id == connID {
			h.userConns[userID] = append(connList[:i], connList[i+1:]...)
			break
		}
	}
	
	// 如果用户没有连接了，删除记录
	if len(h.userConns[userID]) == 0 {
		delete(h.userConns, userID)
	}
}

// removeFromChannel 从频道中移除连接
func (h *Hub) removeFromChannel(channel, connID string) {
	if channelConns, exists := h.channels[channel]; exists {
		delete(channelConns, connID)
		
		// 如果频道没有订阅者了，删除频道
		if len(channelConns) == 0 {
			delete(h.channels, channel)
		}
	}
}

// cleanupTask 清理过期连接
func (h *Hub) cleanupTask() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			h.cleanupDeadConnections()
		case <-h.ctx.Done():
			return
		}
	}
}

// cleanupDeadConnections 清理死连接
func (h *Hub) cleanupDeadConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	now := time.Now()
	timeout := 2 * pongWait
	deadConns := make([]string, 0)
	
	for connID, conn := range h.connections {
		if conn.IsClosed() || now.Sub(conn.LastPing()) > timeout {
			deadConns = append(deadConns, connID)
		}
	}
	
	// 关闭并移除死连接
	for _, connID := range deadConns {
		if conn, exists := h.connections[connID]; exists {
			conn.Close()
			
			// 从用户连接列表中移除
			if conn.UserID != "" {
				h.removeUserConn(conn.UserID, connID)
			}
			
			// 从所有频道中移除
			for channel := range conn.subscriptions {
				h.removeFromChannel(channel, connID)
			}
			
			delete(h.connections, connID)
		}
	}
	
	if len(deadConns) > 0 {
		h.logger.Info("Cleaned up dead connections",
			zap.Int("count", len(deadConns)),
			zap.Int("remaining", len(h.connections)),
		)
	}
}
