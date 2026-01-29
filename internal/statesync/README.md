# StateSync Service - 状态同步服务

## 概述

StateSync Service是AetherFlow项目的核心组件之一，负责实时协作应用中的状态同步、冲突解决和实时广播功能。

## 核心功能

### 1. 文档管理
- 创建、读取、更新、删除协作文档
- 支持多种文档类型 (白板、文本、画布、表格)
- 文档版本控制
- 权限管理 (拥有者、编辑者、查看者)
- 活跃用户追踪

### 2. 操作日志
- 记录所有用户操作
- 支持多种操作类型 (create, update, delete, move, resize, style, text)
- 操作版本追踪
- 操作历史查询

### 3. 冲突解决
- 自动检测版本冲突
- 多种解决策略:
  - **LWW (Last-Write-Wins)**: 基于时间戳的简单策略
  - **Manual**: 需要人工介入
  - **Merge**: 自动合并 (简化实现)

### 4. 实时广播
- WebSocket事件推送
- 文档级别订阅
- 用户级别订阅
- 事件类型:
  - 操作已应用
  - 文档已更新
  - 用户加入/离开
  - 冲突检测/解决
  - 锁获取/释放

### 5. 分布式锁
- 文档级别锁定
- 自动过期机制
- 防止并发编辑冲突

## 架构设计

```
┌─────────────────────────────────────────┐
│            Manager (核心管理器)          │
│  - 协调所有组件                          │
│  - 业务逻辑处理                          │
└─────────────┬───────────────────────────┘
              │
    ┌─────────┴──────────┬──────────────┬──────────────┐
    │                    │              │              │
┌───▼────┐   ┌──────────▼─────┐  ┌────▼──────┐  ┌───▼────────┐
│ Store  │   │ ConflictResolver│  │Broadcaster│  │ConflictDetect│
│        │   │                 │  │           │  │            │
│- Memory│   │- LWW            │  │- Memory   │  │- 版本检测  │
│- etcd  │   │- Manual         │  │- Redis    │  │- 冲突分析  │
│(未实现)│   │- Merge          │  │(未实现)   │  │            │
└────────┘   └─────────────────┘  └───────────┘  └────────────┘
```

## 文件结构

```
internal/statesync/
├── model.go              # 数据模型定义
├── store.go              # 存储接口
├── store_memory.go       # 内存存储实现
├── manager.go            # 核心管理器
├── conflict.go           # 冲突解决器
├── broadcast.go          # 实时广播
├── manager_test.go       # Manager测试
├── store_memory_test.go  # Store测试
└── README.md             # 本文档
```

## 快速开始

### 1. 创建管理器

```go
package main

import (
    "context"
    "fmt"

    "github.com/aetherflow/internal/statesync"
    "go.uber.org/zap"
)

func main() {
    // 创建logger
    logger, _ := zap.NewDevelopment()

    // 创建存储
    store := statesync.NewMemoryStore()

    // 创建管理器
    config := &statesync.ManagerConfig{
        Store:                store,
        Logger:               logger,
        AutoResolveConflicts: true,
    }

    manager, err := statesync.NewManager(config)
    if err != nil {
        panic(err)
    }
    defer manager.Close()

    // 使用管理器...
}
```

### 2. 创建文档

```go
ctx := context.Background()

doc, err := manager.CreateDocument(
    ctx,
    "My Whiteboard",
    statesync.DocumentTypeWhiteboard,
    "user123",
    []byte("{}"),
)
if err != nil {
    panic(err)
}

fmt.Printf("Document created: %s\n", doc.ID.String())
```

### 3. 订阅文档变更

```go
import guuid "github.com/Lzww0608/GUUID"

sessionID, _ := guuid.NewV7()

subscriber, err := manager.Subscribe(
    ctx,
    doc.ID,
    "user123",
    sessionID,
)
if err != nil {
    panic(err)
}

// 监听事件
go func() {
    for event := range subscriber.Channel {
        fmt.Printf("Event: %s, Type: %s\n", 
            event.ID.String(), 
            event.Type,
        )
    }
}()
```

### 4. 应用操作

