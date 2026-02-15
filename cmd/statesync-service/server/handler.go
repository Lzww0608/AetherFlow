package server

import (
	"context"
	"fmt"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	pb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"github.com/aetherflow/aetherflow/internal/statesync"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateDocument 创建文档
func (s *Server) CreateDocument(ctx context.Context, req *pb.CreateDocumentRequest) (*pb.CreateDocumentResponse, error) {
	s.logger.Info("CreateDocument called",
		zap.String("name", req.Name),
		zap.String("type", req.Type),
		zap.String("created_by", req.CreatedBy))

	// 验证请求
	if req.Name == "" {
		return &pb.CreateDocumentResponse{
			Error: "name is required",
		}, nil
	}
	if req.CreatedBy == "" {
		return &pb.CreateDocumentResponse{
			Error: "created_by is required",
		}, nil
	}

	// 转换文档类型
	docType := statesync.DocumentType(req.Type)

	// 创建文档
	doc, err := s.manager.CreateDocument(ctx, req.Name, docType, req.CreatedBy, req.Content)
	if err != nil {
		s.logger.Error("Failed to create document", zap.Error(err))
		return &pb.CreateDocumentResponse{
			Error: err.Error(),
		}, nil
	}

	s.logger.Info("Document created successfully", zap.String("doc_id", doc.ID.String()))

	// 转换为 proto
	pbDoc, err := documentToProto(doc)
	if err != nil {
		return &pb.CreateDocumentResponse{
			Error: err.Error(),
		}, nil
	}

	return &pb.CreateDocumentResponse{
		Document: pbDoc,
	}, nil
}

// GetDocument 获取文档
func (s *Server) GetDocument(ctx context.Context, req *pb.GetDocumentRequest) (*pb.GetDocumentResponse, error) {
	s.logger.Debug("GetDocument called", zap.String("doc_id", req.DocId))

	// 解析文档 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return &pb.GetDocumentResponse{
			Error: "invalid doc_id format",
		}, nil
	}

	// 获取文档
	doc, err := s.manager.GetDocument(ctx, docID)
	if err != nil {
		s.logger.Error("Failed to get document", zap.Error(err))
		return &pb.GetDocumentResponse{
			Error: err.Error(),
		}, nil
	}

	// 转换为 proto
	pbDoc, err := documentToProto(doc)
	if err != nil {
		return &pb.GetDocumentResponse{
			Error: err.Error(),
		}, nil
	}

	return &pb.GetDocumentResponse{
		Document: pbDoc,
	}, nil
}

