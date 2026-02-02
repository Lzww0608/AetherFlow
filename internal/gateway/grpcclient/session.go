package grpcclient

import (
	"context"
	"fmt"
	"time"

	pb "github.com/aetherflow/aetherflow/api/proto/session"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// SessionClient Session服务客户端封装
type SessionClient struct {
	manager    *Manager
	poolName   string
	timeout    time.Duration
	maxRetries int
	logger     *zap.Logger
}

// NewSessionClient 创建Session服务客户端
func NewSessionClient(manager *Manager, poolName string, timeout time.Duration, maxRetries int, logger *zap.Logger) *SessionClient {
	return &SessionClient{
		manager:    manager,
		poolName:   poolName,
		timeout:    timeout,
		maxRetries: maxRetries,
		logger:     logger,
	}
}

// withRetry 带重试的执行函数
func (c *SessionClient) withRetry(ctx context.Context, fn func(client pb.SessionServiceClient) error) error {
	var lastErr error
	
	for i := 0; i <= c.maxRetries; i++ {
		// 获取连接
		conn, err := c.manager.GetConnection(ctx, c.poolName)
		if err != nil {
			lastErr = err
			c.logger.Warn("Failed to get connection",
				zap.Int("attempt", i+1),
				zap.Error(err),
			)
			time.Sleep(time.Duration(i*100) * time.Millisecond)
			continue
		}

		// 创建客户端
		client := pb.NewSessionServiceClient(conn)

		// 执行请求
		err = fn(client)
		
		// 归还连接
		c.manager.PutConnection(c.poolName, conn)

		if err == nil {
			return nil
		}

		lastErr = err
		c.logger.Warn("Request failed",
			zap.Int("attempt", i+1),
			zap.Error(err),
		)

		if i < c.maxRetries {
			time.Sleep(time.Duration((i+1)*100) * time.Millisecond)
		}
	}

	return fmt.Errorf("request failed after %d retries: %w", c.maxRetries+1, lastErr)
}

// CreateSession 创建会话
func (c *SessionClient) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.CreateSessionResponse
	err := c.withRetry(ctx, func(client pb.SessionServiceClient) error {
		var err error
		resp, err = client.CreateSession(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetSession 获取会话
func (c *SessionClient) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.GetSessionResponse
	err := c.withRetry(ctx, func(client pb.SessionServiceClient) error {
		var err error
		resp, err = client.GetSession(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// UpdateSession 更新会话
func (c *SessionClient) UpdateSession(ctx context.Context, req *pb.UpdateSessionRequest) (*pb.UpdateSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.UpdateSessionResponse
	err := c.withRetry(ctx, func(client pb.SessionServiceClient) error {
		var err error
		resp, err = client.UpdateSession(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// DeleteSession 删除会话
func (c *SessionClient) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.DeleteSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.DeleteSessionResponse
	err := c.withRetry(ctx, func(client pb.SessionServiceClient) error {
		var err error
		resp, err = client.DeleteSession(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListSessions 列出会话
func (c *SessionClient) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.ListSessionsResponse
	err := c.withRetry(ctx, func(client pb.SessionServiceClient) error {
		var err error
		resp, err = client.ListSessions(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Heartbeat 会话心跳
func (c *SessionClient) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.HeartbeatResponse
	err := c.withRetry(ctx, func(client pb.SessionServiceClient) error {
		var err error
		resp, err = client.Heartbeat(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// WithConnection 执行自定义操作
func (c *SessionClient) WithConnection(ctx context.Context, fn func(conn *grpc.ClientConn) error) error {
	conn, err := c.manager.GetConnection(ctx, c.poolName)
	if err != nil {
		return err
	}
	defer c.manager.PutConnection(c.poolName, conn)

	return fn(conn)
}
