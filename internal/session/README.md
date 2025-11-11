# Session Management Service

会话管理服务 - AetherFlow项目的核心组件之一，负责管理用户连接和会话生命周期。

## 功能概述

### 核心功能

1. **用户连接管理**
   - 创建和注册新的用户会话
   - 跟踪连接状态（连接中、活跃、空闲、断开中、已关闭）
   - 管理连接元数据（用户ID、IP地址、端口等）
   - 连接与Quantum协议的集成

2. **会话生命周期管理**
   - 会话创建和初始化
   - 会话状态更新
   - 会话心跳保活
   - 会话过期和自动清理
   - 优雅的会话关闭

3. **会话查询和统计**
   - 按会话ID查询
   - 按连接ID查询
   - 按用户ID查询所有会话
   - 会话列表（支持过滤和分页）
   - 会话统计信息（数据包、字节、RTT等）

## 架构设计

### 核心组件

```
┌──────────────────────────────────────────────────────┐
│                  gRPC Service Layer                  │
│              (session_service.proto)                 │
└───────────────────┬──────────────────────────────────┘
                    │
┌───────────────────▼──────────────────────────────────┐
│                  Session Manager                     │
│  - CreateSession()    - UpdateSession()              │
│  - GetSession()       - DeleteSession()              │
│  - Heartbeat()        - ListSessions()               │
│  - 自动过期清理                                       │
└───────────────────┬──────────────────────────────────┘
                    │
┌───────────────────▼──────────────────────────────────┐
│                   Store Interface                    │
│  - Create()  - Get()     - Update()  - Delete()      │
│  - List()    - GetByConnectionID()                   │
│  - GetByUserID()  - DeleteExpired()                  │
└──────────┬────────────────────────────────┬──────────┘
           │                                │
┌──────────▼──────────┐         ┌──────────▼──────────┐
│   Memory Store      │         │    etcd Store       │
│  (开发/测试)         │         │   (生产环境)         │
└─────────────────────┘         └─────────────────────┘
```

### 数据模型

#### Session 会话
```go
type Session struct {
    SessionID    guuid.UUID           // 会话ID (UUIDv7)
    UserID       string               // 用户ID
    ConnectionID guuid.UUID           // Quantum连接ID
    ClientIP     string               // 客户端IP
    ClientPort   uint32               // 客户端端口
    ServerAddr   string               // 服务器地址
    State        State                // 会话状态
    CreatedAt    time.Time            // 创建时间
    LastActiveAt time.Time            // 最后活跃时间
    ExpiresAt    time.Time            // 过期时间
    Metadata     map[string]string    // 自定义元数据
    Stats        *Stats               // 统计信息
}
```

#### State 会话状态
```go
const (
    StateConnecting     // 连接建立中
    StateActive         // 活跃状态
    StateIdle           // 空闲状态
    StateDisconnecting  // 断开中
    StateClosed         // 已关闭
)
```

#### Stats 统计信息
```go
type Stats struct {
    PacketsSent      uint64  // 发送的数据包数
    PacketsReceived  uint64  // 接收的数据包数
    BytesSent        uint64  // 发送的字节数
    BytesReceived    uint64  // 接收的字节数
    Retransmissions  uint64  // 重传次数
    CurrentRTTMs     uint32  // 当前RTT（毫秒）
}
```

## 使用示例

### 基本使用

```go
package main

import (
    "context"
    "time"

    "github.com/aetherflow/aetherflow/internal/session"
    guuid "github.com/Lzww0608/GUUID"
    "go.uber.org/zap"
)

func main() {
    // 创建日志记录器
    logger, _ := zap.NewProduction()
    
    // 创建存储层（开发环境使用内存存储）
    store := session.NewMemoryStore()
    
    // 创建会话管理器
    manager := session.NewManager(&session.ManagerConfig{
        Store:           store,
        Logger:          logger,
        DefaultTimeout:  30 * time.Minute,
        CleanupInterval: 5 * time.Minute,
    })
    defer manager.Close()
    
    ctx := context.Background()
    
    // 生成连接ID（来自Quantum协议）
    connID, _ := guuid.NewV7()
    
    // 创建会话
    sess, token, err := manager.CreateSession(
        ctx,
        "user123",              // 用户ID
        "192.168.1.100",        // 客户端IP
        54321,                  // 客户端端口
        connID,                 // Quantum连接ID
        map[string]string{      // 元数据
            "device": "mobile",
            "version": "1.0.0",
        },
    )
    if err != nil {
        logger.Fatal("failed to create session", zap.Error(err))
    }
    
    logger.Info("session created",
        zap.String("session_id", sess.SessionID.String()),
        zap.String("token", token))
    
    // 获取会话
    retrieved, err := manager.GetSession(ctx, sess.SessionID)
    if err != nil {
        logger.Fatal("failed to get session", zap.Error(err))
    }
    
    // 更新会话状态
    activeState := session.StateActive
    updated, err := manager.UpdateSession(
        ctx,
        sess.SessionID,
        &activeState,
        nil,
        nil,
    )
    if err != nil {
        logger.Fatal("failed to update session", zap.Error(err))
    }
    
    // 发送心跳
    remaining, err := manager.Heartbeat(ctx, sess.SessionID)
    if err != nil {
        logger.Fatal("failed to send heartbeat", zap.Error(err))
    }
    logger.Info("heartbeat successful",
        zap.Duration("remaining", remaining))
    
    // 列出所有会话
    sessions, total, err := manager.ListSessions(ctx, &session.SessionFilter{
        UserID: "user123",
        Limit:  10,
        Offset: 0,
    })
    if err != nil {
        logger.Fatal("failed to list sessions", zap.Error(err))
    }
    logger.Info("sessions listed",
        zap.Int("total", total),
        zap.Int("count", len(sessions)))
    
    // 删除会话
    err = manager.DeleteSession(ctx, sess.SessionID, "user logout")
    if err != nil {
        logger.Fatal("failed to delete session", zap.Error(err))
    }
}
```

