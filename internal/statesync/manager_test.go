package statesync

import (
	"context"
	"testing"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"go.uber.org/zap"
)

func createTestManager(t *testing.T) *Manager {
	logger := zap.NewNop()
	store := NewMemoryStore()
	broadcaster := NewMemoryBroadcaster(logger)
	resolver := NewLWWConflictResolver(logger)

	config := &ManagerConfig{
		Store:                store,
		Broadcaster:          broadcaster,
		ConflictResolver:     resolver,
		Logger:               logger,
		LockTimeout:          30 * time.Second,
		CleanupInterval:      5 * time.Minute,
		AutoResolveConflicts: true,
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	return manager
}

func TestManager_CreateDocument(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	doc, err := manager.CreateDocument(
		ctx,
		"Test Whiteboard",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	if err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}

	if doc.Name != "Test Whiteboard" {
		t.Errorf("Expected name 'Test Whiteboard', got '%s'", doc.Name)
	}

	if doc.Version != 1 {
		t.Errorf("Expected version 1, got %d", doc.Version)
	}

	if doc.CreatedBy != "user1" {
		t.Errorf("Expected created_by 'user1', got '%s'", doc.CreatedBy)
	}
}

func TestManager_GetDocument(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeText,
		"user1",
		[]byte("Hello"),
	)

	// 获取文档
	retrieved, err := manager.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}

	if retrieved.ID != doc.ID {
		t.Error("Document ID mismatch")
	}

	if string(retrieved.Content) != "Hello" {
		t.Errorf("Expected content 'Hello', got '%s'", string(retrieved.Content))
	}
}

func TestManager_ApplyOperation(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	// 应用操作
	opID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()

	operation := &Operation{
		ID:          opID,
		DocID:       doc.ID,
		UserID:      "user1",
		SessionID:   sessionID,
		Type:        OperationTypeUpdate,
		Data:        []byte(`{"x": 100, "y": 200}`),
		PrevVersion: doc.Version,
		Status:      OperationStatusPending,
	}

	err := manager.ApplyOperation(ctx, operation)
	if err != nil {
		t.Fatalf("ApplyOperation failed: %v", err)
	}

	// 验证操作已应用
	updatedDoc, _ := manager.GetDocument(ctx, doc.ID)
	if updatedDoc.Version != 2 {
		t.Errorf("Expected version 2, got %d", updatedDoc.Version)
	}

	if string(updatedDoc.Content) != string(operation.Data) {
		t.Error("Content not updated correctly")
	}
}

func TestManager_ApplyOperation_Conflict(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	sessionID1, _ := guuid.NewV7()
	sessionID2, _ := guuid.NewV7()

	// 第一个操作 (正常应用)
	opID1, _ := guuid.NewV7()
	op1 := &Operation{
		ID:          opID1,
		DocID:       doc.ID,
		UserID:      "user1",
		SessionID:   sessionID1,
		Type:        OperationTypeUpdate,
		Data:        []byte(`{"x": 100}`),
		PrevVersion: doc.Version,
		Status:      OperationStatusPending,
	}
	_ = manager.ApplyOperation(ctx, op1)

	// 第二个操作 (基于旧版本，应该冲突)
	opID2, _ := guuid.NewV7()
	op2 := &Operation{
		ID:          opID2,
		DocID:       doc.ID,
		UserID:      "user2",
		SessionID:   sessionID2,
		Type:        OperationTypeUpdate,
		Data:        []byte(`{"x": 200}`),
		PrevVersion: doc.Version, // 使用旧版本
		Status:      OperationStatusPending,
	}

	err := manager.ApplyOperation(ctx, op2)
	// 不应该返回错误，但操作应该被标记为冲突或已解决
	if err != nil {
		t.Logf("ApplyOperation returned error (expected for conflict): %v", err)
	}
}

func TestManager_Subscribe(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	// 订阅文档
	sessionID, _ := guuid.NewV7()
	subscriber, err := manager.Subscribe(ctx, doc.ID, "user1", sessionID)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if subscriber.UserID != "user1" {
		t.Errorf("Expected user1, got %s", subscriber.UserID)
	}

	if subscriber.DocID != doc.ID {
		t.Error("DocID mismatch")
	}

	// 取消订阅
	err = manager.Unsubscribe(ctx, subscriber.ID, doc.ID, "user1")
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
}

func TestManager_Subscribe_ReceiveEvents(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	// 订阅文档
	sessionID, _ := guuid.NewV7()
	subscriber, _ := manager.Subscribe(ctx, doc.ID, "user1", sessionID)

	// 先消费掉欢迎事件 (EventTypeUserJoined)
	select {
	case <-subscriber.Channel:
		// 消费欢迎事件
	case <-time.After(100 * time.Millisecond):
		t.Log("No welcome event received (ok)")
	}

	// 启动goroutine接收操作事件
	eventReceived := make(chan bool, 1)
	go func() {
		timeout := time.NewTimer(2 * time.Second)
		defer timeout.Stop()

		for {
			select {
			case event := <-subscriber.Channel:
				if event.Type == EventTypeOperationApplied {
					eventReceived <- true
					return
				}
				// 忽略其他事件，继续等待
			case <-timeout.C:
				eventReceived <- false
				return
			}
		}
	}()

	// 应用操作 (应该触发事件)
	opID, _ := guuid.NewV7()
	operation := &Operation{
		ID:          opID,
		DocID:       doc.ID,
		UserID:      "user1",
		SessionID:   sessionID,
		Type:        OperationTypeUpdate,
		Data:        []byte(`{"x": 100}`),
		PrevVersion: doc.Version,
		Status:      OperationStatusPending,
	}
	_ = manager.ApplyOperation(ctx, operation)

	// 等待事件
	received := <-eventReceived
	if !received {
		t.Error("Did not receive operation event")
	}
}

