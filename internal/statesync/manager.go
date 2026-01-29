package statesync

import (
	"context"
	"fmt"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"go.uber.org/zap"
)

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// 存储实现
	Store Store

	// 广播器
	Broadcaster Broadcaster

	// 冲突解决器
	ConflictResolver ConflictResolver

	// 日志
	Logger *zap.Logger

	// 锁超时时间
	LockTimeout time.Duration

	// 清理间隔
	CleanupInterval time.Duration

	// 是否启用自动冲突解决
	AutoResolveConflicts bool
}

// Manager 状态同步管理器
type Manager struct {
	store            Store
	broadcaster      Broadcaster
	conflictResolver ConflictResolver
	conflictDetector *ConflictDetector
	logger           *zap.Logger

	// 配置
	lockTimeout          time.Duration
	cleanupInterval      time.Duration
	autoResolveConflicts bool

	// 状态
	mu     sync.RWMutex
	closed bool

	// 后台任务
	cleanupTicker *time.Ticker
	cleanupStop   chan struct{}
}

// NewManager 创建状态同步管理器
func NewManager(config *ManagerConfig) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.Store == nil {
		return nil, fmt.Errorf("store is required")
	}

	// 设置默认值
	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}

	if config.Broadcaster == nil {
		config.Broadcaster = NewMemoryBroadcaster(config.Logger)
	}

	if config.ConflictResolver == nil {
		config.ConflictResolver = NewLWWConflictResolver(config.Logger)
	}

	if config.LockTimeout == 0 {
		config.LockTimeout = 30 * time.Second
	}

	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	m := &Manager{
		store:                config.Store,
		broadcaster:          config.Broadcaster,
		conflictResolver:     config.ConflictResolver,
		conflictDetector:     NewConflictDetector(config.Logger),
		logger:               config.Logger,
		lockTimeout:          config.LockTimeout,
		cleanupInterval:      config.CleanupInterval,
		autoResolveConflicts: config.AutoResolveConflicts,
		closed:               false,
		cleanupStop:          make(chan struct{}),
	}

	// 启动后台清理任务
	m.startCleanupTask()

	return m, nil
}

// ==================== 文档管理 ====================

// CreateDocument 创建文档
func (m *Manager) CreateDocument(ctx context.Context, name string, docType DocumentType, createdBy string, content []byte) (*Document, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("manager is closed")
	}
	m.mu.RUnlock()

	docID, err := guuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate document ID: %w", err)
	}

	now := time.Now()
	doc := &Document{
		ID:          docID,
		Name:        name,
		Type:        docType,
		State:       DocumentStateActive,
		Version:     1,
		Content:     content,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   createdBy,
		ActiveUsers: []string{},
		Metadata: Metadata{
			Tags:       []string{},
			Properties: make(map[string]string),
			Permissions: Permissions{
				Owner:   createdBy,
				Editors: []string{},
				Viewers: []string{},
				Public:  false,
			},
		},
	}

	if err := m.store.CreateDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	m.logger.Info("Document created",
		zap.String("doc_id", doc.ID.String()),
		zap.String("name", name),
		zap.String("type", string(docType)),
		zap.String("created_by", createdBy),
	)

	// 广播文档创建事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeDocumentUpdated,
		DocID:     doc.ID,
		UserID:    createdBy,
		Document:  doc,
		Timestamp: time.Now(),
	}
	_ = m.broadcaster.Broadcast(ctx, event)

	return doc, nil
}

// GetDocument 获取文档
func (m *Manager) GetDocument(ctx context.Context, docID guuid.UUID) (*Document, error) {
	return m.store.GetDocument(ctx, docID)
}

// UpdateDocument 更新文档
func (m *Manager) UpdateDocument(ctx context.Context, doc *Document) error {
	if err := m.store.UpdateDocument(ctx, doc); err != nil {
		return err
	}

	m.logger.Info("Document updated",
		zap.String("doc_id", doc.ID.String()),
		zap.String("updated_by", doc.UpdatedBy),
	)

	// 广播文档更新事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeDocumentUpdated,
		DocID:     doc.ID,
		UserID:    doc.UpdatedBy,
		Document:  doc,
		Timestamp: time.Now(),
	}
	_ = m.broadcaster.BroadcastToDocument(ctx, doc.ID, event)

	return nil
}

// DeleteDocument 删除文档
func (m *Manager) DeleteDocument(ctx context.Context, docID guuid.UUID, userID string) error {
	if err := m.store.DeleteDocument(ctx, docID); err != nil {
		return err
	}

	m.logger.Info("Document deleted",
		zap.String("doc_id", docID.String()),
		zap.String("deleted_by", userID),
	)

	return nil
}

// ListDocuments 列出文档
func (m *Manager) ListDocuments(ctx context.Context, filter *DocumentFilter) ([]*Document, int, error) {
	return m.store.ListDocuments(ctx, filter)
}

// ==================== 操作管理 ====================

