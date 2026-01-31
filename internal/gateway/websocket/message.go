package websocket

import (
	"encoding/json"
	"time"

	guuid "github.com/Lzww0608/GUUID"
)

// MessageType 消息类型
type MessageType string

const (
	// 系统消息
	MessageTypePing       MessageType = "ping"        // 心跳请求
	MessageTypePong       MessageType = "pong"        // 心跳响应
	MessageTypeAuth       MessageType = "auth"        // 认证请求
	MessageTypeAuthResult MessageType = "auth_result" // 认证结果
	MessageTypeError      MessageType = "error"       // 错误消息
	
	// 业务消息
	MessageTypeSubscribe   MessageType = "subscribe"   // 订阅
	MessageTypeUnsubscribe MessageType = "unsubscribe" // 取消订阅
	MessageTypePublish     MessageType = "publish"     // 发布
	MessageTypeNotify      MessageType = "notify"      // 通知
)

// Message WebSocket 消息结构
type Message struct {
	ID        string      `json:"id"`                  // 消息ID (UUIDv7)
	Type      MessageType `json:"type"`                // 消息类型
	Timestamp time.Time   `json:"timestamp"`           // 时间戳
	Data      interface{} `json:"data,omitempty"`      // 消息数据
	RequestID string      `json:"request_id,omitempty"` // 关联的请求ID
	Error     string      `json:"error,omitempty"`     // 错误信息
}

// NewMessage 创建新消息
func NewMessage(msgType MessageType, data interface{}) *Message {
	id, _ := guuid.NewV7()
	return &Message{
		ID:        id.String(),
		Type:      msgType,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// NewErrorMessage 创建错误消息
func NewErrorMessage(err string) *Message {
	return &Message{
		ID:        newMessageID(),
		Type:      MessageTypeError,
		Timestamp: time.Now(),
		Error:     err,
	}
}

// ToJSON 转换为JSON
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON 从JSON解析
func FromJSON(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// AuthData 认证数据
type AuthData struct {
	Token     string `json:"token"`               // JWT Token
	UserID    string `json:"user_id,omitempty"`   // 用户ID
	SessionID string `json:"session_id,omitempty"` // 会话ID
}

// AuthResult 认证结果
type AuthResult struct {
	Success   bool   `json:"success"`             // 是否成功
	Message   string `json:"message,omitempty"`   // 消息
	UserID    string `json:"user_id,omitempty"`   // 用户ID
	SessionID string `json:"session_id,omitempty"` // 会话ID
}

// SubscribeData 订阅数据
type SubscribeData struct {
	Channel string `json:"channel"` // 频道名称
	Filter  string `json:"filter,omitempty"` // 过滤条件
}

// PublishData 发布数据
type PublishData struct {
	Channel string      `json:"channel"` // 频道名称
	Data    interface{} `json:"data"`    // 发布数据
}

// NotifyData 通知数据
type NotifyData struct {
	Channel string      `json:"channel"` // 频道名称
	Event   string      `json:"event"`   // 事件类型
	Data    interface{} `json:"data"`    // 通知数据
}

// newMessageID 生成消息ID
func newMessageID() string {
	id, err := guuid.NewV7()
	if err != nil {
		return "unknown"
	}
	return id.String()
}
