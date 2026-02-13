-- Migration: 001_initial_schema
-- Description: Initial schema for StateSync service
-- Date: 2024-01-15

BEGIN;

-- 启用 UUID 扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 文档表
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
    public BOOLEAN DEFAULT FALSE,
    
    CONSTRAINT chk_type CHECK (type IN ('whiteboard', 'text', 'canvas', 'sheet')),
    CONSTRAINT chk_state CHECK (state IN ('active', 'archived', 'deleted'))
);

CREATE INDEX idx_documents_created_by ON documents(created_by);
CREATE INDEX idx_documents_type ON documents(type);
CREATE INDEX idx_documents_state ON documents(state);
CREATE INDEX idx_documents_created_at ON documents(created_at DESC);
CREATE INDEX idx_documents_updated_at ON documents(updated_at DESC);
CREATE INDEX idx_documents_owner ON documents(owner);
CREATE INDEX idx_documents_active_users ON documents USING GIN(active_users);
CREATE INDEX idx_documents_properties ON documents USING GIN(properties);

-- 操作表
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
    extra JSONB,
    
    CONSTRAINT chk_op_type CHECK (type IN ('create', 'update', 'delete', 'move', 'resize', 'style', 'text')),
    CONSTRAINT chk_op_status CHECK (status IN ('pending', 'applied', 'conflict', 'rejected', 'resolved'))
);

CREATE INDEX idx_operations_doc_id ON operations(doc_id);
CREATE INDEX idx_operations_user_id ON operations(user_id);
CREATE INDEX idx_operations_session_id ON operations(session_id);
CREATE INDEX idx_operations_timestamp ON operations(timestamp DESC);
CREATE INDEX idx_operations_version ON operations(doc_id, version DESC);
CREATE INDEX idx_operations_status ON operations(status);
CREATE INDEX idx_operations_doc_time ON operations(doc_id, timestamp DESC);

-- 冲突表
CREATE TABLE conflicts (
    id UUID PRIMARY KEY,
    doc_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    resolution VARCHAR(50) NOT NULL DEFAULT 'manual',
    resolved_by VARCHAR(255),
    resolved_at TIMESTAMP,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT chk_resolution CHECK (resolution IN ('lww', 'manual', 'merge'))
);

CREATE INDEX idx_conflicts_doc_id ON conflicts(doc_id);
CREATE INDEX idx_conflicts_resolved_by ON conflicts(resolved_by);
CREATE INDEX idx_conflicts_created_at ON conflicts(created_at DESC);

-- 冲突操作关联表
CREATE TABLE conflict_operations (
    conflict_id UUID NOT NULL REFERENCES conflicts(id) ON DELETE CASCADE,
    operation_id UUID NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
    PRIMARY KEY (conflict_id, operation_id)
);

CREATE INDEX idx_conflict_ops_conflict ON conflict_operations(conflict_id);
CREATE INDEX idx_conflict_ops_operation ON conflict_operations(operation_id);

-- 锁表
CREATE TABLE locks (
    id UUID PRIMARY KEY,
    doc_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    session_id UUID NOT NULL,
    acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    
    CONSTRAINT uq_doc_active_lock UNIQUE(doc_id, active)
);

CREATE INDEX idx_locks_doc_id ON locks(doc_id);
CREATE INDEX idx_locks_user_id ON locks(user_id);
CREATE INDEX idx_locks_expires_at ON locks(expires_at);
CREATE INDEX idx_locks_active ON locks(active);

-- 触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 原子更新版本函数
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

-- 清理过期锁函数
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

-- 添加/移除活跃用户函数
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

COMMIT;
