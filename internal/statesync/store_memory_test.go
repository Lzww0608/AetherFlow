package statesync

import (
	"context"
	"testing"
	"time"

	guuid "github.com/Lzww0608/GUUID"
)

func TestMemoryStore_CreateDocument(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Test Doc",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		Version:   1,
		Content:   []byte("{}"),
		CreatedBy: "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := store.CreateDocument(ctx, doc)
	if err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}

	// 尝试重复创建
	err = store.CreateDocument(ctx, doc)
	if err != ErrDocumentExists {
		t.Fatalf("Expected ErrDocumentExists, got %v", err)
	}
}

func TestMemoryStore_GetDocument(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Test Doc",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		Version:   1,
		Content:   []byte("{}"),
		CreatedBy: "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = store.CreateDocument(ctx, doc)

	retrieved, err := store.GetDocument(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}

	if retrieved.Name != doc.Name {
		t.Errorf("Expected name %s, got %s", doc.Name, retrieved.Name)
	}

	// 测试不存在的文档
	nonExistentID, _ := guuid.NewV7()
	_, err = store.GetDocument(ctx, nonExistentID)
	if err != ErrDocumentNotFound {
		t.Fatalf("Expected ErrDocumentNotFound, got %v", err)
	}
}

func TestMemoryStore_UpdateDocumentVersion(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Test Doc",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		Version:   1,
		Content:   []byte("{}"),
		CreatedBy: "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = store.CreateDocument(ctx, doc)

	// 正常更新
	newContent := []byte(`{"x": 100}`)
	err := store.UpdateDocumentVersion(ctx, docID, 1, 2, newContent)
	if err != nil {
		t.Fatalf("UpdateDocumentVersion failed: %v", err)
	}

	// 验证更新
	updated, _ := store.GetDocument(ctx, docID)
	if updated.Version != 2 {
		t.Errorf("Expected version 2, got %d", updated.Version)
	}
	if string(updated.Content) != string(newContent) {
		t.Errorf("Content not updated correctly")
	}

	// 版本冲突
	err = store.UpdateDocumentVersion(ctx, docID, 1, 3, newContent)
	if err != ErrVersionMismatch {
		t.Fatalf("Expected ErrVersionMismatch, got %v", err)
	}
}

func TestMemoryStore_CreateOperation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	opID, _ := guuid.NewV7()
	docID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()

	op := &Operation{
		ID:          opID,
		DocID:       docID,
		UserID:      "user1",
		SessionID:   sessionID,
		Type:        OperationTypeUpdate,
		Data:        []byte(`{"action": "move"}`),
		Timestamp:   time.Now(),
		Version:     1,
		PrevVersion: 0,
		Status:      OperationStatusPending,
	}

	err := store.CreateOperation(ctx, op)
	if err != nil {
		t.Fatalf("CreateOperation failed: %v", err)
	}

	// 验证可以获取
	retrieved, err := store.GetOperation(ctx, opID)
	if err != nil {
		t.Fatalf("GetOperation failed: %v", err)
	}

	if retrieved.UserID != "user1" {
		t.Errorf("Expected user1, got %s", retrieved.UserID)
	}
}

func TestMemoryStore_GetOperationsByDocument(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	docID, _ := guuid.NewV7()

	// 创建多个操作
	for i := 0; i < 5; i++ {
		opID, _ := guuid.NewV7()
		sessionID, _ := guuid.NewV7()
		op := &Operation{
			ID:        opID,
			DocID:     docID,
			UserID:    "user1",
			SessionID: sessionID,
			Type:      OperationTypeUpdate,
			Data:      []byte(`{}`),
			Timestamp: time.Now(),
			Version:   uint64(i + 1),
			Status:    OperationStatusApplied,
		}
		_ = store.CreateOperation(ctx, op)
	}

	// 获取操作历史
	ops, err := store.GetOperationsByDocument(ctx, docID, 10)
	if err != nil {
		t.Fatalf("GetOperationsByDocument failed: %v", err)
	}

	if len(ops) != 5 {
		t.Errorf("Expected 5 operations, got %d", len(ops))
	}

	// 测试限制
	ops, err = store.GetOperationsByDocument(ctx, docID, 3)
	if err != nil {
		t.Fatalf("GetOperationsByDocument failed: %v", err)
	}

	if len(ops) != 3 {
		t.Errorf("Expected 3 operations, got %d", len(ops))
	}
}

