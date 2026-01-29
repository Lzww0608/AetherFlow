package statesync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
)

var (
	ErrDocumentNotFound  = errors.New("document not found")
	ErrOperationNotFound = errors.New("operation not found")
	ErrConflictNotFound  = errors.New("conflict not found")
	ErrLockNotFound      = errors.New("lock not found")
	ErrDocumentExists    = errors.New("document already exists")
	ErrVersionMismatch   = errors.New("version mismatch")
	ErrLockExists        = errors.New("lock already exists")
	ErrLockExpired       = errors.New("lock expired")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrInvalidFilter     = errors.New("invalid filter")
)

// MemoryStore 内存存储实现
// 适用于开发和测试环境
type MemoryStore struct {
	mu sync.RWMutex

	// 数据存储
	documents  map[guuid.UUID]*Document
	operations map[guuid.UUID]*Operation
	conflicts  map[guuid.UUID]*Conflict
	locks      map[guuid.UUID]*Lock

	// 索引
	docsByUser     map[string][]guuid.UUID     // userID -> []docID
	opsByDoc       map[guuid.UUID][]guuid.UUID // docID -> []opID
	opsByUser      map[string][]guuid.UUID     // userID -> []opID
	conflictsByDoc map[guuid.UUID][]guuid.UUID // docID -> []conflictID
	locksByDoc     map[guuid.UUID]guuid.UUID   // docID -> lockID
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		documents:      make(map[guuid.UUID]*Document),
		operations:     make(map[guuid.UUID]*Operation),
		conflicts:      make(map[guuid.UUID]*Conflict),
		locks:          make(map[guuid.UUID]*Lock),
		docsByUser:     make(map[string][]guuid.UUID),
		opsByDoc:       make(map[guuid.UUID][]guuid.UUID),
		opsByUser:      make(map[string][]guuid.UUID),
		conflictsByDoc: make(map[guuid.UUID][]guuid.UUID),
		locksByDoc:     make(map[guuid.UUID]guuid.UUID),
	}
}

// ==================== 文档管理 ====================

func (s *MemoryStore) CreateDocument(ctx context.Context, doc *Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.documents[doc.ID]; exists {
		return ErrDocumentExists
	}

	// 创建文档副本
	docCopy := *doc
	s.documents[doc.ID] = &docCopy

	// 更新索引 - 添加创建者
	s.addDocToUser(doc.CreatedBy, doc.ID)

	// 添加所有有权限的用户
	if doc.Metadata.Permissions.Owner != "" {
		s.addDocToUser(doc.Metadata.Permissions.Owner, doc.ID)
	}
	for _, editor := range doc.Metadata.Permissions.Editors {
		s.addDocToUser(editor, doc.ID)
	}
	for _, viewer := range doc.Metadata.Permissions.Viewers {
		s.addDocToUser(viewer, doc.ID)
	}

	return nil
}

func (s *MemoryStore) GetDocument(ctx context.Context, docID guuid.UUID) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, exists := s.documents[docID]
	if !exists {
		return nil, ErrDocumentNotFound
	}

	// 返回副本
	docCopy := *doc
	return &docCopy, nil
}

func (s *MemoryStore) UpdateDocument(ctx context.Context, doc *Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.documents[doc.ID]; !exists {
		return ErrDocumentNotFound
	}

	doc.UpdatedAt = time.Now()
	docCopy := *doc
	s.documents[doc.ID] = &docCopy

	return nil
}

func (s *MemoryStore) DeleteDocument(ctx context.Context, docID guuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, exists := s.documents[docID]
	if !exists {
		return ErrDocumentNotFound
	}

	// 软删除
	doc.State = DocumentStateDeleted
	doc.UpdatedAt = time.Now()

	return nil
}

func (s *MemoryStore) ListDocuments(ctx context.Context, filter *DocumentFilter) ([]*Document, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Document

	for _, doc := range s.documents {
		if filter != nil {
			// 应用过滤条件
			if filter.Type != nil && doc.Type != *filter.Type {
				continue
			}
			if filter.State != nil && doc.State != *filter.State {
				continue
			}
			if filter.CreatedBy != nil && doc.CreatedBy != *filter.CreatedBy {
				continue
			}
			if filter.UserID != nil {
				// 检查用户是否有权限
				if !s.hasPermission(*filter.UserID, doc) {
					continue
				}
			}
		}

		// 创建副本
		docCopy := *doc
		result = append(result, &docCopy)
	}

	total := len(result)

	// 应用分页
	if filter != nil {
		start := filter.Offset
		end := filter.Offset + filter.Limit

		if start > total {
			return []*Document{}, total, nil
		}
		if end > total {
			end = total
		}
		if filter.Limit > 0 {
			result = result[start:end]
		}
	}

	return result, total, nil
}