// UpdateDocument 更新文档
func (s *Server) UpdateDocument(ctx context.Context, req *pb.UpdateDocumentRequest) (*pb.UpdateDocumentResponse, error) {
	s.logger.Info("UpdateDocument called", zap.String("doc_id", req.Document.Id))

	if req.Document == nil {
		return &pb.UpdateDocumentResponse{
			Success: false,
			Error:   "document is required",
		}, nil
	}

	// 转换为内部文档
	doc, err := protoToDocument(req.Document)
	if err != nil {
		return &pb.UpdateDocumentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 更新文档
	err = s.manager.UpdateDocument(ctx, doc)
	if err != nil {
		s.logger.Error("Failed to update document", zap.Error(err))
		return &pb.UpdateDocumentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.UpdateDocumentResponse{
		Success: true,
	}, nil
}

// DeleteDocument 删除文档
func (s *Server) DeleteDocument(ctx context.Context, req *pb.DeleteDocumentRequest) (*pb.DeleteDocumentResponse, error) {
	s.logger.Info("DeleteDocument called",
		zap.String("doc_id", req.DocId),
		zap.String("user_id", req.UserId))

	// 解析文档 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return &pb.DeleteDocumentResponse{
			Success: false,
			Error:   "invalid doc_id format",
		}, nil
	}

	// 删除文档
	err = s.manager.DeleteDocument(ctx, docID, req.UserId)
	if err != nil {
		s.logger.Error("Failed to delete document", zap.Error(err))
		return &pb.DeleteDocumentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.DeleteDocumentResponse{
		Success: true,
	}, nil
}

// ListDocuments 列出文档
func (s *Server) ListDocuments(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.ListDocumentsResponse, error) {
	s.logger.Debug("ListDocuments called",
		zap.String("type", req.Type),
		zap.String("state", req.State))

	// 构建过滤器
	filter := &statesync.DocumentFilter{
		Offset: int(req.Offset),
		Limit:  int(req.Limit),
	}

	if req.Type != "" {
		docType := statesync.DocumentType(req.Type)
		filter.Type = &docType
	}

	if req.State != "" {
		docState := statesync.DocumentState(req.State)
		filter.State = &docState
	}

	if req.CreatedBy != "" {
		filter.CreatedBy = &req.CreatedBy
	}

	if req.UserId != "" {
		filter.UserID = &req.UserId
	}

	// 列出文档
	docs, total, err := s.manager.ListDocuments(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list documents", zap.Error(err))
		return &pb.ListDocumentsResponse{
			Error: err.Error(),
		}, nil
	}

	// 转换为 proto
	pbDocs := make([]*pb.Document, 0, len(docs))
	for _, doc := range docs {
		pbDoc, err := documentToProto(doc)
		if err != nil {
			s.logger.Error("Failed to convert document", zap.Error(err))
			continue
		}
		pbDocs = append(pbDocs, pbDoc)
	}

	return &pb.ListDocumentsResponse{
		Documents: pbDocs,
		Total:     int32(total),
	}, nil
}

// ApplyOperation 应用操作
func (s *Server) ApplyOperation(ctx context.Context, req *pb.ApplyOperationRequest) (*pb.ApplyOperationResponse, error) {
	s.logger.Debug("ApplyOperation called",
		zap.String("op_type", req.Operation.Type),
		zap.String("doc_id", req.Operation.DocId))

	// 转换为内部操作
	op, err := protoToOperation(req.Operation)
	if err != nil {
		return &pb.ApplyOperationResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 应用操作
	err = s.manager.ApplyOperation(ctx, op)
	if err != nil {
		s.logger.Error("Failed to apply operation", zap.Error(err))
		return &pb.ApplyOperationResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换为 proto
	pbOp, err := operationToProto(op)
	if err != nil {
		return &pb.ApplyOperationResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.ApplyOperationResponse{
		Success:          true,
		AppliedOperation: pbOp,
	}, nil
}

// GetOperationHistory 获取操作历史
func (s *Server) GetOperationHistory(ctx context.Context, req *pb.GetOperationHistoryRequest) (*pb.GetOperationHistoryResponse, error) {
	s.logger.Debug("GetOperationHistory called",
		zap.String("doc_id", req.DocId),
		zap.Int32("limit", req.Limit))

	// 解析文档 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return &pb.GetOperationHistoryResponse{
			Error: "invalid doc_id format",
		}, nil
	}

	// 构建过滤器
	filter := &statesync.OperationFilter{
		DocID: &docID,
		Limit: int(req.Limit),
	}

	// 获取操作历史  
	ops, _, err := s.manager.GetStore().ListOperations(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get operation history", zap.Error(err))
		return &pb.GetOperationHistoryResponse{
			Error: err.Error(),
		}, nil
	}

	// 转换为 proto
	pbOps := make([]*pb.Operation, 0, len(ops))
	for _, op := range ops {
		pbOp, err := operationToProto(op)
		if err != nil {
			s.logger.Error("Failed to convert operation", zap.Error(err))
			continue
		}
		pbOps = append(pbOps, pbOp)
	}

	return &pb.GetOperationHistoryResponse{
		Operations: pbOps,
	}, nil
}

// UnsubscribeDocument 取消订阅文档
func (s *Server) UnsubscribeDocument(ctx context.Context, req *pb.UnsubscribeDocumentRequest) (*pb.UnsubscribeDocumentResponse, error) {
	s.logger.Info("UnsubscribeDocument called",
		zap.String("subscriber_id", req.SubscriberId),
		zap.String("doc_id", req.DocId))

	// 解析文档 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return &pb.UnsubscribeDocumentResponse{
			Success: false,
			Error:   "invalid doc_id format",
		}, nil
	}

	// 取消订阅
	err = s.manager.Unsubscribe(ctx, req.SubscriberId, docID, req.UserId)
	if err != nil {
		s.logger.Error("Failed to unsubscribe", zap.Error(err))
		return &pb.UnsubscribeDocumentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.UnsubscribeDocumentResponse{
		Success: true,
	}, nil
}

// AcquireLock 获取锁
func (s *Server) AcquireLock(ctx context.Context, req *pb.AcquireLockRequest) (*pb.AcquireLockResponse, error) {
	s.logger.Info("AcquireLock called",
		zap.String("doc_id", req.DocId),
		zap.String("user_id", req.UserId))

	// 解析 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return &pb.AcquireLockResponse{
			Error: "invalid doc_id format",
		}, nil
	}

	sessionID, err := guuid.Parse(req.SessionId)
	if err != nil {
		return &pb.AcquireLockResponse{
			Error: "invalid session_id format",
		}, nil
	}

	// 获取锁
	lock, err := s.manager.AcquireLock(ctx, docID, req.UserId, sessionID)
	if err != nil {
		s.logger.Error("Failed to acquire lock", zap.Error(err))
		return &pb.AcquireLockResponse{
			Error: err.Error(),
		}, nil
	}

	// 转换为 proto
	pbLock := lockToProto(lock)

	return &pb.AcquireLockResponse{
		Lock: pbLock,
	}, nil
}

