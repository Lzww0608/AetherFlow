package statesync

import (
	"context"

	guuid "github.com/Lzww0608/GUUID"
)

// Store 状态同步存储接口
// 定义了文档、操作和冲突的持久化操作
type Store interface {
	// ==================== 文档管理 ====================

	// CreateDocument 创建文档
	CreateDocument(ctx context.Context, doc *Document) error

	// GetDocument 获取文档
	GetDocument(ctx context.Context, docID guuid.UUID) (*Document, error)

	// UpdateDocument 更新文档
	UpdateDocument(ctx context.Context, doc *Document) error

	// DeleteDocument 删除文档 (软删除)
	DeleteDocument(ctx context.Context, docID guuid.UUID) error

	// ListDocuments 列出文档
	// 返回: 文档列表, 总数, 错误
	ListDocuments(ctx context.Context, filter *DocumentFilter) ([]*Document, int, error)

	// GetDocumentsByUser 获取用户有权限的文档列表
	GetDocumentsByUser(ctx context.Context, userID string) ([]*Document, error)

	// UpdateDocumentVersion 更新文档版本 (原子操作)
	UpdateDocumentVersion(ctx context.Context, docID guuid.UUID, oldVersion, newVersion uint64, content []byte) error

	// AddActiveUser 添加活跃用户
	AddActiveUser(ctx context.Context, docID guuid.UUID, userID string) error

	// RemoveActiveUser 移除活跃用户
	RemoveActiveUser(ctx context.Context, docID guuid.UUID, userID string) error

	// ==================== 操作管理 ====================

	// CreateOperation 创建操作
	CreateOperation(ctx context.Context, op *Operation) error

	// GetOperation 获取操作
	GetOperation(ctx context.Context, opID guuid.UUID) (*Operation, error)

	// UpdateOperation 更新操作
	UpdateOperation(ctx context.Context, op *Operation) error

	// ListOperations 列出操作
	// 返回: 操作列表, 总数, 错误
	ListOperations(ctx context.Context, filter *OperationFilter) ([]*Operation, int, error)

	// GetOperationsByDocument 获取文档的操作历史
	GetOperationsByDocument(ctx context.Context, docID guuid.UUID, limit int) ([]*Operation, error)

	// GetOperationsByVersion 获取指定版本范围的操作
	GetOperationsByVersion(ctx context.Context, docID guuid.UUID, minVersion, maxVersion uint64) ([]*Operation, error)

	// GetPendingOperations 获取待处理的操作
	GetPendingOperations(ctx context.Context, docID guuid.UUID) ([]*Operation, error)

	// ==================== 冲突管理 ====================

	// CreateConflict 创建冲突记录
	CreateConflict(ctx context.Context, conflict *Conflict) error

	// GetConflict 获取冲突
	GetConflict(ctx context.Context, conflictID guuid.UUID) (*Conflict, error)

	// UpdateConflict 更新冲突
	UpdateConflict(ctx context.Context, conflict *Conflict) error

	// ListConflicts 列出冲突
	ListConflicts(ctx context.Context, docID guuid.UUID) ([]*Conflict, error)

	// GetUnresolvedConflicts 获取未解决的冲突
	GetUnresolvedConflicts(ctx context.Context, docID guuid.UUID) ([]*Conflict, error)

	// ==================== 锁管理 ====================

	// AcquireLock 获取锁
	AcquireLock(ctx context.Context, lock *Lock) error

	// ReleaseLock 释放锁
	ReleaseLock(ctx context.Context, docID guuid.UUID, userID string) error

	// GetLock 获取锁信息
	GetLock(ctx context.Context, docID guuid.UUID) (*Lock, error)

	// IsLocked 检查文档是否被锁定
	IsLocked(ctx context.Context, docID guuid.UUID) (bool, error)

	// CleanExpiredLocks 清理过期的锁
	CleanExpiredLocks(ctx context.Context) (int, error)

	// ==================== 统计信息 ====================

	// GetStats 获取统计信息
	GetStats(ctx context.Context) (*Stats, error)

	// CountDocuments 统计文档数量
	CountDocuments(ctx context.Context) (int, error)

	// CountOperations 统计操作数量
	CountOperations(ctx context.Context, docID *guuid.UUID) (int, error)

	// CountConflicts 统计冲突数量
	CountConflicts(ctx context.Context, docID *guuid.UUID) (int, error)
}
