package grpcclient

import (
	"context"

	pb_session "github.com/aetherflow/aetherflow/api/proto/session"
	pb_statesync "github.com/aetherflow/aetherflow/api/proto/statesync"
	"github.com/aetherflow/aetherflow/internal/gateway/breaker"
)

// BreakerSessionClient 带熔断保护的Session客户端
type BreakerSessionClient struct {
	client  *SessionClient
	breaker *breaker.CircuitBreaker
}

// NewBreakerSessionClient 创建带熔断保护的Session客户端
func NewBreakerSessionClient(client *SessionClient, breakerInstance *breaker.CircuitBreaker) *BreakerSessionClient {
	return &BreakerSessionClient{
		client:  client,
		breaker: breakerInstance,
	}
}

// CreateSession 创建会话（带熔断保护）
func (c *BreakerSessionClient) CreateSession(ctx context.Context, req *pb_session.CreateSessionRequest) (*pb_session.CreateSessionResponse, error) {
	var resp *pb_session.CreateSessionResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.CreateSession(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// GetSession 获取会话（带熔断保护）
func (c *BreakerSessionClient) GetSession(ctx context.Context, req *pb_session.GetSessionRequest) (*pb_session.GetSessionResponse, error) {
	var resp *pb_session.GetSessionResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.GetSession(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// UpdateSession 更新会话（带熔断保护）
func (c *BreakerSessionClient) UpdateSession(ctx context.Context, req *pb_session.UpdateSessionRequest) (*pb_session.UpdateSessionResponse, error) {
	var resp *pb_session.UpdateSessionResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.UpdateSession(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// DeleteSession 删除会话（带熔断保护）
func (c *BreakerSessionClient) DeleteSession(ctx context.Context, req *pb_session.DeleteSessionRequest) (*pb_session.DeleteSessionResponse, error) {
	var resp *pb_session.DeleteSessionResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.DeleteSession(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// ListSessions 列出会话（带熔断保护）
func (c *BreakerSessionClient) ListSessions(ctx context.Context, req *pb_session.ListSessionsRequest) (*pb_session.ListSessionsResponse, error) {
	var resp *pb_session.ListSessionsResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.ListSessions(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// Heartbeat 心跳（带熔断保护）
func (c *BreakerSessionClient) Heartbeat(ctx context.Context, req *pb_session.HeartbeatRequest) (*pb_session.HeartbeatResponse, error) {
	var resp *pb_session.HeartbeatResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.Heartbeat(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// BreakerStateSyncClient 带熔断保护的StateSync客户端
type BreakerStateSyncClient struct {
	client  *StateSyncClient
	breaker *breaker.CircuitBreaker
}

// NewBreakerStateSyncClient 创建带熔断保护的StateSync客户端
func NewBreakerStateSyncClient(client *StateSyncClient, breakerInstance *breaker.CircuitBreaker) *BreakerStateSyncClient {
	return &BreakerStateSyncClient{
		client:  client,
		breaker: breakerInstance,
	}
}

// CreateDocument 创建文档（带熔断保护）
func (c *BreakerStateSyncClient) CreateDocument(ctx context.Context, req *pb_statesync.CreateDocumentRequest) (*pb_statesync.CreateDocumentResponse, error) {
	var resp *pb_statesync.CreateDocumentResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.CreateDocument(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// GetDocument 获取文档（带熔断保护）
func (c *BreakerStateSyncClient) GetDocument(ctx context.Context, req *pb_statesync.GetDocumentRequest) (*pb_statesync.GetDocumentResponse, error) {
	var resp *pb_statesync.GetDocumentResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.GetDocument(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}

// ApplyOperation 应用操作（带熔断保护）
func (c *BreakerStateSyncClient) ApplyOperation(ctx context.Context, req *pb_statesync.ApplyOperationRequest) (*pb_statesync.ApplyOperationResponse, error) {
	var resp *pb_statesync.ApplyOperationResponse
	var err error
	
	breakerErr := c.breaker.ExecuteContext(ctx, func(ctx context.Context) error {
		resp, err = c.client.ApplyOperation(ctx, req)
		return err
	})
	
	if breakerErr != nil {
		return nil, breakerErr
	}
	
	return resp, err
}
