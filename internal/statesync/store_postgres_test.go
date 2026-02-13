package statesync

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// 创建测试用的 PostgreSQL 连接
func newTestPostgresDB(t *testing.T) *sql.DB {
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=aetherflow_test sslmode=disable"
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	
	return db
}

// 检查 PostgreSQL 是否可用
func isPostgresAvailable(t *testing.T) bool {
	db := newTestPostgresDB(t)
	defer db.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err := db.PingContext(ctx)
	return err == nil
}

// 初始化测试数据库
func setupTestDB(t *testing.T) *sql.DB {
	// 连接到默认数据库
	defaultConn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", defaultConn)
	require.NoError(t, err)
	
	// 删除测试数据库（如果存在）
	_, _ = db.Exec("DROP DATABASE IF EXISTS aetherflow_test")
	
	// 创建测试数据库
	_, err = db.Exec("CREATE DATABASE aetherflow_test")
	require.NoError(t, err)
	
	db.Close()
	
	// 连接到测试数据库
	testDB := newTestPostgresDB(t)
	
	// 读取并执行 schema
	ctx := context.Background()
	
	// 执行 schema（简化版，仅核心表）
	_, err = testDB.ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		
		CREATE TABLE documents (
			id UUID PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			state VARCHAR(50) NOT NULL DEFAULT 'active',
			version BIGINT NOT NULL DEFAULT 0,
			content BYTEA,
			created_by VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_by VARCHAR(255),
			active_users TEXT[],
			tags TEXT[],
			description TEXT,
			properties JSONB,
			owner VARCHAR(255),
			editors TEXT[],
			viewers TEXT[],
			public BOOLEAN DEFAULT FALSE
		);
		
		CREATE TABLE operations (
			id UUID PRIMARY KEY,
			doc_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			user_id VARCHAR(255) NOT NULL,
			session_id UUID NOT NULL,
			type VARCHAR(50) NOT NULL,
			data BYTEA NOT NULL,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			version BIGINT NOT NULL,
			prev_version BIGINT NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			client_id VARCHAR(255),
			ip VARCHAR(45),
			user_agent TEXT,
			platform VARCHAR(100),
			extra JSONB
		);
		
		CREATE TABLE conflicts (
			id UUID PRIMARY KEY,
			doc_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			resolution VARCHAR(50) NOT NULL DEFAULT 'manual',
			resolved_by VARCHAR(255),
			resolved_at TIMESTAMP,
			description TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE conflict_operations (
			conflict_id UUID NOT NULL REFERENCES conflicts(id) ON DELETE CASCADE,
			operation_id UUID NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
			PRIMARY KEY (conflict_id, operation_id)
		);
		
		CREATE TABLE locks (
			id UUID PRIMARY KEY,
			doc_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			user_id VARCHAR(255) NOT NULL,
			session_id UUID NOT NULL,
			acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			active BOOLEAN NOT NULL DEFAULT TRUE
		);
		
		CREATE OR REPLACE FUNCTION atomic_update_document_version(
			p_doc_id UUID,
			p_old_version BIGINT,
			p_new_version BIGINT,
			p_content BYTEA,
			p_updated_by VARCHAR(255)
		)
		RETURNS BOOLEAN AS $$
		DECLARE
			rows_affected INT;
		BEGIN
			UPDATE documents
			SET 
				version = p_new_version,
				content = p_content,
				updated_by = p_updated_by,
				updated_at = CURRENT_TIMESTAMP
			WHERE 
				id = p_doc_id 
				AND version = p_old_version;
			
			GET DIAGNOSTICS rows_affected = ROW_COUNT;
			
			RETURN rows_affected > 0;
		END;
		$$ LANGUAGE plpgsql;
		
		CREATE OR REPLACE FUNCTION clean_expired_locks()
		RETURNS INT AS $$
		DECLARE
			rows_affected INT;
		BEGIN
			UPDATE locks
			SET active = FALSE
			WHERE active = TRUE 
			  AND expires_at < CURRENT_TIMESTAMP;
			
			GET DIAGNOSTICS rows_affected = ROW_COUNT;
			
			RETURN rows_affected;
		END;
		$$ LANGUAGE plpgsql;
		
		CREATE OR REPLACE FUNCTION add_active_user(
			p_doc_id UUID,
			p_user_id VARCHAR(255)
		)
		RETURNS VOID AS $$
		BEGIN
			UPDATE documents
			SET active_users = array_append(
				COALESCE(active_users, ARRAY[]::TEXT[]),
				p_user_id
			)
			WHERE id = p_doc_id
			  AND (active_users IS NULL OR NOT (p_user_id = ANY(active_users)));
		END;
		$$ LANGUAGE plpgsql;
		
		CREATE OR REPLACE FUNCTION remove_active_user(
			p_doc_id UUID,
			p_user_id VARCHAR(255)
		)
		RETURNS VOID AS $$
		BEGIN
			UPDATE documents
			SET active_users = array_remove(active_users, p_user_id)
			WHERE id = p_doc_id;
		END;
		$$ LANGUAGE plpgsql;
	`)
	require.NoError(t, err)
	
	return testDB
}

// 清理测试数据库
func teardownTestDB(t *testing.T, db *sql.DB) {
	db.Close()
	
	// 删除测试数据库
	defaultConn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	defaultDB, err := sql.Open("postgres", defaultConn)
	if err == nil {
		_, _ = defaultDB.Exec("DROP DATABASE IF EXISTS aetherflow_test")
		defaultDB.Close()
	}
}

func TestPostgresStore_CreateAndGetDocument(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建测试文档
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Test Document",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		Version:   1,
		Content:   []byte("test content"),
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UpdatedBy: "test-user",
		Metadata: Metadata{
			Tags:        []string{"test", "demo"},
			Description: "Test document",
			Properties:  map[string]string{"key": "value"},
			Permissions: Permissions{
				Owner:  "test-user",
				Public: false,
			},
		},
	}

	// 测试创建
	err = store.CreateDocument(ctx, doc)
	assert.NoError(t, err)

	// 测试获取
	retrieved, err := store.GetDocument(ctx, docID)
	assert.NoError(t, err)
	assert.Equal(t, doc.ID, retrieved.ID)
	assert.Equal(t, doc.Name, retrieved.Name)
	assert.Equal(t, doc.Type, retrieved.Type)
}

func TestPostgresStore_UpdateDocument(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建文档
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Original Name",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		Version:   1,
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  Metadata{},
	}

	err = store.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// 更新文档
	doc.Name = "Updated Name"
	doc.Version = 2
	err = store.UpdateDocument(ctx, doc)
	assert.NoError(t, err)

	// 验证更新
	retrieved, err := store.GetDocument(ctx, docID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
	assert.Equal(t, uint64(2), retrieved.Version)
}

func TestPostgresStore_DeleteDocument(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建文档
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Test Document",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  Metadata{},
	}

	err = store.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// 软删除
	err = store.DeleteDocument(ctx, docID)
	assert.NoError(t, err)

	// 验证删除（GetDocument 应该返回 not found）
	_, err = store.GetDocument(ctx, docID)
	assert.Error(t, err)
}

func TestPostgresStore_CreateAndGetOperation(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 先创建文档
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Test Doc",
		Type:      DocumentTypeWhiteboard,
		CreatedBy: "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  Metadata{},
	}
	err = store.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// 创建操作
	opID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()
	op := &Operation{
		ID:          opID,
		DocID:       docID,
		UserID:      "test-user",
		SessionID:   sessionID,
		Type:        OperationTypeCreate,
		Data:        []byte("operation data"),
		Timestamp:   time.Now(),
		Version:     1,
		PrevVersion: 0,
		Status:      OperationStatusPending,
		ClientID:    "client-1",
		Metadata:    OpMetadata{},
	}

	err = store.CreateOperation(ctx, op)
	assert.NoError(t, err)

	// 获取操作
	retrieved, err := store.GetOperation(ctx, opID)
	assert.NoError(t, err)
	assert.Equal(t, op.ID, retrieved.ID)
	assert.Equal(t, op.Type, retrieved.Type)
}

func TestPostgresStore_AtomicVersionUpdate(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建文档
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Version Test",
		Type:      DocumentTypeWhiteboard,
		Version:   1,
		Content:   []byte("v1"),
		CreatedBy: "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  Metadata{},
	}
	err = store.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// 原子更新版本（正确的旧版本）
	err = store.UpdateDocumentVersion(ctx, docID, 1, 2, []byte("v2"))
	assert.NoError(t, err)

	// 原子更新版本（错误的旧版本，应该失败）
	err = store.UpdateDocumentVersion(ctx, docID, 1, 3, []byte("v3"))
	assert.Error(t, err, "应该因为版本冲突而失败")

	// 验证版本
	retrieved, err := store.GetDocument(ctx, docID)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), retrieved.Version)
}

func TestPostgresStore_Lock(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建文档
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Lock Test",
		Type:      DocumentTypeWhiteboard,
		CreatedBy: "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  Metadata{},
	}
	err = store.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// 获取锁
	lockID, _ := guuid.NewV7()
	sessionID, _ := guuid.NewV7()
	lock := &Lock{
		ID:         lockID,
		DocID:      docID,
		UserID:     "test-user",
		SessionID:  sessionID,
		AcquiredAt: time.Now(),
		ExpiresAt:  time.Now().Add(30 * time.Second),
		Active:     true,
	}

	err = store.AcquireLock(ctx, lock)
	assert.NoError(t, err)

	// 检查锁
	locked, err := store.IsLocked(ctx, docID)
	assert.NoError(t, err)
	assert.True(t, locked)

	// 获取锁信息
	retrievedLock, err := store.GetLock(ctx, docID)
	assert.NoError(t, err)
	assert.Equal(t, lock.UserID, retrievedLock.UserID)

	// 释放锁
	err = store.ReleaseLock(ctx, docID, "test-user")
	assert.NoError(t, err)

	// 验证锁已释放
	locked, err = store.IsLocked(ctx, docID)
	assert.NoError(t, err)
	assert.False(t, locked)
}

func TestPostgresStore_ListDocuments(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建多个文档
	for i := 0; i < 5; i++ {
		docID, _ := guuid.NewV7()
		doc := &Document{
			ID:        docID,
			Name:      fmt.Sprintf("Doc %d", i),
			Type:      DocumentTypeWhiteboard,
			State:     DocumentStateActive,
			CreatedBy: "test-user",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  Metadata{},
		}
		err = store.CreateDocument(ctx, doc)
		require.NoError(t, err)
	}

	// 列出文档
	docs, total, err := store.ListDocuments(ctx, &DocumentFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Equal(t, 5, len(docs))

	// 测试分页
	docs, total, err = store.ListDocuments(ctx, &DocumentFilter{
		Limit:  2,
		Offset: 0,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Equal(t, 2, len(docs))
}

func TestPostgresStore_GetStats(t *testing.T) {
	if !isPostgresAvailable(t) {
		t.Skip("PostgreSQL not available, skipping test")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	store, err := NewPostgresStore(&PostgresStoreConfig{
		DB:     db,
		Logger: zaptest.NewLogger(t),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建测试数据
	docID, _ := guuid.NewV7()
	doc := &Document{
		ID:        docID,
		Name:      "Stats Test",
		Type:      DocumentTypeWhiteboard,
		State:     DocumentStateActive,
		CreatedBy: "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  Metadata{},
	}
	err = store.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// 获取统计信息
	stats, err := store.GetStats(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalDocuments)
	assert.Equal(t, int64(1), stats.ActiveDocuments)
}