// ApplyOperation 应用操作
func (m *Manager) ApplyOperation(ctx context.Context, op *Operation) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return fmt.Errorf("manager is closed")
	}
	m.mu.RUnlock()

	// 1. 获取文档
	doc, err := m.store.GetDocument(ctx, op.DocID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// 2. 检查版本冲突
	if op.PrevVersion != doc.Version {
		// 版本不匹配，可能有冲突
		pendingOps, err := m.store.GetPendingOperations(ctx, op.DocID)
		if err != nil {
			return fmt.Errorf("failed to get pending operations: %w", err)
		}

		// 添加当前操作
		pendingOps = append(pendingOps, op)

		// 检测冲突
		conflicts, err := m.conflictDetector.DetectConflicts(pendingOps, doc)
		if err != nil {
			return fmt.Errorf("failed to detect conflicts: %w", err)
		}

		if len(conflicts) > 0 {
			// 有冲突
			conflict := conflicts[0]

			// 保存冲突记录
			if err := m.store.CreateConflict(ctx, conflict); err != nil {
				m.logger.Error("Failed to save conflict",
					zap.Error(err),
					zap.String("conflict_id", conflict.ID.String()),
				)
			}

			// 广播冲突事件
			eventID, _ := guuid.NewV7()
			conflictEvent := &Event{
				ID:        eventID,
				Type:      EventTypeConflictDetected,
				DocID:     op.DocID,
				UserID:    op.UserID,
				Conflict:  conflict,
				Timestamp: time.Now(),
			}
			_ = m.broadcaster.BroadcastToDocument(ctx, op.DocID, conflictEvent)

			// 如果启用自动解决
			if m.autoResolveConflicts {
				if err := m.resolveConflict(ctx, conflict, op.UserID); err != nil {
					m.logger.Error("Failed to auto-resolve conflict",
						zap.Error(err),
						zap.String("conflict_id", conflict.ID.String()),
					)
					op.Status = OperationStatusConflict
				} else {
					op.Status = OperationStatusResolved
				}
			} else {
				op.Status = OperationStatusConflict
			}

			// 保存操作记录
			return m.store.CreateOperation(ctx, op)
		}
	}

	// 3. 应用操作
	newVersion := doc.Version + 1
	op.Version = newVersion
	op.Status = OperationStatusApplied
	op.Timestamp = time.Now()

	// 更新文档版本和内容
	if err := m.store.UpdateDocumentVersion(ctx, op.DocID, doc.Version, newVersion, op.Data); err != nil {
		if err == ErrVersionMismatch {
			// 版本冲突，标记为待处理
			op.Status = OperationStatusPending
			return m.store.CreateOperation(ctx, op)
		}
		return fmt.Errorf("failed to update document version: %w", err)
	}

	// 4. 保存操作记录
	if err := m.store.CreateOperation(ctx, op); err != nil {
		return fmt.Errorf("failed to save operation: %w", err)
	}

	m.logger.Debug("Operation applied",
		zap.String("op_id", op.ID.String()),
		zap.String("doc_id", op.DocID.String()),
		zap.String("user_id", op.UserID),
		zap.String("type", string(op.Type)),
		zap.Uint64("version", newVersion),
	)

	// 5. 广播操作已应用事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeOperationApplied,
		DocID:     op.DocID,
		UserID:    op.UserID,
		Operation: op,
		Timestamp: time.Now(),
	}

	return m.broadcaster.BroadcastToDocument(ctx, op.DocID, event)
}

// GetOperationHistory 获取操作历史
func (m *Manager) GetOperationHistory(ctx context.Context, docID guuid.UUID, limit int) ([]*Operation, error) {
	return m.store.GetOperationsByDocument(ctx, docID, limit)
}

// ==================== 订阅管理 ====================

// Subscribe 订阅文档
func (m *Manager) Subscribe(ctx context.Context, docID guuid.UUID, userID string, sessionID guuid.UUID) (*Subscriber, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("manager is closed")
	}
	m.mu.RUnlock()

	// 添加活跃用户
	if err := m.store.AddActiveUser(ctx, docID, userID); err != nil {
		return nil, fmt.Errorf("failed to add active user: %w", err)
	}

	// 订阅文档变更
	subscriber, err := m.broadcaster.Subscribe(ctx, docID, userID, sessionID)
	if err != nil {
		_ = m.store.RemoveActiveUser(ctx, docID, userID)
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// 广播用户加入事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeUserJoined,
		DocID:     docID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"session_id": sessionID.String()},
	}
	_ = m.broadcaster.BroadcastToDocument(ctx, docID, event)

	m.logger.Info("User subscribed to document",
		zap.String("doc_id", docID.String()),
		zap.String("user_id", userID),
		zap.String("subscriber_id", subscriber.ID),
	)

	return subscriber, nil
}

// Unsubscribe 取消订阅
func (m *Manager) Unsubscribe(ctx context.Context, subscriberID string, docID guuid.UUID, userID string) error {
	// 移除活跃用户
	if err := m.store.RemoveActiveUser(ctx, docID, userID); err != nil {
		m.logger.Warn("Failed to remove active user",
			zap.Error(err),
			zap.String("user_id", userID),
		)
	}

	// 取消订阅
	if err := m.broadcaster.Unsubscribe(subscriberID); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	// 广播用户离开事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeUserLeft,
		DocID:     docID,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	_ = m.broadcaster.BroadcastToDocument(ctx, docID, event)

	m.logger.Info("User unsubscribed from document",
		zap.String("doc_id", docID.String()),
		zap.String("user_id", userID),
		zap.String("subscriber_id", subscriberID),
	)

	return nil
}