func TestMemoryStore_AcquireLock(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	lockID, _ := guuid.NewV7()
	docID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()

	lock := &Lock{
		ID:         lockID,
		DocID:      docID,
		UserID:     "user1",
		SessionID:  sessionID,
		AcquiredAt: time.Now(),
		ExpiresAt:  time.Now().Add(30 * time.Second),
		Active:     true,
	}

	err := store.AcquireLock(ctx, lock)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	// 尝试再次获取锁 (应该失败)
	lockID2, _ := guuid.NewV7()
	lock2 := &Lock{
		ID:         lockID2,
		DocID:      docID,
		UserID:     "user2",
		SessionID:  sessionID,
		AcquiredAt: time.Now(),
		ExpiresAt:  time.Now().Add(30 * time.Second),
		Active:     true,
	}

	err = store.AcquireLock(ctx, lock2)
	if err == nil {
		t.Fatal("Expected error when acquiring existing lock")
	}
}

func TestMemoryStore_ReleaseLock(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	lockID, _ := guuid.NewV7()
	docID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()

	lock := &Lock{
		ID:         lockID,
		DocID:      docID,
		UserID:     "user1",
		SessionID:  sessionID,
		AcquiredAt: time.Now(),
		ExpiresAt:  time.Now().Add(30 * time.Second),
		Active:     true,
	}

	_ = store.AcquireLock(ctx, lock)

	// 释放锁
	err := store.ReleaseLock(ctx, docID, "user1")
	if err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	// 验证锁已释放
	locked, _ := store.IsLocked(ctx, docID)
	if locked {
		t.Error("Document should not be locked")
	}

	// 尝试释放不存在的锁
	err = store.ReleaseLock(ctx, docID, "user1")
	if err != ErrLockNotFound {
		t.Fatalf("Expected ErrLockNotFound, got %v", err)
	}
}

func TestMemoryStore_CleanExpiredLocks(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	lockID, _ := guuid.NewV7()
	docID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()

	// 创建已过期的锁
	lock := &Lock{
		ID:         lockID,
		DocID:      docID,
		UserID:     "user1",
		SessionID:  sessionID,
		AcquiredAt: time.Now().Add(-1 * time.Hour),
		ExpiresAt:  time.Now().Add(-30 * time.Second), // 已过期
		Active:     true,
	}

	_ = store.AcquireLock(ctx, lock)

	// 清理过期锁
	count, err := store.CleanExpiredLocks(ctx)
	if err != nil {
		t.Fatalf("CleanExpiredLocks failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 expired lock, got %d", count)
	}

	// 验证锁已清理
	locked, _ := store.IsLocked(ctx, docID)
	if locked {
		t.Error("Expired lock should be cleaned")
	}
}

func TestMemoryStore_GetStats(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 创建一些数据
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Test Doc",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		Version:   1,
		Content:   []byte("{}"),
		CreatedBy: "user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = store.CreateDocument(ctx, doc)

	opID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()
	op := &Operation{
		ID:        opID,
		DocID:     docID,
		UserID:    "user1",
		SessionID: sessionID,
		Type:      OperationTypeUpdate,
		Data:      []byte(`{}`),
		Timestamp: time.Now(),
		Version:   1,
		Status:    OperationStatusApplied,
	}
	_ = store.CreateOperation(ctx, op)

	// 获取统计
	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalDocuments != 1 {
		t.Errorf("Expected 1 document, got %d", stats.TotalDocuments)
	}

	if stats.ActiveDocuments != 1 {
		t.Errorf("Expected 1 active document, got %d", stats.ActiveDocuments)
	}

	if stats.TotalOperations != 1 {
		t.Errorf("Expected 1 operation, got %d", stats.TotalOperations)
	}
}