func (s *MemoryStore) GetDocumentsByUser(ctx context.Context, userID string) ([]*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docIDs, exists := s.docsByUser[userID]
	if !exists {
		return []*Document{}, nil
	}

	result := make([]*Document, 0, len(docIDs))
	for _, docID := range docIDs {
		if doc, exists := s.documents[docID]; exists {
			docCopy := *doc
			result = append(result, &docCopy)
		}
	}

	return result, nil
}

func (s *MemoryStore) UpdateDocumentVersion(ctx context.Context, docID guuid.UUID, oldVersion, newVersion uint64, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, exists := s.documents[docID]
	if !exists {
		return ErrDocumentNotFound
	}

	// 版本检查 (乐观锁)
	if doc.Version != oldVersion {
		return ErrVersionMismatch
	}

	doc.Version = newVersion
	doc.Content = content
	doc.UpdatedAt = time.Now()

	return nil
}

func (s *MemoryStore) AddActiveUser(ctx context.Context, docID guuid.UUID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, exists := s.documents[docID]
	if !exists {
		return ErrDocumentNotFound
	}

	// 检查是否已存在
	for _, uid := range doc.ActiveUsers {
		if uid == userID {
			return nil
		}
	}

	doc.ActiveUsers = append(doc.ActiveUsers, userID)
	return nil
}

func (s *MemoryStore) RemoveActiveUser(ctx context.Context, docID guuid.UUID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, exists := s.documents[docID]
	if !exists {
		return ErrDocumentNotFound
	}

	// 移除用户
	for i, uid := range doc.ActiveUsers {
		if uid == userID {
			doc.ActiveUsers = append(doc.ActiveUsers[:i], doc.ActiveUsers[i+1:]...)
			return nil
		}
	}

	return nil
}

// ==================== 操作管理 ====================

func (s *MemoryStore) CreateOperation(ctx context.Context, op *Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建操作副本
	opCopy := *op
	s.operations[op.ID] = &opCopy

	// 更新索引
	s.opsByDoc[op.DocID] = append(s.opsByDoc[op.DocID], op.ID)
	s.opsByUser[op.UserID] = append(s.opsByUser[op.UserID], op.ID)

	return nil
}

func (s *MemoryStore) GetOperation(ctx context.Context, opID guuid.UUID) (*Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, exists := s.operations[opID]
	if !exists {
		return nil, ErrOperationNotFound
	}

	opCopy := *op
	return &opCopy, nil
}

func (s *MemoryStore) UpdateOperation(ctx context.Context, op *Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.operations[op.ID]; !exists {
		return ErrOperationNotFound
	}

	opCopy := *op
	s.operations[op.ID] = &opCopy

	return nil
}

func (s *MemoryStore) ListOperations(ctx context.Context, filter *OperationFilter) ([]*Operation, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Operation

	for _, op := range s.operations {
		if filter != nil {
			// 应用过滤条件
			if filter.DocID != nil && op.DocID != *filter.DocID {
				continue
			}
			if filter.UserID != nil && op.UserID != *filter.UserID {
				continue
			}
			if filter.Type != nil && op.Type != *filter.Type {
				continue
			}
			if filter.Status != nil && op.Status != *filter.Status {
				continue
			}
			if filter.FromTime != nil && op.Timestamp.Before(*filter.FromTime) {
				continue
			}
			if filter.ToTime != nil && op.Timestamp.After(*filter.ToTime) {
				continue
			}
			if filter.MinVersion != nil && op.Version < *filter.MinVersion {
				continue
			}
			if filter.MaxVersion != nil && op.Version > *filter.MaxVersion {
				continue
			}
		}

		opCopy := *op
		result = append(result, &opCopy)
	}

	total := len(result)

	// 应用分页
	if filter != nil && filter.Limit > 0 {
		start := filter.Offset
		end := filter.Offset + filter.Limit

		if start > total {
			return []*Operation{}, total, nil
		}
		if end > total {
			end = total
		}
		result = result[start:end]
	}

	return result, total, nil
}