func TestManager_AcquireLock(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	sessionID, _ := guuid.NewV7()

	// 获取锁
	lock, err := manager.AcquireLock(ctx, doc.ID, "user1", sessionID)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	if lock.UserID != "user1" {
		t.Errorf("Expected user1, got %s", lock.UserID)
	}

	// 尝试再次获取锁 (应该失败)
	sessionID2, _ := guuid.NewV7()
	_, err = manager.AcquireLock(ctx, doc.ID, "user2", sessionID2)
	if err == nil {
		t.Error("Expected error when acquiring existing lock")
	}

	// 释放锁
	err = manager.ReleaseLock(ctx, doc.ID, "user1")
	if err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	// 验证可以再次获取
	_, err = manager.AcquireLock(ctx, doc.ID, "user2", sessionID2)
	if err != nil {
		t.Errorf("Should be able to acquire lock after release: %v", err)
	}
}

func TestManager_GetStats(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建一些数据
	_, _ = manager.CreateDocument(ctx, "Doc1", DocumentTypeWhiteboard, "user1", []byte("{}"))
	_, _ = manager.CreateDocument(ctx, "Doc2", DocumentTypeText, "user1", []byte("{}"))

	// 获取统计
	stats, err := manager.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalDocuments != 2 {
		t.Errorf("Expected 2 documents, got %d", stats.TotalDocuments)
	}

	if stats.ActiveDocuments != 2 {
		t.Errorf("Expected 2 active documents, got %d", stats.ActiveDocuments)
	}
}

func TestManager_ListDocuments(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建多个文档
	_, _ = manager.CreateDocument(ctx, "Doc1", DocumentTypeWhiteboard, "user1", []byte("{}"))
	_, _ = manager.CreateDocument(ctx, "Doc2", DocumentTypeText, "user1", []byte("{}"))
	_, _ = manager.CreateDocument(ctx, "Doc3", DocumentTypeWhiteboard, "user2", []byte("{}"))

	// 列出所有文档
	docs, total, err := manager.ListDocuments(ctx, nil)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected 3 documents, got %d", total)
	}

	if len(docs) != 3 {
		t.Errorf("Expected 3 documents in result, got %d", len(docs))
	}

	// 按类型过滤
	docType := DocumentTypeWhiteboard
	filter := &DocumentFilter{
		Type: &docType,
	}
	docs, total, err = manager.ListDocuments(ctx, filter)
	if err != nil {
		t.Fatalf("ListDocuments with filter failed: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected 2 whiteboard documents, got %d", total)
	}

	// 按用户过滤
	user := "user1"
	filter = &DocumentFilter{
		UserID: &user,
	}
	docs, total, err = manager.ListDocuments(ctx, filter)
	if err != nil {
		t.Fatalf("ListDocuments with user filter failed: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected 2 documents for user1, got %d", total)
	}
}

func TestManager_DeleteDocument(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	// 删除文档
	err := manager.DeleteDocument(ctx, doc.ID, "user1")
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	// 验证文档状态
	deleted, _ := manager.GetDocument(ctx, doc.ID)
	if deleted.State != DocumentStateDeleted {
		t.Errorf("Expected state deleted, got %s", deleted.State)
	}
}

func TestManager_GetOperationHistory(t *testing.T) {
	manager := createTestManager(t)
	defer manager.Close()

	ctx := context.Background()

	// 创建文档
	doc, _ := manager.CreateDocument(
		ctx,
		"Test Doc",
		DocumentTypeWhiteboard,
		"user1",
		[]byte("{}"),
	)

	sessionID, _ := guuid.NewV7()

	// 应用多个操作
	for i := 0; i < 5; i++ {
		opID, _ := guuid.NewV7()
		op := &Operation{
			ID:          opID,
			DocID:       doc.ID,
			UserID:      "user1",
			SessionID:   sessionID,
			Type:        OperationTypeUpdate,
			Data:        []byte(`{}`),
			PrevVersion: uint64(i + 1),
			Status:      OperationStatusPending,
		}
		_ = manager.ApplyOperation(ctx, op)

		// 更新文档版本供下一次操作
		doc, _ = manager.GetDocument(ctx, doc.ID)
	}

	// 获取操作历史
	ops, err := manager.GetOperationHistory(ctx, doc.ID, 10)
	if err != nil {
		t.Fatalf("GetOperationHistory failed: %v", err)
	}

	if len(ops) != 5 {
		t.Errorf("Expected 5 operations, got %d", len(ops))
	}
}
