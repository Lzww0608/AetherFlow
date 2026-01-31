package websocket

import (
	"net/http"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 在生产环境中应该检查Origin
		return true
	},
}

// Server WebSocket服务器
type Server struct {
	hub     *Hub
	logger  *zap.Logger
	handler MessageHandler
}

// NewServer 创建WebSocket服务器
func NewServer(logger *zap.Logger) *Server {
	hub := NewHub(logger, nil)
	handler := NewDefaultHandler(hub, logger)
	hub.handler = handler
	
	return &Server{
		hub:     hub,
		logger:  logger,
		handler: handler,
	}
}

// GetHub 获取Hub
func (s *Server) GetHub() *Hub {
	return s.hub
}

// SetMessageHandler 设置消息处理器
func (s *Server) SetMessageHandler(handler MessageHandler) {
	s.handler = handler
	s.hub.handler = handler
}

// SetAuthFunc 设置认证函数
func (s *Server) SetAuthFunc(f func(string) (string, string, error)) {
	if defaultHandler, ok := s.handler.(*DefaultHandler); ok {
		defaultHandler.SetAuthFunc(f)
	}
}

// HandleWebSocket 处理WebSocket连接升级
func (s *Server) HandleWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 升级为WebSocket连接
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			s.logger.Error("Failed to upgrade connection",
				zap.Error(err),
				zap.String("remote_addr", r.RemoteAddr),
			)
			return
		}
		
		// 生成连接ID
		connID, err := guuid.NewV7()
		if err != nil {
			s.logger.Error("Failed to generate connection ID", zap.Error(err))
			conn.Close()
			return
		}
		
		// 创建连接对象
		wsConn := NewConnection(connID.String(), conn, s.logger)
		
		// 注册连接
		s.hub.Register(wsConn)
		
		// 启动连接处理
		wsConn.Start(s.handler)
		
		// 连接关闭后注销
		defer s.hub.Unregister(wsConn.ID)
		
		s.logger.Info("WebSocket connection established",
			zap.String("conn_id", wsConn.ID),
			zap.String("remote_addr", r.RemoteAddr),
		)
	}
}

// Broadcast 广播消息
func (s *Server) Broadcast(msg *Message) int {
	return s.hub.Broadcast(msg)
}

// BroadcastToChannel 广播到频道
func (s *Server) BroadcastToChannel(channel string, msg *Message) int {
	return s.hub.BroadcastToChannel(channel, msg)
}

// SendToUser 发送消息给用户
func (s *Server) SendToUser(userID string, msg *Message) int {
	return s.hub.SendToUser(userID, msg)
}

// GetStats 获取统计信息
func (s *Server) GetStats() map[string]interface{} {
	return s.hub.GetStats()
}

// Close 关闭服务器
func (s *Server) Close() {
	s.hub.Close()
}