func (s *MemoryStore) GetOperationsByDocument(ctx context.Context, docID guuid.UUID, limit int) ([]*Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opIDs, exists := s.opsByDoc[docID]
	if !exists {
		return []*Operation{}, nil
	}

	result := make([]*Operation, 0, len(opIDs))
	for _, opID := range opIDs {
		if op, exists := s.operations[opID]; exists {
			opCopy := *op
			result = append(result, &opCopy)
		}
	}

	// 应用限制
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}

	return result, nil
}

func (s *MemoryStore) GetOperationsByVersion(ctx context.Context, docID guuid.UUID, minVersion, maxVersion uint64) ([]*Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opIDs, exists := s.opsByDoc[docID]
	if !exists {
		return []*Operation{}, nil
	}

	result := make([]*Operation, 0)
	for _, opID := range opIDs {
		if op, exists := s.operations[opID]; exists {
			if op.Version >= minVersion && op.Version <= maxVersion {
				opCopy := *op
				result = append(result, &opCopy)
			}
		}
	}

	return result, nil
}

func (s *MemoryStore) GetPendingOperations(ctx context.Context, docID guuid.UUID) ([]*Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opIDs, exists := s.opsByDoc[docID]
	if !exists {
		return []*Operation{}, nil
	}

	result := make([]*Operation, 0)
	for _, opID := range opIDs {
		if op, exists := s.operations[opID]; exists {
			if op.Status == OperationStatusPending {
				opCopy := *op
				result = append(result, &opCopy)
			}
		}
	}

	return result, nil
}

// ==================== 冲突管理 ====================

func (s *MemoryStore) CreateConflict(ctx context.Context, conflict *Conflict) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conflictCopy := *conflict
	s.conflicts[conflict.ID] = &conflictCopy

	// 更新索引
	s.conflictsByDoc[conflict.DocID] = append(s.conflictsByDoc[conflict.DocID], conflict.ID)

	return nil
}

func (s *MemoryStore) GetConflict(ctx context.Context, conflictID guuid.UUID) (*Conflict, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conflict, exists := s.conflicts[conflictID]
	if !exists {
		return nil, ErrConflictNotFound
	}

	conflictCopy := *conflict
	return &conflictCopy, nil
}

func (s *MemoryStore) UpdateConflict(ctx context.Context, conflict *Conflict) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.conflicts[conflict.ID]; !exists {
		return ErrConflictNotFound
	}

	conflictCopy := *conflict
	s.conflicts[conflict.ID] = &conflictCopy

	return nil
}

func (s *MemoryStore) ListConflicts(ctx context.Context, docID guuid.UUID) ([]*Conflict, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conflictIDs, exists := s.conflictsByDoc[docID]
	if !exists {
		return []*Conflict{}, nil
	}

	result := make([]*Conflict, 0, len(conflictIDs))
	for _, conflictID := range conflictIDs {
		if conflict, exists := s.conflicts[conflictID]; exists {
			conflictCopy := *conflict
			result = append(result, &conflictCopy)
		}
	}

	return result, nil
}

func (s *MemoryStore) GetUnresolvedConflicts(ctx context.Context, docID guuid.UUID) ([]*Conflict, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conflictIDs, exists := s.conflictsByDoc[docID]
	if !exists {
		return []*Conflict{}, nil
	}

	result := make([]*Conflict, 0)
	for _, conflictID := range conflictIDs {
		if conflict, exists := s.conflicts[conflictID]; exists {
			if conflict.ResolvedAt.IsZero() {
				conflictCopy := *conflict
				result = append(result, &conflictCopy)
			}
		}
	}

	return result, nil
}

// ==================== 锁管理 ====================

func (s *MemoryStore) AcquireLock(ctx context.Context, lock *Lock) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有锁
	if lockID, exists := s.locksByDoc[lock.DocID]; exists {
		existingLock := s.locks[lockID]
		// 检查锁是否过期
		if existingLock.ExpiresAt.After(time.Now()) {
			return fmt.Errorf("%w: locked by user %s", ErrLockExists, existingLock.UserID)
		}
		// 锁已过期，删除旧锁
		delete(s.locks, lockID)
		delete(s.locksByDoc, lock.DocID)
	}

	// 创建新锁
	lockCopy := *lock
	lockCopy.Active = true
	lockCopy.AcquiredAt = time.Now()
	s.locks[lock.ID] = &lockCopy
	s.locksByDoc[lock.DocID] = lock.ID

	return nil
}