### 集成Quantum协议

```go
// 当Quantum连接建立时创建会话
func handleNewConnection(conn *quantum.Connection) {
    session, token, err := sessionManager.CreateSession(
        context.Background(),
        conn.UserID(),
        conn.RemoteAddr().IP.String(),
        uint32(conn.RemoteAddr().Port),
        conn.GUID(),
        nil,
    )
    if err != nil {
        log.Error("failed to create session", zap.Error(err))
        return
    }
    
    // 将token发送给客户端
    conn.Send([]byte(token))
    
    // 定期更新统计信息
    go updateSessionStats(session.SessionID, conn)
}

func updateSessionStats(sessionID guuid.UUID, conn *quantum.Connection) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := conn.Stats()
        sessionManager.UpdateSession(
            context.Background(),
            sessionID,
            nil,
            nil,
            &session.Stats{
                PacketsSent:     stats.PacketsSent,
                PacketsReceived: stats.PacketsReceived,
                BytesSent:       stats.BytesSent,
                BytesReceived:   stats.BytesReceived,
                Retransmissions: stats.Retransmissions,
                CurrentRTTMs:    uint32(stats.RTT.Milliseconds()),
            },
        )
    }
}
```

## API接口

### gRPC服务定义

详见 `api/proto/session.proto`:

- `CreateSession` - 创建新会话
- `GetSession` - 获取会话信息
- `UpdateSession` - 更新会话
- `DeleteSession` - 删除会话
- `ListSessions` - 列出会话
- `Heartbeat` - 会话心跳

## 配置选项

### ManagerConfig

```go
type ManagerConfig struct {
    Store           Store           // 存储层实现
    Logger          *zap.Logger     // 日志记录器
    DefaultTimeout  time.Duration   // 默认会话超时时间
    CleanupInterval time.Duration   // 过期会话清理间隔
}
```

**默认值**:
- `DefaultTimeout`: 30分钟
- `CleanupInterval`: 5分钟

## 测试

### 运行所有测试

```bash
go test ./internal/session/... -v
```

### 运行特定测试

```bash
# 测试Manager
go test ./internal/session/... -v -run TestManager

# 测试Store
go test ./internal/session/... -v -run TestMemoryStore
```

### 测试覆盖率

```bash
go test ./internal/session/... -cover
```

## 性能考虑

### 内存存储（MemoryStore）

- **优点**:
  - 极快的读写性能（内存操作）
  - 零网络延迟
  - 适合开发和测试

- **缺点**:
  - 不支持分布式
  - 数据不持久化
  - 受限于单机内存

- **适用场景**:
  - 本地开发
  - 单元测试
  - 小规模部署

### etcd存储（待实现）

- **优点**:
  - 分布式一致性
  - 数据持久化
  - 支持Watch机制
  - 高可用

- **缺点**:
  - 网络延迟
  - 需要额外的etcd集群

- **适用场景**:
  - 生产环境
  - 分布式部署
  - 需要高可用

## 监控指标

### 关键指标

- `session_total` - 总会话数
- `session_active` - 活跃会话数
- `session_idle` - 空闲会话数
- `session_created_total` - 累计创建会话数
- `session_deleted_total` - 累计删除会话数
- `session_expired_total` - 累计过期会话数
- `session_duration_seconds` - 会话持续时间分布

### 日志示例

```
2025-11-07T22:30:46.660+0800 INFO session/manager.go:119 session created
    {"session_id": "019a5eba-1c84-7652-9ee0-f146556b141f",
     "user_id": "user123",
     "connection_id": "019a5eba-1c84-7651-958a-8a6c86b4c79f",
     "client_ip": "192.168.1.1"}

2025-11-07T22:30:46.760+0800 DEBUG session/manager.go:257 session heartbeat
    {"session_id": "019a5eba-1c84-765a-b1b6-ff95a0ad0e92",
     "remaining": "1m0s"}

2025-11-07T22:30:46.861+0800 INFO session/manager.go:328 expired sessions cleaned up
    {"count": 1}
```

## 最佳实践

### 1. 会话超时设置

根据应用场景调整超时时间：
- **实时协作**: 5-10分钟
- **长连接应用**: 30-60分钟
- **物联网设备**: 1-5分钟

### 2. 心跳间隔

建议心跳间隔为超时时间的1/3：
- 超时30分钟 → 心跳10分钟
- 超时10分钟 → 心跳3分钟

### 3. 清理策略

- 设置合理的清理间隔（建议5分钟）
- 在低峰期进行批量清理
- 记录清理的会话数量

### 4. 错误处理

- 会话不存在：返回明确的错误信息
- 会话过期：自动删除并通知客户端
- 网络错误：重试机制

### 5. 安全考虑

- 使用加密的会话令牌
- 定期轮换令牌
- 限制单用户会话数
- IP地址验证

## 参考资料

- [UUIDv7规范 (RFC 9562)](https://www.rfc-editor.org/rfc/rfc9562.html)
- [gRPC Best Practices](https://grpc.io/docs/guides/performance/)
- [etcd Documentation](https://etcd.io/docs/)
- [Zap Logger](https://github.com/uber-go/zap)