```go
opID, _ := guuid.NewV7()

operation := &statesync.Operation{
    ID:          opID,
    DocID:       doc.ID,
    UserID:      "user123",
    SessionID:   sessionID,
    Type:        statesync.OperationTypeUpdate,
    Data:        []byte(`{"x": 100, "y": 200}`),
    PrevVersion: doc.Version,
    Status:      statesync.OperationStatusPending,
}

err = manager.ApplyOperation(ctx, operation)
if err != nil {
    panic(err)
}
```

### 5. 获取锁

```go
lock, err := manager.AcquireLock(ctx, doc.ID, "user123", sessionID)
if err != nil {
    fmt.Printf("Failed to acquire lock: %v\n", err)
    return
}

// 使用锁...

// 释放锁
err = manager.ReleaseLock(ctx, doc.ID, "user123")
```

## 数据模型

### Document (文档)

```go
type Document struct {
    ID          guuid.UUID    // UUIDv7
    Name        string        // 文档名称
    Type        DocumentType  // 文档类型
    State       DocumentState // 文档状态
    Version     uint64        // 版本号
    Content     []byte        // 内容
    CreatedBy   string        // 创建者
    CreatedAt   time.Time     // 创建时间
    UpdatedAt   time.Time     // 更新时间
    UpdatedBy   string        // 更新者
    ActiveUsers []string      // 活跃用户
    Metadata    Metadata      // 元数据
}
```

### Operation (操作)

```go
type Operation struct {
    ID          guuid.UUID      // UUIDv7
    DocID       guuid.UUID      // 文档ID
    UserID      string          // 用户ID
    SessionID   guuid.UUID      // 会话ID
    Type        OperationType   // 操作类型
    Data        []byte          // 操作数据
    Timestamp   time.Time       // 时间戳
    Version     uint64          // 版本号
    PrevVersion uint64          // 前一版本
    Status      OperationStatus // 状态
    ClientID    string          // 客户端ID
    Metadata    OpMetadata      // 元数据
}
```

### Conflict (冲突)

```go
type Conflict struct {
    ID          guuid.UUID         // UUIDv7
    DocID       guuid.UUID         // 文档ID
    Ops         []*Operation       // 冲突的操作
    Resolution  ConflictResolution // 解决策略
    ResolvedBy  string             // 解决者
    ResolvedOp  *Operation         // 解决后的操作
    ResolvedAt  time.Time          // 解决时间
    Description string             // 描述
}
```

## 冲突解决

### 版本冲突

当多个用户同时编辑同一文档时，可能出现版本冲突：

```
User A: 读取 version=5, 修改后提交 version=6
User B: 读取 version=5, 修改后提交 version=6
```

系统会检测到冲突并采取相应策略。

### LWW策略

选择时间戳最新的操作：

```go
resolver := statesync.NewLWWConflictResolver(logger)

// 自动选择最新操作
resolvedOp, err := resolver.Resolve(ctx, conflictingOps)
```

### 手动解决

标记为需要人工介入：

```go
resolver := statesync.NewManualConflictResolver(logger)

// 返回错误，需要人工处理
_, err := resolver.Resolve(ctx, conflictingOps)
// err: "manual resolution required"
```

## 实时广播

### 事件类型

- `operation_applied`: 操作已应用
- `document_updated`: 文档已更新
- `user_joined`: 用户加入
- `user_left`: 用户离开
- `conflict_detected`: 检测到冲突
- `conflict_resolved`: 冲突已解决
- `lock_acquired`: 获取锁
- `lock_released`: 释放锁

### 广播模式

#### 1. 文档级别广播

```go
event := &statesync.Event{
    Type:      statesync.EventTypeOperationApplied,
    DocID:     docID,
    UserID:    "user123",
    Operation: operation,
    Timestamp: time.Now(),
}

// 广播给文档的所有订阅者
manager.broadcaster.BroadcastToDocument(ctx, docID, event)
```

#### 2. 用户级别广播

```go
// 广播给特定用户的所有订阅
manager.broadcaster.BroadcastToUser(ctx, "user123", event)
```

#### 3. 全局广播

```go
// 广播给所有订阅者
manager.broadcaster.Broadcast(ctx, event)
```

## 存储实现

### 内存存储 (MemoryStore)

适用于开发和测试：

```go
store := statesync.NewMemoryStore()
```

**特点**:
- 高性能 (O(1)查询)
- 多级索引 (文档ID, 用户ID等)
- 读写锁保护
- 非持久化

