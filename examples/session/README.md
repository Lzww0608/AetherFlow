# Session Service 示例

本示例演示了如何使用AetherFlow的会话管理服务（Session Service）。

## 功能演示

这个示例程序展示了以下功能：

### 1. 创建会话
- 为多个用户创建会话
- 生成会话ID和认证令牌
- 关联Quantum连接ID
- 设置会话元数据

### 2. 查询会话
- 按会话ID查询
- 按连接ID查询
- 按用户ID查询
- 列出所有会话

### 3. 更新会话
- 更新会话状态
- 更新元数据
- 更新统计信息

### 4. 心跳保活
- 定期发送心跳
- 延长会话有效期
- 检查剩余时间

### 5. 会话管理
- 删除会话
- 自动过期清理
- 获取系统统计

## 运行示例

```bash
cd examples/session
go run main.go
```

## 示例输出

```
2025-11-07T22:30:00.000+0800 INFO === AetherFlow Session Service 示例 ===

--- 场景1: 创建用户会话 ---
INFO session/manager.go:119 session created {"session_id": "...", "user_id": "user1", ...}
INFO 会话创建成功 {"user_id": "user1", "session_id": "...", "token": "..."}

--- 场景2: 查询会话 ---
INFO 会话信息 {"user_id": "user1", "state": "CONNECTING", ...}

--- 场景3: 更新会话状态 ---
INFO 会话状态已更新 {"session_id": "...", "new_state": "ACTIVE", ...}

--- 场景4: 会话心跳 ---
DEBUG session/manager.go:257 session heartbeat {"session_id": "...", "remaining": "5m0s"}
INFO 心跳成功 {"heartbeat_count": 1, "remaining": "5m0s"}

--- 场景5: 列出所有会话 ---
INFO 会话列表 {"total": 3, "count": 3}

--- 场景6: 按用户查询 ---
INFO 用户会话 {"user_id": "user1", "session_count": 1}

--- 场景7: 获取统计信息 ---
INFO 系统统计 {"total_sessions": 3, "active_sessions": 1, "idle_sessions": 0}

--- 场景8: 按连接ID查询 ---
INFO 通过连接ID找到会话 {"connection_id": "...", "session_id": "...", "user_id": "user2"}

--- 场景9: 删除会话 ---
INFO session/manager.go:209 session deleted {"session_id": "...", "user_id": "user3", "reason": "user logout"}
INFO 会话已删除 {"user_id": "user3"}
INFO 删除后剩余会话 {"count": 2}

--- 场景10: 会话过期演示 ---
INFO 创建一个短暂的会话...
INFO 临时会话已创建，等待过期... {"session_id": "..."}
INFO session/manager.go:328 expired sessions cleaned up {"count": 1}
INFO 会话已过期并被清理 {"error": "session not found: ..."}

=== 示例完成 ===
```

## 核心概念

### Session（会话）
每个会话代表一个用户的连接，包含：
- **SessionID**: UUIDv7格式的唯一标识
- **UserID**: 用户标识符
- **ConnectionID**: Quantum协议连接ID
- **State**: 会话状态（CONNECTING, ACTIVE, IDLE等）
- **Metadata**: 自定义元数据
- **Stats**: 连接统计信息

### SessionManager（会话管理器）
负责会话的完整生命周期管理：
- 创建和注册新会话
- 查询和更新会话
- 心跳保活
- 自动过期清理

### Store（存储层）
抽象的存储接口，支持多种实现：
- **MemoryStore**: 内存存储（开发/测试）
- **etcdStore**: etcd分布式存储（生产环境，待实现）

## 实际应用场景

### 1. 实时协作应用
```go
// 用户连接时
session, token, _ := manager.CreateSession(ctx, userID, clientIP, port, connID, nil)
// 返回token给客户端用于后续认证

// 协作操作时更新状态
manager.UpdateSession(ctx, sessionID, &activeState, nil, stats)

// 定期心跳
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        manager.Heartbeat(ctx, sessionID)
    }
}()
```

### 2. 在线状态管理
```go
// 获取某个用户的所有会话
sessions, total, _ := manager.ListSessions(ctx, &SessionFilter{
    UserID: userID,
})

// 检查用户是否在线
isOnline := total > 0

// 获取用户的设备列表
for _, sess := range sessions {
    device := sess.Metadata["device"]
    // 处理多设备场景
}
```

### 3. 连接监控
```go
// 获取系统统计
stats, _ := manager.GetStats(ctx)
fmt.Printf("在线用户: %d\n", stats["active"])

// 列出所有活跃会话
activeSessions, _, _ := manager.ListSessions(ctx, &SessionFilter{
    State: &StateActive,
})

// 监控异常连接
for _, sess := range activeSessions {
    if time.Since(sess.LastActiveAt) > 10*time.Minute {
        // 标记为空闲或断开
        idleState := StateIdle
        manager.UpdateSession(ctx, sess.SessionID, &idleState, nil, nil)
    }
}
```

## 最佳实践

### 1. 合理设置超时时间
根据应用类型选择合适的超时：
```go
// 实时游戏: 短超时
config := &ManagerConfig{
    DefaultTimeout: 5 * time.Minute,
}

// 文档协作: 中等超时
config := &ManagerConfig{
    DefaultTimeout: 30 * time.Minute,
}

// 长连接应用: 长超时
config := &ManagerConfig{
    DefaultTimeout: 2 * time.Hour,
}
```

### 2. 实现心跳机制
客户端应定期发送心跳：
```go
heartbeatInterval := timeout / 3  // 超时时间的1/3
```

### 3. 处理会话恢复
支持断线重连：
```go
// 尝试获取现有会话
existingSessions, _ := manager.ListSessions(ctx, &SessionFilter{
    UserID: userID,
})

if len(existingSessions) > 0 {
    // 恢复现有会话
    session = existingSessions[0]
} else {
    // 创建新会话
    session, _, _ = manager.CreateSession(...)
}
```

### 4. 会话清理
在应用关闭时清理会话：
```go
defer func() {
    // 删除所有会话
    sessions, _, _ := manager.ListSessions(ctx, &SessionFilter{})
    for _, sess := range sessions {
        manager.DeleteSession(ctx, sess.SessionID, "server shutdown")
    }
    manager.Close()
}()
```

## 相关文档

- [Session Service 完整文档](../../internal/session/README.md)
- [gRPC API定义](../../api/proto/session.proto)
- [Quantum协议文档](../../docs/QUANTUM_IMPLEMENTATION.md)