// ReleaseLock 释放锁
func (s *Server) ReleaseLock(ctx context.Context, req *pb.ReleaseLockRequest) (*pb.ReleaseLockResponse, error) {
	s.logger.Info("ReleaseLock called",
		zap.String("doc_id", req.DocId),
		zap.String("user_id", req.UserId))

	// 解析文档 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return &pb.ReleaseLockResponse{
			Success: false,
			Error:   "invalid doc_id format",
		}, nil
	}

	// 释放锁
	err = s.manager.ReleaseLock(ctx, docID, req.UserId)
	if err != nil {
		s.logger.Error("Failed to release lock", zap.Error(err))
		return &pb.ReleaseLockResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.ReleaseLockResponse{
		Success: true,
	}, nil
}

// IsLocked 检查是否已锁定
func (s *Server) IsLocked(ctx context.Context, req *pb.IsLockedRequest) (*pb.IsLockedResponse, error) {
	s.logger.Debug("IsLocked called", zap.String("doc_id", req.DocId))

	// 解析文档 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return &pb.IsLockedResponse{
			Locked: false,
			Error:  "invalid doc_id format",
		}, nil
	}

	// 检查锁
	locked, err := s.manager.IsLocked(ctx, docID)
	if err != nil {
		s.logger.Error("Failed to check lock", zap.Error(err))
		return &pb.IsLockedResponse{
			Locked: false,
			Error:  err.Error(),
		}, nil
	}

	if !locked {
		return &pb.IsLockedResponse{
			Locked: false,
		}, nil
	}

	// 获取锁详情
	lockInfo, err := s.manager.GetStore().GetLock(ctx, docID)
	if err != nil || lockInfo == nil {
		return &pb.IsLockedResponse{
			Locked: true,
		}, nil
	}

	// 转换为 proto
	pbLock := lockToProto(lockInfo)

	return &pb.IsLockedResponse{
		Locked: true,
		Lock:   pbLock,
	}, nil
}

