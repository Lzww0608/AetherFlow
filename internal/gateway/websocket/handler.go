package websocket

import (
	"go.uber.org/zap"
)

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(conn *Connection, msg *Message)
}

// DefaultHandler 默认消息处理器
type DefaultHandler struct {
	hub    *Hub
	logger *zap.Logger
	
	// 认证回调 (可选)
	authFunc func(token string) (userID, sessionID string, err error)
}

// NewDefaultHandler 创建默认处理器
func NewDefaultHandler(hub *Hub, logger *zap.Logger) *DefaultHandler {
	return &DefaultHandler{
		hub:    hub,
		logger: logger,
	}
}

// SetAuthFunc 设置认证函数
func (h *DefaultHandler) SetAuthFunc(f func(string) (string, string, error)) {
	h.authFunc = f
}

// HandleMessage 处理消息
func (h *DefaultHandler) HandleMessage(conn *Connection, msg *Message) {
	h.logger.Debug("Handling message",
		zap.String("conn_id", conn.ID),
		zap.String("msg_type", string(msg.Type)),
		zap.String("msg_id", msg.ID),
	)
	
	switch msg.Type {
	case MessageTypePing:
		h.handlePing(conn, msg)
		
	case MessageTypeAuth:
		h.handleAuth(conn, msg)
		
	case MessageTypeSubscribe:
		h.handleSubscribe(conn, msg)
		
	case MessageTypeUnsubscribe:
		h.handleUnsubscribe(conn, msg)
		
	case MessageTypePublish:
		h.handlePublish(conn, msg)
		
	default:
		h.logger.Warn("Unknown message type",
			zap.String("conn_id", conn.ID),
			zap.String("msg_type", string(msg.Type)),
		)
		conn.Send(NewErrorMessage("Unknown message type"))
	}
}

// handlePing 处理Ping消息
func (h *DefaultHandler) handlePing(conn *Connection, msg *Message) {
	pong := NewMessage(MessageTypePong, map[string]interface{}{
		"timestamp": msg.Timestamp,
	})
	conn.Send(pong)
}

// handleAuth 处理认证消息
func (h *DefaultHandler) handleAuth(conn *Connection, msg *Message) {
	// 解析认证数据
	authData, ok := msg.Data.(map[string]interface{})
	if !ok {
		conn.Send(&Message{
			ID:    newMessageID(),
			Type:  MessageTypeAuthResult,
			Data: AuthResult{
				Success: false,
				Message: "Invalid auth data format",
			},
		})
		return
	}
	
	token, ok := authData["token"].(string)
	if !ok || token == "" {
		conn.Send(&Message{
			ID:    newMessageID(),
			Type:  MessageTypeAuthResult,
			Data: AuthResult{
				Success: false,
				Message: "Token is required",
			},
		})
		return
	}
	
	// 调用认证函数
	var userID, sessionID string
	var err error
	
	if h.authFunc != nil {
		userID, sessionID, err = h.authFunc(token)
		if err != nil {
			conn.Send(&Message{
				ID:    newMessageID(),
				Type:  MessageTypeAuthResult,
				Data: AuthResult{
					Success: false,
					Message: "Authentication failed: " + err.Error(),
				},
			})
			return
		}
	} else {
		// 如果没有设置认证函数，使用测试模式
		userID = "test_user"
		sessionID = "test_session"
	}
	
	// 设置连接的用户ID
	h.hub.SetUserID(conn.ID, userID, sessionID)
	
	// 发送认证成功响应
	conn.Send(&Message{
		ID:    newMessageID(),
		Type:  MessageTypeAuthResult,
		Data: AuthResult{
			Success:   true,
			Message:   "Authentication successful",
			UserID:    userID,
			SessionID: sessionID,
		},
	})
}

// handleSubscribe 处理订阅消息
func (h *DefaultHandler) handleSubscribe(conn *Connection, msg *Message) {
	if !conn.IsAuthenticated() {
		conn.Send(NewErrorMessage("Not authenticated"))
		return
	}
	
	// 解析订阅数据
	subData, ok := msg.Data.(map[string]interface{})
	if !ok {
		conn.Send(NewErrorMessage("Invalid subscribe data format"))
		return
	}
	
	channel, ok := subData["channel"].(string)
	if !ok || channel == "" {
		conn.Send(NewErrorMessage("Channel is required"))
		return
	}
	
	// 订阅频道
	if err := h.hub.SubscribeChannel(conn.ID, channel); err != nil {
		conn.Send(NewErrorMessage("Failed to subscribe: " + err.Error()))
		return
	}
	
	// 发送订阅成功响应
	conn.Send(NewMessage(MessageTypeSubscribe, map[string]interface{}{
		"channel": channel,
		"success": true,
	}))
}

// handleUnsubscribe 处理取消订阅消息
func (h *DefaultHandler) handleUnsubscribe(conn *Connection, msg *Message) {
	if !conn.IsAuthenticated() {
		conn.Send(NewErrorMessage("Not authenticated"))
		return
	}
	
	// 解析取消订阅数据
	unsubData, ok := msg.Data.(map[string]interface{})
	if !ok {
		conn.Send(NewErrorMessage("Invalid unsubscribe data format"))
		return
	}
	
	channel, ok := unsubData["channel"].(string)
	if !ok || channel == "" {
		conn.Send(NewErrorMessage("Channel is required"))
		return
	}
	
	// 取消订阅频道
	if err := h.hub.UnsubscribeChannel(conn.ID, channel); err != nil {
		conn.Send(NewErrorMessage("Failed to unsubscribe: " + err.Error()))
		return
	}
	
	// 发送取消订阅成功响应
	conn.Send(NewMessage(MessageTypeUnsubscribe, map[string]interface{}{
		"channel": channel,
		"success": true,
	}))
}

// handlePublish 处理发布消息
func (h *DefaultHandler) handlePublish(conn *Connection, msg *Message) {
	if !conn.IsAuthenticated() {
		conn.Send(NewErrorMessage("Not authenticated"))
		return
	}
	
	// 解析发布数据
	pubData, ok := msg.Data.(map[string]interface{})
	if !ok {
		conn.Send(NewErrorMessage("Invalid publish data format"))
		return
	}
	
	channel, ok := pubData["channel"].(string)
	if !ok || channel == "" {
		conn.Send(NewErrorMessage("Channel is required"))
		return
	}
	
	data := pubData["data"]
	
	// 创建通知消息
	notifyMsg := NewMessage(MessageTypeNotify, NotifyData{
		Channel: channel,
		Event:   "publish",
		Data:    data,
	})
	
	// 广播到频道
	count := h.hub.BroadcastToChannel(channel, notifyMsg)
	
	h.logger.Debug("Message published to channel",
		zap.String("channel", channel),
		zap.Int("subscribers", count),
	)
	
	// 发送发布成功响应
	conn.Send(NewMessage(MessageTypePublish, map[string]interface{}{
		"channel":     channel,
		"success":     true,
		"subscribers": count,
	}))
}
