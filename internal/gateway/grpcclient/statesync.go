package grpcclient

import (
	"context"
	"fmt"
	"time"

	pb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// StateSyncClient StateSync服务客户端封装
type StateSyncClient struct {
	manager    *Manager
	poolName   string
	timeout    time.Duration
	maxRetries int
	logger     *zap.Logger
}

// NewStateSyncClient 创建StateSync服务客户端
func NewStateSyncClient(manager *Manager, poolName string, timeout time.Duration, maxRetries int, logger *zap.Logger) *StateSyncClient {
	return &StateSyncClient{
		manager:    manager,
		poolName:   poolName,
		timeout:    timeout,
		maxRetries: maxRetries,
		logger:     logger,
	}
}

// withRetry 带重试的执行函数
func (c *StateSyncClient) withRetry(ctx context.Context, fn func(client pb.StateSyncServiceClient) error) error {
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
		client := pb.NewStateSyncServiceClient(conn)

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

// CreateDocument 创建文档
func (c *StateSyncClient) CreateDocument(ctx context.Context, req *pb.CreateDocumentRequest) (*pb.CreateDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.CreateDocumentResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.CreateDocument(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetDocument 获取文档
func (c *StateSyncClient) GetDocument(ctx context.Context, req *pb.GetDocumentRequest) (*pb.GetDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.GetDocumentResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.GetDocument(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// UpdateDocument 更新文档
func (c *StateSyncClient) UpdateDocument(ctx context.Context, req *pb.UpdateDocumentRequest) (*pb.UpdateDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.UpdateDocumentResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.UpdateDocument(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// DeleteDocument 删除文档
func (c *StateSyncClient) DeleteDocument(ctx context.Context, req *pb.DeleteDocumentRequest) (*pb.DeleteDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.DeleteDocumentResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.DeleteDocument(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListDocuments 列出文档
func (c *StateSyncClient) ListDocuments(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.ListDocumentsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.ListDocumentsResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.ListDocuments(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ApplyOperation 应用操作
func (c *StateSyncClient) ApplyOperation(ctx context.Context, req *pb.ApplyOperationRequest) (*pb.ApplyOperationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.ApplyOperationResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.ApplyOperation(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetOperationHistory 获取操作历史
func (c *StateSyncClient) GetOperationHistory(ctx context.Context, req *pb.GetOperationHistoryRequest) (*pb.GetOperationHistoryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.GetOperationHistoryResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.GetOperationHistory(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// SubscribeDocument 订阅文档 (流式RPC)
func (c *StateSyncClient) SubscribeDocument(ctx context.Context, req *pb.SubscribeDocumentRequest) (pb.StateSyncService_SubscribeDocumentClient, *grpc.ClientConn, error) {
	// 流式RPC不使用重试机制
	conn, err := c.manager.GetConnection(ctx, c.poolName)
	if err != nil {
		return nil, nil, err
	}

	client := pb.NewStateSyncServiceClient(conn)
	stream, err := client.SubscribeDocument(ctx, req)
	if err != nil {
		c.manager.PutConnection(c.poolName, conn)
		return nil, nil, err
	}

	// 注意：调用者需要在使用完后归还连接
	return stream, conn, nil
}

// UnsubscribeDocument 取消订阅文档
func (c *StateSyncClient) UnsubscribeDocument(ctx context.Context, req *pb.UnsubscribeDocumentRequest) (*pb.UnsubscribeDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.UnsubscribeDocumentResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.UnsubscribeDocument(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// AcquireLock 获取锁
func (c *StateSyncClient) AcquireLock(ctx context.Context, req *pb.AcquireLockRequest) (*pb.AcquireLockResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.AcquireLockResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.AcquireLock(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ReleaseLock 释放锁
func (c *StateSyncClient) ReleaseLock(ctx context.Context, req *pb.ReleaseLockRequest) (*pb.ReleaseLockResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.ReleaseLockResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.ReleaseLock(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// IsLocked 检查锁状态
func (c *StateSyncClient) IsLocked(ctx context.Context, req *pb.IsLockedRequest) (*pb.IsLockedResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.IsLockedResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.IsLocked(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetStats 获取统计信息
func (c *StateSyncClient) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp *pb.GetStatsResponse
	err := c.withRetry(ctx, func(client pb.StateSyncServiceClient) error {
		var err error
		resp, err = client.GetStats(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// WithConnection 执行自定义操作
func (c *StateSyncClient) WithConnection(ctx context.Context, fn func(conn *grpc.ClientConn) error) error {
	conn, err := c.manager.GetConnection(ctx, c.poolName)
	if err != nil {
		return err
	}
	defer c.manager.PutConnection(c.poolName, conn)

	return fn(conn)
}