// GetStats 获取统计信息
func (s *Server) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	s.logger.Debug("GetStats called")

	// 获取统计信息
	stats, err := s.manager.GetStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get stats", zap.Error(err))
		return &pb.GetStatsResponse{
			Error: err.Error(),
		}, nil
	}

	// 转换为 proto
	pbStats := &pb.Stats{
		TotalDocuments:    stats.TotalDocuments,
		ActiveDocuments:   stats.ActiveDocuments,
		ArchivedDocuments: stats.ArchivedDocuments,
		TotalOperations:   stats.TotalOperations,
		TotalConflicts:    stats.TotalConflicts,
		ResolvedConflicts: stats.ResolvedConflicts,
		ActiveSubscribers: stats.ActiveSubscribers,
		ActiveLocks:       stats.ActiveLocks,
		LastUpdated:       timestamppb.New(stats.LastUpdated),
	}

	return &pb.GetStatsResponse{
		Stats: pbStats,
	}, nil
}

// ==================== 辅助转换函数 ====================

// documentToProto 将内部文档转换为 proto
func documentToProto(doc *statesync.Document) (*pb.Document, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}

	return &pb.Document{
		Id:          doc.ID.String(),
		Name:        doc.Name,
		Type:        string(doc.Type),
		State:       string(doc.State),
		Version:     doc.Version,
		Content:     doc.Content,
		CreatedBy:   doc.CreatedBy,
		CreatedAt:   timestamppb.New(doc.CreatedAt),
		UpdatedAt:   timestamppb.New(doc.UpdatedAt),
		UpdatedBy:   doc.UpdatedBy,
		ActiveUsers: doc.ActiveUsers,
		Metadata:    metadataToProto(&doc.Metadata),
	}, nil
}

// protoToDocument 将 proto 转换为内部文档
func protoToDocument(pbDoc *pb.Document) (*statesync.Document, error) {
	if pbDoc == nil {
		return nil, fmt.Errorf("proto document is nil")
	}

	docID, err := guuid.Parse(pbDoc.Id)
	if err != nil {
		return nil, fmt.Errorf("invalid document id: %w", err)
	}

	return &statesync.Document{
		ID:          docID,
		Name:        pbDoc.Name,
		Type:        statesync.DocumentType(pbDoc.Type),
		State:       statesync.DocumentState(pbDoc.State),
		Version:     pbDoc.Version,
		Content:     pbDoc.Content,
		CreatedBy:   pbDoc.CreatedBy,
		CreatedAt:   pbDoc.CreatedAt.AsTime(),
		UpdatedAt:   pbDoc.UpdatedAt.AsTime(),
		UpdatedBy:   pbDoc.UpdatedBy,
		ActiveUsers: pbDoc.ActiveUsers,
		Metadata:    protoMetadataToInternal(pbDoc.Metadata),
	}, nil
}

// operationToProto 将内部操作转换为 proto
func operationToProto(op *statesync.Operation) (*pb.Operation, error) {
	if op == nil {
		return nil, fmt.Errorf("operation is nil")
	}

	return &pb.Operation{
		Id:          op.ID.String(),
		DocId:       op.DocID.String(),
		UserId:      op.UserID,
		SessionId:   op.SessionID.String(),
		Type:        string(op.Type),
		Data:        op.Data,
		Timestamp:   timestamppb.New(op.Timestamp),
		Version:     op.Version,
		PrevVersion: op.PrevVersion,
		Status:      string(op.Status),
		ClientId:    op.ClientID,
		Metadata:    opMetadataToProto(&op.Metadata),
	}, nil
}