func (s *MemoryStore) ReleaseLock(ctx context.Context, docID guuid.UUID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockID, exists := s.locksByDoc[docID]
	if !exists {
		return ErrLockNotFound
	}

	lock := s.locks[lockID]

	// 检查是否是锁的持有者
	if lock.UserID != userID {
		return ErrPermissionDenied
	}

	// 删除锁
	delete(s.locks, lockID)
	delete(s.locksByDoc, docID)

	return nil
}

func (s *MemoryStore) GetLock(ctx context.Context, docID guuid.UUID) (*Lock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lockID, exists := s.locksByDoc[docID]
	if !exists {
		return nil, ErrLockNotFound
	}

	lock := s.locks[lockID]

	// 检查锁是否过期
	if lock.ExpiresAt.Before(time.Now()) {
		return nil, ErrLockExpired
	}

	lockCopy := *lock
	return &lockCopy, nil
}

func (s *MemoryStore) IsLocked(ctx context.Context, docID guuid.UUID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lockID, exists := s.locksByDoc[docID]
	if !exists {
		return false, nil
	}

	lock := s.locks[lockID]

	// 检查锁是否过期
	if lock.ExpiresAt.Before(time.Now()) {
		return false, nil
	}

	return true, nil
}

func (s *MemoryStore) CleanExpiredLocks(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()

	// 找出所有过期的锁
	expiredLockIDs := make([]guuid.UUID, 0)
	for lockID, lock := range s.locks {
		if lock.ExpiresAt.Before(now) {
			expiredLockIDs = append(expiredLockIDs, lockID)
		}
	}

	// 删除过期锁
	for _, lockID := range expiredLockIDs {
		lock := s.locks[lockID]
		delete(s.locks, lockID)
		delete(s.locksByDoc, lock.DocID)
		count++
	}

	return count, nil
}

// ==================== 统计信息 ====================

func (s *MemoryStore) GetStats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		TotalDocuments:  int64(len(s.documents)),
		TotalOperations: int64(len(s.operations)),
		TotalConflicts:  int64(len(s.conflicts)),
		ActiveLocks:     int64(len(s.locks)),
		LastUpdated:     time.Now(),
	}

	// 统计活跃文档数
	for _, doc := range s.documents {
		if doc.State == DocumentStateActive {
			stats.ActiveDocuments++
		} else if doc.State == DocumentStateArchived {
			stats.ArchivedDocuments++
		}
	}

	// 统计已解决的冲突
	for _, conflict := range s.conflicts {
		if !conflict.ResolvedAt.IsZero() {
			stats.ResolvedConflicts++
		}
	}

	return stats, nil
}

func (s *MemoryStore) CountDocuments(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.documents), nil
}

func (s *MemoryStore) CountOperations(ctx context.Context, docID *guuid.UUID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if docID == nil {
		return len(s.operations), nil
	}

	if opIDs, exists := s.opsByDoc[*docID]; exists {
		return len(opIDs), nil
	}

	return 0, nil
}

func (s *MemoryStore) CountConflicts(ctx context.Context, docID *guuid.UUID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if docID == nil {
		return len(s.conflicts), nil
	}

	if conflictIDs, exists := s.conflictsByDoc[*docID]; exists {
		return len(conflictIDs), nil
	}

	return 0, nil
}

// ==================== 辅助方法 ====================

func (s *MemoryStore) addDocToUser(userID string, docID guuid.UUID) {
	// 检查是否已存在
	for _, id := range s.docsByUser[userID] {
		if id == docID {
			return
		}
	}
	s.docsByUser[userID] = append(s.docsByUser[userID], docID)
}

func (s *MemoryStore) hasPermission(userID string, doc *Document) bool {
	// 检查是否是拥有者或创建者
	if doc.CreatedBy == userID || doc.Metadata.Permissions.Owner == userID {
		return true
	}

	// 检查是否在编辑者列表中
	for _, editor := range doc.Metadata.Permissions.Editors {
		if editor == userID {
			return true
		}
	}

	// 检查是否在查看者列表中
	for _, viewer := range doc.Metadata.Permissions.Viewers {
		if viewer == userID {
			return true
		}
	}

	// 检查是否公开
	if doc.Metadata.Permissions.Public {
		return true
	}

	return false
}