// GetSubscribers 获取订阅者列表
func (m *Manager) GetSubscribers(docID guuid.UUID) []*Subscriber {
	return m.broadcaster.GetSubscribers(docID)
}

// ==================== 锁管理 ====================

// AcquireLock 获取锁
func (m *Manager) AcquireLock(ctx context.Context, docID guuid.UUID, userID string, sessionID guuid.UUID) (*Lock, error) {
	lockID, err := guuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate lock ID: %w", err)
	}

	now := time.Now()
	lock := &Lock{
		ID:         lockID,
		DocID:      docID,
		UserID:     userID,
		SessionID:  sessionID,
		AcquiredAt: now,
		ExpiresAt:  now.Add(m.lockTimeout),
		Active:     true,
	}

	if err := m.store.AcquireLock(ctx, lock); err != nil {
		return nil, err
	}

	m.logger.Info("Lock acquired",
		zap.String("lock_id", lock.ID.String()),
		zap.String("doc_id", docID.String()),
		zap.String("user_id", userID),
	)

	// 广播锁获取事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeLockAcquired,
		DocID:     docID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"lock_id": lock.ID.String()},
	}
	_ = m.broadcaster.BroadcastToDocument(ctx, docID, event)

	return lock, nil
}

// ReleaseLock 释放锁
func (m *Manager) ReleaseLock(ctx context.Context, docID guuid.UUID, userID string) error {
	if err := m.store.ReleaseLock(ctx, docID, userID); err != nil {
		return err
	}

	m.logger.Info("Lock released",
		zap.String("doc_id", docID.String()),
		zap.String("user_id", userID),
	)

	// 广播锁释放事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeLockReleased,
		DocID:     docID,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	_ = m.broadcaster.BroadcastToDocument(ctx, docID, event)

	return nil
}

// IsLocked 检查是否锁定
func (m *Manager) IsLocked(ctx context.Context, docID guuid.UUID) (bool, error) {
	return m.store.IsLocked(ctx, docID)
}

// ==================== 统计信息 ====================

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context) (*Stats, error) {
	stats, err := m.store.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// 添加订阅者统计
	// 遍历所有文档统计订阅者
	// (这里简化实现，实际可以在broadcaster中维护总数)
	stats.ActiveSubscribers = 0

	return stats, nil
}

// ==================== 私有方法 ====================

// resolveConflict 解决冲突
func (m *Manager) resolveConflict(ctx context.Context, conflict *Conflict, userID string) error {
	if err := ResolveConflict(ctx, conflict, m.conflictResolver, userID); err != nil {
		return err
	}

	// 更新冲突记录
	if err := m.store.UpdateConflict(ctx, conflict); err != nil {
		return fmt.Errorf("failed to update conflict: %w", err)
	}

	// 广播冲突已解决事件
	eventID, _ := guuid.NewV7()
	event := &Event{
		ID:        eventID,
		Type:      EventTypeConflictResolved,
		DocID:     conflict.DocID,
		UserID:    userID,
		Conflict:  conflict,
		Timestamp: time.Now(),
	}
	_ = m.broadcaster.BroadcastToDocument(ctx, conflict.DocID, event)

	m.logger.Info("Conflict resolved",
		zap.String("conflict_id", conflict.ID.String()),
		zap.String("doc_id", conflict.DocID.String()),
		zap.String("resolution", string(conflict.Resolution)),
	)

	return nil
}

// startCleanupTask 启动清理任务
func (m *Manager) startCleanupTask() {
	m.cleanupTicker = time.NewTicker(m.cleanupInterval)

	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				m.cleanup()
			case <-m.cleanupStop:
				return
			}
		}
	}()

	m.logger.Info("Cleanup task started",
		zap.Duration("interval", m.cleanupInterval),
	)
}

// cleanup 清理过期数据
func (m *Manager) cleanup() {
	ctx := context.Background()

	// 清理过期的锁
	count, err := m.store.CleanExpiredLocks(ctx)
	if err != nil {
		m.logger.Error("Failed to clean expired locks", zap.Error(err))
	} else if count > 0 {
		m.logger.Info("Cleaned expired locks", zap.Int("count", count))
	}

	// 清理不活跃的订阅者
	if memBroadcaster, ok := m.broadcaster.(*MemoryBroadcaster); ok {
		count := memBroadcaster.CleanInactiveSubscribers()
		if count > 0 {
			m.logger.Info("Cleaned inactive subscribers", zap.Int("count", count))
		}
	}
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	// 停止清理任务
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}
	close(m.cleanupStop)

	// 关闭广播器
	if err := m.broadcaster.Close(); err != nil {
		m.logger.Error("Failed to close broadcaster", zap.Error(err))
	}

	m.logger.Info("Manager closed")

	return nil
}