// protoToOperation 将 proto 转换为内部操作
func protoToOperation(pbOp *pb.Operation) (*statesync.Operation, error) {
	if pbOp == nil {
		return nil, fmt.Errorf("proto operation is nil")
	}

	var opID guuid.UUID
	var err error
	if pbOp.Id != "" {
		opID, err = guuid.Parse(pbOp.Id)
		if err != nil {
			return nil, fmt.Errorf("invalid operation id: %w", err)
		}
	} else {
		// 如果没有 ID，生成一个新的
		opID, err = guuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("failed to generate operation id: %w", err)
		}
	}

	docID, err := guuid.Parse(pbOp.DocId)
	if err != nil {
		return nil, fmt.Errorf("invalid doc id: %w", err)
	}

	sessionID, err := guuid.Parse(pbOp.SessionId)
	if err != nil {
		return nil, fmt.Errorf("invalid session id: %w", err)
	}

	timestamp := time.Now()
	if pbOp.Timestamp != nil {
		timestamp = pbOp.Timestamp.AsTime()
	}

	return &statesync.Operation{
		ID:          opID,
		DocID:       docID,
		UserID:      pbOp.UserId,
		SessionID:   sessionID,
		Type:        statesync.OperationType(pbOp.Type),
		Data:        pbOp.Data,
		Timestamp:   timestamp,
		Version:     pbOp.Version,
		PrevVersion: pbOp.PrevVersion,
		Status:      statesync.OperationStatus(pbOp.Status),
		ClientID:    pbOp.ClientId,
		Metadata:    protoOpMetadataToInternal(pbOp.Metadata),
	}, nil
}

// lockToProto 将内部锁转换为 proto
func lockToProto(lock *statesync.Lock) *pb.Lock {
	if lock == nil {
		return nil
	}

	return &pb.Lock{
		Id:         lock.ID.String(),
		DocId:      lock.DocID.String(),
		UserId:     lock.UserID,
		SessionId:  lock.SessionID.String(),
		AcquiredAt: timestamppb.New(lock.AcquiredAt),
		ExpiresAt:  timestamppb.New(lock.ExpiresAt),
		Active:     lock.Active,
	}
}

// metadataToProto 将内部元数据转换为 proto
func metadataToProto(meta *statesync.Metadata) *pb.Metadata {
	if meta == nil {
		return nil
	}

	return &pb.Metadata{
		Tags:        meta.Tags,
		Description: meta.Description,
		Properties:  meta.Properties,
		Permissions: permissionsToProto(&meta.Permissions),
	}
}

// protoMetadataToInternal 将 proto 元数据转换为内部
func protoMetadataToInternal(pbMeta *pb.Metadata) statesync.Metadata {
	if pbMeta == nil {
		return statesync.Metadata{}
	}

	return statesync.Metadata{
		Tags:        pbMeta.Tags,
		Description: pbMeta.Description,
		Properties:  pbMeta.Properties,
		Permissions: protoPermissionsToInternal(pbMeta.Permissions),
	}
}

// permissionsToProto 将内部权限转换为 proto
func permissionsToProto(perm *statesync.Permissions) *pb.Permissions {
	if perm == nil {
		return nil
	}

	return &pb.Permissions{
		Owner:   perm.Owner,
		Editors: perm.Editors,
		Viewers: perm.Viewers,
		Public:  perm.Public,
	}
}

// protoPermissionsToInternal 将 proto 权限转换为内部
func protoPermissionsToInternal(pbPerm *pb.Permissions) statesync.Permissions {
	if pbPerm == nil {
		return statesync.Permissions{}
	}

	return statesync.Permissions{
		Owner:   pbPerm.Owner,
		Editors: pbPerm.Editors,
		Viewers: pbPerm.Viewers,
		Public:  pbPerm.Public,
	}
}

// opMetadataToProto 将内部操作元数据转换为 proto
func opMetadataToProto(meta *statesync.OpMetadata) *pb.OpMetadata {
	if meta == nil {
		return nil
	}

	return &pb.OpMetadata{
		Ip:        meta.IP,
		UserAgent: meta.UserAgent,
		Platform:  meta.Platform,
		Extra:     meta.Extra,
	}
}

// protoOpMetadataToInternal 将 proto 操作元数据转换为内部
func protoOpMetadataToInternal(pbMeta *pb.OpMetadata) statesync.OpMetadata {
	if pbMeta == nil {
		return statesync.OpMetadata{}
	}

	return statesync.OpMetadata{
		IP:        pbMeta.Ip,
		UserAgent: pbMeta.UserAgent,
		Platform:  pbMeta.Platform,
		Extra:     pbMeta.Extra,
	}
}
