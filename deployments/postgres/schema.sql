-- AetherFlow StateSync PostgreSQL Schema
-- Version: 1.0.0

-- 启用 UUID 扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ==================== 文档表 ====================

CREATE TABLE IF NOT EXISTS documents (
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
    active_users TEXT[], -- 活跃用户列表
    
    -- 元数据（存储为 JSONB）
    tags TEXT[],
    description TEXT,
    properties JSONB,
    
    -- 权限
    owner VARCHAR(255),
    editors TEXT[],
    viewers TEXT[],
    public BOOLEAN DEFAULT FALSE,
    
    -- 索引
    CONSTRAINT chk_type CHECK (type IN ('whiteboard', 'text', 'canvas', 'sheet')),
    CONSTRAINT chk_state CHECK (state IN ('active', 'archived', 'deleted'))
);

-- 文档表索引
CREATE INDEX idx_documents_created_by ON documents(created_by);
CREATE INDEX idx_documents_type ON documents(type);
CREATE INDEX idx_documents_state ON documents(state);
CREATE INDEX idx_documents_created_at ON documents(created_at DESC);
CREATE INDEX idx_documents_updated_at ON documents(updated_at DESC);
CREATE INDEX idx_documents_owner ON documents(owner);
CREATE INDEX idx_documents_active_users ON documents USING GIN(active_users);
CREATE INDEX idx_documents_properties ON documents USING GIN(properties);

-- ==================== 操作表 ====================

CREATE TABLE IF NOT EXISTS operations (
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
    
    -- 操作元数据
    ip VARCHAR(45),
    user_agent TEXT,
    platform VARCHAR(100),
    extra JSONB,
    
    -- 约束
    CONSTRAINT chk_op_type CHECK (type IN ('create', 'update', 'delete', 'move', 'resize', 'style', 'text')),
    CONSTRAINT chk_op_status CHECK (status IN ('pending', 'applied', 'conflict', 'rejected', 'resolved'))
);

-- 操作表索引
CREATE INDEX idx_operations_doc_id ON operations(doc_id);
CREATE INDEX idx_operations_user_id ON operations(user_id);
CREATE INDEX idx_operations_session_id ON operations(session_id);
CREATE INDEX idx_operations_timestamp ON operations(timestamp DESC);
CREATE INDEX idx_operations_version ON operations(doc_id, version DESC);
CREATE INDEX idx_operations_status ON operations(status);
CREATE INDEX idx_operations_doc_time ON operations(doc_id, timestamp DESC);

-- ==================== 冲突表 ====================

CREATE TABLE IF NOT EXISTS conflicts (
    id UUID PRIMARY KEY,
    doc_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    resolution VARCHAR(50) NOT NULL DEFAULT 'manual',
    resolved_by VARCHAR(255),
    resolved_at TIMESTAMP,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- 约束
    CONSTRAINT chk_resolution CHECK (resolution IN ('lww', 'manual', 'merge'))
);

-- 冲突表索引
CREATE INDEX idx_conflicts_doc_id ON conflicts(doc_id);
CREATE INDEX idx_conflicts_resolved_by ON conflicts(resolved_by);
CREATE INDEX idx_conflicts_created_at ON conflicts(created_at DESC);

-- 冲突操作关联表 (多对多)
CREATE TABLE IF NOT EXISTS conflict_operations (
    conflict_id UUID NOT NULL REFERENCES conflicts(id) ON DELETE CASCADE,
    operation_id UUID NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
    PRIMARY KEY (conflict_id, operation_id)
);

CREATE INDEX idx_conflict_ops_conflict ON conflict_operations(conflict_id);
CREATE INDEX idx_conflict_ops_operation ON conflict_operations(operation_id);

-- ==================== 锁表 ====================

CREATE TABLE IF NOT EXISTS locks (
    id UUID PRIMARY KEY,
    doc_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    session_id UUID NOT NULL,
    acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- 唯一约束：每个文档只能有一个活跃的锁
    CONSTRAINT uq_doc_active_lock UNIQUE(doc_id, active)
);

-- 锁表索引
CREATE INDEX idx_locks_doc_id ON locks(doc_id);
CREATE INDEX idx_locks_user_id ON locks(user_id);
CREATE INDEX idx_locks_expires_at ON locks(expires_at);
CREATE INDEX idx_locks_active ON locks(active);

-- ==================== 视图 ====================

-- 活跃文档视图
CREATE OR REPLACE VIEW v_active_documents AS
SELECT 
    d.*,
    COUNT(DISTINCT o.id) as operation_count,
    COUNT(DISTINCT l.id) as lock_count
FROM documents d
LEFT JOIN operations o ON d.id = o.doc_id
LEFT JOIN locks l ON d.id = l.doc_id AND l.active = TRUE
WHERE d.state = 'active'
GROUP BY d.id;

-- 文档统计视图
CREATE OR REPLACE VIEW v_document_stats AS
SELECT 
    d.id,
    d.name,
    d.type,
    d.state,
    d.version,
    COUNT(DISTINCT o.id) as total_operations,
    COUNT(DISTINCT CASE WHEN o.status = 'pending' THEN o.id END) as pending_operations,
    COUNT(DISTINCT c.id) as total_conflicts,
    COUNT(DISTINCT CASE WHEN c.resolved_at IS NULL THEN c.id END) as unresolved_conflicts,
    array_length(d.active_users, 1) as active_user_count,
    d.created_at,
    d.updated_at
FROM documents d
LEFT JOIN operations o ON d.id = o.doc_id
LEFT JOIN conflicts c ON d.id = c.doc_id
GROUP BY d.id;

-- ==================== 函数 ====================

-- 更新 updated_at 触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为 documents 表创建触发器
DROP TRIGGER IF EXISTS trigger_update_documents_updated_at ON documents;
CREATE TRIGGER trigger_update_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 原子更新文档版本函数
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

-- 添加活跃用户函数
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

-- 移除活跃用户函数
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

-- ==================== 初始数据 ====================

-- 可以在这里插入一些测试数据（可选）

-- ==================== 权限管理 ====================

-- 创建只读用户（可选，用于查询服务）
-- CREATE USER aetherflow_readonly WITH PASSWORD 'your_password';
-- GRANT CONNECT ON DATABASE aetherflow TO aetherflow_readonly;
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO aetherflow_readonly;

-- 创建读写用户（应用使用）
-- CREATE USER aetherflow_app WITH PASSWORD 'your_password';
-- GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO aetherflow_app;
-- GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO aetherflow_app;

-- ==================== 维护任务 ====================

-- 定期清理过期锁（建议通过 pg_cron 或应用层定时任务执行）
-- SELECT clean_expired_locks();

-- 定期分析表以优化查询计划
-- ANALYZE documents;
-- ANALYZE operations;
-- ANALYZE conflicts;
-- ANALYZE locks;

-- ==================== 监控查询 ====================

-- 查看表大小
-- SELECT 
--     schemaname,
--     tablename,
--     pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
-- FROM pg_tables
-- WHERE schemaname = 'public'
-- ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- 查看索引使用情况
-- SELECT 
--     schemaname,
--     tablename,
--     indexname,
--     idx_scan as index_scans,
--     idx_tup_read as tuples_read,
--     idx_tup_fetch as tuples_fetched
-- FROM pg_stat_user_indexes
-- ORDER BY idx_scan DESC;

COMMIT;
