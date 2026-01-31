package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Connection WebSocket连接封装
type Connection struct {
	// 基础信息
	ID        string // 连接ID (UUIDv7)
	UserID    string // 用户ID (认证后设置)
	SessionID string // 会话ID
	
	// WebSocket连接
	conn *websocket.Conn
	
	// 通道
	send chan *Message // 发送消息通道
	
	// 状态
	authenticated bool      // 是否已认证
	lastPing      time.Time // 最后心跳时间
	closed        bool      // 是否已关闭
	
	// 订阅
	subscriptions map[string]bool // 订阅的频道
	
	// 同步
	mu     sync.RWMutex
	logger *zap.Logger
	
	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// NewConnection 创建新连接
func NewConnection(id string, conn *websocket.Conn, logger *zap.Logger) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Connection{
		ID:            id,
		conn:          conn,
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

// Send 发送消息
func (c *Connection) Send(msg *Message) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()
	
	select {
	case c.send <- msg:
		return nil
	case <-c.ctx.Done():
		return ErrConnectionClosed
	default:
		// 发送通道满，丢弃消息
		c.logger.Warn("Send channel full, dropping message",
			zap.String("conn_id", c.ID),
			zap.String("msg_type", string(msg.Type)),
		)
		return ErrSendChannelFull
	}
}

// Close 关闭连接
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	close(c.send)
	
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsClosed 是否已关闭
func (c *Connection) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// SetAuthenticated 设置认证状态
func (c *Connection) SetAuthenticated(userID, sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.authenticated = true
	c.UserID = userID
	c.SessionID = sessionID
}

// IsAuthenticated 是否已认证
func (c *Connection) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authenticated
}

// UpdatePing 更新心跳时间
func (c *Connection) UpdatePing() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPing = time.Now()
}

// LastPing 获取最后心跳时间
func (c *Connection) LastPing() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastPing
}

// Subscribe 订阅频道
func (c *Connection) Subscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[channel] = true
	
	c.logger.Debug("Subscribed to channel",
		zap.String("conn_id", c.ID),
		zap.String("channel", channel),
	)
}

// Unsubscribe 取消订阅频道
func (c *Connection) Unsubscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscriptions, channel)
	
	c.logger.Debug("Unsubscribed from channel",
		zap.String("conn_id", c.ID),
		zap.String("channel", channel),
	)
}

// IsSubscribed 是否订阅了频道
func (c *Connection) IsSubscribed(channel string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subscriptions[channel]
}

// GetSubscriptions 获取所有订阅
func (c *Connection) GetSubscriptions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	channels := make([]string, 0, len(c.subscriptions))
	for channel := range c.subscriptions {
		channels = append(channels, channel)
	}
	return channels
}

// readPump 读取消息循环
func (c *Connection) readPump(handler MessageHandler) {
	defer func() {
		c.Close()
	}()
	
	// 设置读取超时
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.UpdatePing()
		return nil
	})
	
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("WebSocket read error",
					zap.String("conn_id", c.ID),
					zap.Error(err),
				)
			}
			break
		}
		
		// 解析消息
		msg, err := FromJSON(data)
		if err != nil {
			c.logger.Warn("Failed to parse message",
				zap.String("conn_id", c.ID),
				zap.Error(err),
			)
			c.Send(NewErrorMessage("Invalid message format"))
			continue
		}
		
		// 处理消息
		if handler != nil {
			handler.HandleMessage(c, msg)
		}
	}
}

// writePump 写入消息循环
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()
	
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 通道关闭
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			// 发送消息
			data, err := msg.ToJSON()
			if err != nil {
				c.logger.Error("Failed to marshal message",
					zap.String("conn_id", c.ID),
					zap.Error(err),
				)
				continue
			}
			
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.logger.Error("Failed to write message",
					zap.String("conn_id", c.ID),
					zap.Error(err),
				)
				return
			}
			
		case <-ticker.C:
			// 发送心跳
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			
		case <-c.ctx.Done():
			return
		}
	}
}

// Start 启动连接处理
func (c *Connection) Start(handler MessageHandler) {
	go c.writePump()
	go c.readPump(handler)
}
