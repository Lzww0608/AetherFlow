package handler

import (
	"encoding/json"
	"net/http"

	pb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"github.com/aetherflow/aetherflow/internal/gateway/middleware"
	"github.com/aetherflow/aetherflow/internal/gateway/svc"
)

// CreateDocumentHandler 创建文档
func CreateDocumentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())

		if userID == "" {
			UnauthorizedResponse(w, "Not authenticated", requestID)
			return
		}

		var req struct {
			Name    string            `json:"name"`
			Type    string            `json:"type"`
			Content []byte            `json:"content"`
			Tags    []string          `json:"tags"`
			Metadata map[string]string `json:"metadata"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.CreateDocument(r.Context(), &pb.CreateDocumentRequest{
			Name:      req.Name,
			Type:      req.Type,
			CreatedBy: userID,
			Content:   req.Content,
			Metadata: &pb.Metadata{
				Tags:       req.Tags,
				Properties: req.Metadata,
			},
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to create document: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			InternalServerErrorResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, resp.Document, requestID)
	}
}

// GetDocumentHandler 获取文档
func GetDocumentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		docID := r.URL.Query().Get("doc_id")

		if docID == "" {
			BadRequestResponse(w, "doc_id is required", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.GetDocument(r.Context(), &pb.GetDocumentRequest{
			DocId: docID,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to get document: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			InternalServerErrorResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, resp.Document, requestID)
	}
}

// ListDocumentsHandler 列出文档
func ListDocumentsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.ListDocuments(r.Context(), &pb.ListDocumentsRequest{
			UserId: userID,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to list documents: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			InternalServerErrorResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"documents": resp.Documents,
			"total":     resp.Total,
		}, requestID)
	}
}

// ApplyOperationHandler 应用操作
func ApplyOperationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())

		if userID == "" {
			UnauthorizedResponse(w, "Not authenticated", requestID)
			return
		}

		var req struct {
			DocID   string `json:"doc_id"`
			Type    string `json:"type"`
			Data    []byte `json:"data"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.ApplyOperation(r.Context(), &pb.ApplyOperationRequest{
			Operation: &pb.Operation{
				DocId:  req.DocID,
				UserId: userID,
				Type:   req.Type,
				Data:   req.Data,
			},
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to apply operation: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			InternalServerErrorResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, resp.AppliedOperation, requestID)
	}
}

// GetOperationHistoryHandler 获取操作历史
func GetOperationHistoryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		docID := r.URL.Query().Get("doc_id")

		if docID == "" {
			BadRequestResponse(w, "doc_id is required", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.GetOperationHistory(r.Context(), &pb.GetOperationHistoryRequest{
			DocId: docID,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to get operation history: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			InternalServerErrorResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"operations": resp.Operations,
		}, requestID)
	}
}

// AcquireLockHandler 获取锁
func AcquireLockHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())

		if userID == "" {
			UnauthorizedResponse(w, "Not authenticated", requestID)
			return
		}

		var req struct {
			DocID     string `json:"doc_id"`
			SessionID string `json:"session_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.AcquireLock(r.Context(), &pb.AcquireLockRequest{
			DocId:     req.DocID,
			UserId:    userID,
			SessionId: req.SessionID,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to acquire lock: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			BadRequestResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"lock": resp.Lock,
		}, requestID)
	}
}

// ReleaseLockHandler 释放锁
func ReleaseLockHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())
		userID := middleware.UserIDFromContext(r.Context())

		if userID == "" {
			UnauthorizedResponse(w, "Not authenticated", requestID)
			return
		}

		var req struct {
			DocID string `json:"doc_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			BadRequestResponse(w, "Invalid request body", requestID)
			return
		}

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.ReleaseLock(r.Context(), &pb.ReleaseLockRequest{
			DocId:  req.DocID,
			UserId: userID,
		})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to release lock: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			InternalServerErrorResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, map[string]interface{}{
			"success": resp.Success,
		}, requestID)
	}
}

// GetStatsHandler 获取统计信息
func GetStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())

		// 调用gRPC服务
		resp, err := svcCtx.StateSyncClient.GetStats(r.Context(), &pb.GetStatsRequest{})

		if err != nil {
			InternalServerErrorResponse(w, "Failed to get stats: "+err.Error(), requestID)
			return
		}

		if resp.Error != "" {
			InternalServerErrorResponse(w, resp.Error, requestID)
			return
		}

		// 返回响应
		SuccessResponse(w, resp.Stats, requestID)
	}
}
