# PostgreSQL Store 实现指南

本文档详细介绍 AetherFlow StateSync Service 的 PostgreSQL Store 实现。

## 📋 目录

- [概述](#概述)
- [数据库设计](#数据库设计)
- [核心功能](#核心功能)
- [配置说明](#配置说明)
- [使用指南](#使用指南)
- [性能优化](#性能优化)
- [故障排查](#故障排查)

## 概述

### 为什么需要 PostgreSQL Store

**MemoryStore 的局限**:
- ❌ 重启后数据丢失
- ❌ 无法跨实例共享
- ❌ 不支持复杂查询
- ❌ 无 ACID 保证

**PostgreSQL Store 的优势**:
- ✅ 完整的 ACID 事务
- ✅ 复杂查询支持（JOIN, 聚合）
- ✅ 数据持久化
- ✅ 多实例共享
- ✅ 丰富的索引（B-tree, GIN, JSONB）
- ✅ 生产就绪

### 技术特性

- **ACID 事务**: 保证数据一致性
- **关系型设计**: 文档、操作、冲突、锁 4张核心表
- **JSONB 支持**: 灵活的元数据存储
- **数组类型**: 高效的标签和权限管理
- **触发器**: 自动更新时间戳
- **存储函数**: 原子版本更新、锁清理

## 数据库设计

### ER 图

```
┌─────────────┐         ┌──────────────┐
│  documents  │◄────────┤  operations  │
│             │ 1     * │              │
│  - id (PK)  │         │  - id (PK)   │
│  - name     │         │  - doc_id (FK)│
│  - type     │         │  - version   │
│  - version  │         │  - data      │
│  - content  │         └──────────────┘
└──────┬──────┘                │
       │                       │
       │ 1                     │ *
       │                       │
       │                ┌──────▼────────────┐
       │                │ conflict_operations│
       │                │                   │
       │                │  - conflict_id (FK)│
       │                │  - operation_id (FK)│
       │                └───────────────────┘
       │                       │
       │ *                     │ *
       │                       │
┌──────▼──────┐         ┌──────▼──────┐
│  conflicts  │         │    locks    │
│             │         │             │
│  - id (PK)  │         │  - id (PK)  │
│  - doc_id   │         │  - doc_id   │
│  - resolution│        │  - user_id  │
└─────────────┘         │  - active   │
                        └─────────────┘
```

### 表结构

#### 1. documents 表（文档）

| 字段 | 类型 | 说明 | 索引 |
|------|------|------|------|
| id | UUID | 主键 | PK |
| name | VARCHAR(255) | 文档名称 | - |
| type | VARCHAR(50) | 文档类型 | ✅ |
| state | VARCHAR(50) | 状态 | ✅ |
| version | BIGINT | 版本号 | - |
| content | BYTEA | 内容 | - |
| created_by | VARCHAR(255) | 创建者 | ✅ |
| created_at | TIMESTAMP | 创建时间 | ✅ |
| updated_at | TIMESTAMP | 更新时间 | ✅ |
| updated_by | VARCHAR(255) | 更新者 | - |
| active_users | TEXT[] | 活跃用户 | GIN |
| tags | TEXT[] | 标签 | - |
| description | TEXT | 描述 | - |
| properties | JSONB | 属性 | GIN |
| owner | VARCHAR(255) | 拥有者 | ✅ |
| editors | TEXT[] | 编辑者 | - |
| viewers | TEXT[] | 查看者 | - |
| public | BOOLEAN | 公开 | - |

**索引策略**:
- B-tree: created_by, type, state, created_at, updated_at, owner
- GIN: active_users, properties

#### 2. operations 表（操作）

| 字段 | 类型 | 说明 | 索引 |
|------|------|------|------|
| id | UUID | 主键 | PK |
| doc_id | UUID | 文档ID（外键） | ✅ |
| user_id | VARCHAR(255) | 用户ID | ✅ |
| session_id | UUID | 会话ID | ✅ |
| type | VARCHAR(50) | 操作类型 | - |
| data | BYTEA | 操作数据 | - |
| timestamp | TIMESTAMP | 时间戳 | ✅ |
| version | BIGINT | 版本号 | 复合 |
| prev_version | BIGINT | 前版本 | - |
| status | VARCHAR(50) | 状态 | ✅ |
| client_id | VARCHAR(255) | 客户端ID | - |
| ip | VARCHAR(45) | IP地址 | - |
| user_agent | TEXT | User Agent | - |
| platform | VARCHAR(100) | 平台 | - |
| extra | JSONB | 额外数据 | - |

**索引策略**:
- 单列: doc_id, user_id, session_id, timestamp, status
- 复合: (doc_id, version), (doc_id, timestamp)

#### 3. conflicts 表（冲突）

| 字段 | 类型 | 说明 | 索引 |
|------|------|------|------|
| id | UUID | 主键 | PK |
| doc_id | UUID | 文档ID（外键） | ✅ |
| resolution | VARCHAR(50) | 解决策略 | - |
| resolved_by | VARCHAR(255) | 解决者 | ✅ |
| resolved_at | TIMESTAMP | 解决时间 | - |
| description | TEXT | 描述 | - |
| created_at | TIMESTAMP | 创建时间 | ✅ |

#### 4. locks 表（锁）

| 字段 | 类型 | 说明 | 索引 |
|------|------|------|------|
| id | UUID | 主键 | PK |
| doc_id | UUID | 文档ID（外键） | ✅ |
| user_id | VARCHAR(255) | 用户ID | ✅ |
| session_id | UUID | 会话ID | - |
| acquired_at | TIMESTAMP | 获取时间 | - |
| expires_at | TIMESTAMP | 过期时间 | ✅ |
| active | BOOLEAN | 是否活跃 | ✅ |

**唯一约束**: (doc_id, active) - 每个文档只能有一个活跃锁

### 存储函数

#### 1. atomic_update_document_version

原子更新文档版本，防止并发冲突。

```sql
SELECT atomic_update_document_version(
    doc_id,      -- 文档ID
    old_version, -- 期望的旧版本
    new_version, -- 新版本
    content,     -- 新内容
    updated_by   -- 更新者
);
-- 返回: BOOLEAN (成功/失败)
```

**特点**: 
- ✅ 原子性：使用 WHERE version = old_version
- ✅ 乐观锁：版本不匹配则失败
- ✅ 自动更新 updated_at

#### 2. clean_expired_locks

清理过期的锁。

```sql
SELECT clean_expired_locks();
-- 返回: INT (清理的锁数量)
```

#### 3. add_active_user / remove_active_user

管理活跃用户列表。

```sql
SELECT add_active_user(doc_id, user_id);
SELECT remove_active_user(doc_id, user_id);
```

## 核心功能

### 1. 文档管理

**创建文档**:
```go
doc := &Document{
    ID:        docID,
    Name:      "白板文档",
    Type:      DocumentTypeWhiteboard,
    CreatedBy: "user-001",
    // ...
}
store.CreateDocument(ctx, doc)
```

**获取文档**:
```go
doc, err := store.GetDocument(ctx, docID)
```

**更新文档**:
```go
doc.Version = 2
store.UpdateDocument(ctx, doc)
```

**原子版本更新**:
```go
// 乐观锁机制
err := store.UpdateDocumentVersion(ctx, docID, oldVer, newVer, content)
if err != nil {
    // 版本冲突，需要重试
}
```

### 2. 操作管理

**创建操作**:
```go
op := &Operation{
    ID:      opID,
    DocID:   docID,
    Type:    OperationTypeCreate,
    Version: 1,
    // ...
}
store.CreateOperation(ctx, op)
```

**查询操作历史**:
```go
// 获取文档的最近100个操作
ops, err := store.GetOperationsByDocument(ctx, docID, 100)

// 获取版本范围内的操作
ops, err := store.GetOperationsByVersion(ctx, docID, 1, 10)
```

### 3. 冲突管理

**创建冲突记录**:
```go
conflict := &Conflict{
    ID:          conflictID,
    DocID:       docID,
    Ops:         []*Operation{op1, op2}, // 冲突的操作
    Resolution:  ConflictResolutionLWW,
    // ...
}
store.CreateConflict(ctx, conflict)
```

**查询冲突**:
```go
// 获取未解决的冲突
conflicts, err := store.GetUnresolvedConflicts(ctx, docID)
```

### 4. 锁管理

**获取锁**:
```go
lock := &Lock{
    ID:        lockID,
    DocID:     docID,
    UserID:    "user-001",
    ExpiresAt: time.Now().Add(30 * time.Second),
    Active:    true,
}
err := store.AcquireLock(ctx, lock)
```

**释放锁**:
```go
err := store.ReleaseLock(ctx, docID, userID)
```

**检查锁**:
```go
locked, err := store.IsLocked(ctx, docID)
if locked {
    // 文档已被锁定
}
```

## 配置说明

### StateSync Service 配置

**configs/statesync.yaml**:

```yaml
Server:
  Host: 0.0.0.0
  Port: 9002

Store:
  Type: postgres  # memory, postgres
  Postgres:
    Host: localhost        # PostgreSQL 地址
    Port: 5432            # PostgreSQL 端口
    User: postgres        # 数据库用户
    Password: postgres    # 数据库密码
    DBName: aetherflow    # 数据库名称
    SSLMode: disable      # SSL 模式（disable, require, verify-full）
    MaxOpenConns: 25      # 最大连接数
    MaxIdleConns: 5       # 最大空闲连接

Manager:
  LockTimeout: 30s
  CleanupInterval: 5m
  AutoResolveConflicts: true
```

### PostgreSQL 配置建议

**postgresql.conf**:

```conf
# 连接设置
max_connections = 100
shared_buffers = 256MB
effective_cache_size = 1GB

# 性能优化
work_mem = 16MB
maintenance_work_mem = 128MB
random_page_cost = 1.1

# WAL 配置
wal_level = replica
max_wal_size = 1GB
min_wal_size = 80MB

# 检查点
checkpoint_timeout = 10min
checkpoint_completion_target = 0.9

# 日志
log_min_duration_statement = 1000
log_connections = on
log_disconnections = on
```

## 使用指南

### 方式 1: 本地 PostgreSQL

```bash
# 1. 安装 PostgreSQL
brew install postgresql@15  # macOS
apt install postgresql-15   # Ubuntu

# 2. 启动 PostgreSQL
pg_ctl start -D /usr/local/var/postgres

# 3. 创建数据库和运行迁移
./scripts/migrate-postgres.sh up

# 4. 修改配置
vim configs/statesync.yaml  # 设置 Store.Type: postgres

# 5. 启动服务
./scripts/start-with-postgres.sh
```

### 方式 2: Docker Compose

```bash
# 启动 PostgreSQL + StateSync Service
docker-compose -f deployments/docker-compose.postgres.yml up -d

# 查看日志
docker-compose logs -f statesync-service

# 访问 pgAdmin (Web UI)
open http://localhost:5050
# Email: admin@aetherflow.com
# Password: admin
```

### 方式 3: Kubernetes

```bash
# 部署 PostgreSQL StatefulSet
kubectl apply -f deployments/kubernetes/postgres-statefulset.yaml

# 部署 StateSync Service
kubectl apply -f deployments/kubernetes/statesync-service-deployment.yaml
```

### 数据库迁移

```bash
# 应用迁移（创建表）
./scripts/migrate-postgres.sh up

# 回滚迁移（删除表）
./scripts/migrate-postgres.sh down

# 重置数据库
./scripts/migrate-postgres.sh reset
```

### 验证数据库

```bash
# 连接到数据库
psql -h localhost -p 5432 -U postgres -d aetherflow

# 查看表
\dt

# 查看文档
SELECT id, name, type, state, version FROM documents;

# 查看操作
SELECT id, type, version, status FROM operations LIMIT 10;

# 查看统计
SELECT 
    (SELECT COUNT(*) FROM documents) as docs,
    (SELECT COUNT(*) FROM operations) as ops,
    (SELECT COUNT(*) FROM conflicts) as conflicts,
    (SELECT COUNT(*) FROM locks WHERE active = TRUE) as active_locks;
```

## 性能优化

### 1. 索引优化

**已创建的索引**:
```sql
-- 文档表（8个索引）
CREATE INDEX idx_documents_created_by ON documents(created_by);
CREATE INDEX idx_documents_type ON documents(type);
CREATE INDEX idx_documents_state ON documents(state);
CREATE INDEX idx_documents_created_at ON documents(created_at DESC);
CREATE INDEX idx_documents_updated_at ON documents(updated_at DESC);
CREATE INDEX idx_documents_owner ON documents(owner);
CREATE INDEX idx_documents_active_users ON documents USING GIN(active_users);
CREATE INDEX idx_documents_properties ON documents USING GIN(properties);

-- 操作表（7个索引）
CREATE INDEX idx_operations_doc_id ON operations(doc_id);
CREATE INDEX idx_operations_timestamp ON operations(timestamp DESC);
CREATE INDEX idx_operations_version ON operations(doc_id, version DESC);
-- ...
```

**性能提升**:
- 查询速度提升 10-100倍
- 支持高效的 JOIN 和排序

### 2. 连接池优化

```yaml
Postgres:
  MaxOpenConns: 25      # 根据并发量调整
  MaxIdleConns: 5       # 保持热连接
```

**建议**:
- 低并发（<100 RPS）: MaxOpenConns=10
- 中并发（100-1000 RPS）: MaxOpenConns=25
- 高并发（>1000 RPS）: MaxOpenConns=50

### 3. 查询优化

**使用 EXPLAIN 分析**:
```sql
EXPLAIN ANALYZE 
SELECT * FROM documents 
WHERE created_by = 'user-001' 
ORDER BY created_at DESC 
LIMIT 10;
```

**批量查询**:
```sql
-- 使用 IN 批量查询
SELECT * FROM documents WHERE id = ANY($1::UUID[]);
```

### 4. 事务优化

```go
// 批量操作使用事务
tx, _ := db.BeginTx(ctx, nil)
defer tx.Rollback()

for _, doc := range docs {
    // 插入操作
}

tx.Commit()
```

### 性能指标

| 操作 | 时间复杂度 | 预期延迟 | 说明 |
|------|-----------|---------|------|
| CreateDocument | O(1) | < 10ms | 含索引更新 |
| GetDocument | O(1) | < 5ms | 主键查询 |
| UpdateDocument | O(1) | < 8ms | 含索引更新 |
| ListDocuments | O(N) | < 50ms | 使用索引 |
| CreateOperation | O(1) | < 5ms | 单条插入 |
| GetOperationsByDocument | O(N) | < 20ms | 索引查询 |
| AcquireLock | O(1) | < 5ms | 唯一约束 |

## 故障排查

### 问题 1: 连接失败

**症状**: `failed to connect to PostgreSQL`

**排查**:
```bash
# 1. 检查 PostgreSQL 是否运行
pg_isready -h localhost -p 5432

# 2. 检查网络连接
telnet localhost 5432

# 3. 检查认证
psql -h localhost -p 5432 -U postgres -d aetherflow

# 4. 查看日志
tail -f /var/log/postgresql/postgresql-15-main.log
```

### 问题 2: 迁移失败

**症状**: Schema 创建失败

**排查**:
```bash
# 查看数据库错误
psql -h localhost -U postgres -d aetherflow

# 手动执行 SQL
psql -h localhost -U postgres -d aetherflow -f deployments/postgres/schema.sql

# 检查表是否存在
psql -h localhost -U postgres -d aetherflow -c '\dt'
```

### 问题 3: 版本冲突

**症状**: `version conflict: expected X`

**原因**: 并发更新导致版本不一致

**解决**: 
```go
// 重试机制
for i := 0; i < 3; i++ {
    doc, _ := store.GetDocument(ctx, docID)
    err := store.UpdateDocumentVersion(ctx, docID, doc.Version, doc.Version+1, newContent)
    if err == nil {
        break
    }
}
```

### 问题 4: 锁冲突

**症状**: `document is already locked`

**排查**:
```sql
-- 查看活跃的锁
SELECT * FROM locks WHERE active = TRUE;

-- 强制释放锁
UPDATE locks SET active = FALSE WHERE doc_id = 'xxx';
```

### 问题 5: 慢查询

**症状**: 查询延迟高

**排查**:
```sql
-- 查看慢查询
SELECT * FROM pg_stat_statements 
ORDER BY mean_exec_time DESC 
LIMIT 10;

-- 分析查询计划
EXPLAIN ANALYZE SELECT ...;

-- 查看缺失的索引
SELECT schemaname, tablename, attname
FROM pg_stats
WHERE schemaname = 'public'
  AND n_distinct > 100;
```

## 监控与运维

### 关键指标

```sql
-- 数据库大小
SELECT pg_size_pretty(pg_database_size('aetherflow'));

-- 表大小
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- 索引使用情况
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan as scans,
    idx_tup_read as tuples_read
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;

-- 连接数
SELECT count(*) FROM pg_stat_activity;

-- 活跃查询
SELECT pid, usename, state, query
FROM pg_stat_activity
WHERE state != 'idle';
```

### 备份与恢复

```bash
# 备份数据库
pg_dump -h localhost -U postgres aetherflow > backup.sql

# 恢复数据库
psql -h localhost -U postgres aetherflow < backup.sql

# 定期备份（crontab）
0 2 * * * pg_dump -h localhost -U postgres aetherflow | gzip > /backup/aetherflow_$(date +\%Y\%m\%d).sql.gz
```

## 最佳实践

### ✅ 推荐

1. **使用事务**: 批量操作时使用事务保证一致性
2. **索引优化**: 根据查询模式创建合适的索引
3. **连接池**: 合理配置连接池大小
4. **定期清理**: 归档旧操作和冲突记录
5. **监控**: 持续监控查询性能和数据库大小

### ❌ 避免

1. **大事务**: 避免长时间事务阻塞
2. **N+1 查询**: 使用 JOIN 或批量查询
3. **无索引查询**: 确保常用查询有索引
4. **全表扫描**: 大表查询添加条件
5. **忽略备份**: 定期备份避免数据丢失

## 参考资料

- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [lib/pq 驱动文档](https://github.com/lib/pq)
- [PostgresStore 实现](../../internal/statesync/store_postgres.go)
- [数据库 Schema](../../deployments/postgres/schema.sql)
- [gRPC 服务指南](./GRPC_SERVICES_GUIDE.md)