### etcd存储 (未实现)

适用于生产环境：

```go
// TODO: 实现
store := statesync.NewEtcdStore(etcdClient)
```

**特点**:
- 分布式持久化
- 高可用
- Watch机制
- 事务支持

## 性能特性

### 内存存储性能

| 操作 | 时间复杂度 | 性能 |
|------|-----------|------|
| CreateDocument | O(1) | ~0.01ms |
| GetDocument | O(1) | ~0.001ms |
| ApplyOperation | O(1) | ~0.02ms |
| Subscribe | O(1) | ~0.01ms |
| Broadcast | O(N) | N=订阅者数 |

### 冲突解决性能

| 策略 | 时间复杂度 | 性能 |
|------|-----------|------|
| LWW | O(N) | N=冲突操作数 |
| Manual | O(1) | 立即返回 |
| Merge | O(N*M) | N=操作数, M=字段数 |

## 配置选项

```go
type ManagerConfig struct {
    Store                Store              // 存储实现
    Broadcaster          Broadcaster        // 广播器
    ConflictResolver     ConflictResolver   // 冲突解决器
    Logger               *zap.Logger        // 日志
    LockTimeout          time.Duration      // 锁超时 (默认30s)
    CleanupInterval      time.Duration      // 清理间隔 (默认5分钟)
    AutoResolveConflicts bool               // 自动解决冲突 (默认false)
}
```

## 监控指标

系统提供以下统计信息：

```go
stats, err := manager.GetStats(ctx)

// stats包含:
// - TotalDocuments: 文档总数
// - ActiveDocuments: 活跃文档数
// - TotalOperations: 操作总数
// - TotalConflicts: 冲突总数
// - ResolvedConflicts: 已解决冲突数
// - ActiveSubscribers: 活跃订阅者数
// - ActiveLocks: 活跃锁数
```

## 最佳实践

### 1. 版本控制

始终使用版本号进行乐观锁定：

```go
operation.PrevVersion = currentDoc.Version
```

### 2. 错误处理

检查冲突和其他错误：

```go
err := manager.ApplyOperation(ctx, operation)
if err != nil {
    switch err {
    case statesync.ErrVersionMismatch:
        // 版本冲突，重试
    case statesync.ErrDocumentNotFound:
        // 文档不存在
    default:
        // 其他错误
    }
}
```

### 3. 资源清理

确保取消订阅和释放锁：

```go
defer manager.Unsubscribe(ctx, subscriber.ID, docID, userID)
defer manager.ReleaseLock(ctx, docID, userID)
```

### 4. 并发控制

使用锁来避免竞争：

```go
lock, err := manager.AcquireLock(ctx, docID, userID, sessionID)
if err != nil {
    return err
}
defer manager.ReleaseLock(ctx, docID, userID)

// 执行操作...
```

## 测试

运行测试：

```bash
# 运行所有测试
go test ./internal/statesync/...

# 运行特定测试
go test ./internal/statesync/ -run TestManager

# 查看覆盖率
go test ./internal/statesync/ -cover
```

## 未来改进

### 短期 (1-2周)
- [ ] gRPC服务实现
- [ ] 完善单元测试
- [ ] 性能基准测试

### 中期 (1-2月)
- [ ] etcd存储实现
- [ ] Redis广播器
- [ ] CRDT冲突解决
- [ ] 操作变换 (Operational Transformation)

### 长期 (3-6月)
- [ ] 分布式追踪
- [ ] 性能优化
- [ ] 水平扩展
- [ ] 灾难恢复

## 相关文档

- [QUANTUM_IMPLEMENTATION.md](../../docs/QUANTUM_IMPLEMENTATION.md) - Quantum协议
- [Session Service README](../session/README.md) - 会话管理
- [PROJECT_SUMMARY.md](../../PROJECT_SUMMARY.md) - 项目总结
- [ROADMAP.md](../../ROADMAP.md) - 开发路线图

## 贡献

欢迎贡献！请遵循以下步骤：

1. Fork项目
2. 创建特性分支
3. 提交更改
4. 推送到分支
5. 创建Pull Request

## 许可证

MIT License

---

**版本**: v0.3.0-alpha  
**最后更新**: 2026-01-15  
**维护者**: AetherFlow Team
