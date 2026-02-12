# gRPC 服务实现指南

本文档详细介绍 AetherFlow 项目中 Session Service 和 StateSync Service 的 gRPC 服务器实现。

## 📋 目录

- [概述](#概述)
- [架构设计](#架构设计)
- [Session Service](#session-service)
- [StateSync Service](#statesync-service)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [测试指南](#测试指南)
- [部署方案](#部署方案)
- [故障排查](#故障排查)

## 概述

### 实现的服务

#### 1. Session Service（会话服务）
- **端口**: 9001 (gRPC), 9101 (Metrics)
- **职责**: 管理用户会话和连接状态
- **存储**: 内存存储（Memory Store）/ Redis（待实现）
- **功能**:
  - 会话创建、获取、更新、删除
  - 会话心跳保活
  - 会话列表查询
  - 会话统计信息

#### 2. StateSync Service（状态同步服务）
- **端口**: 9002 (gRPC), 9102 (Metrics)
- **职责**: 管理实时协作文档和操作同步
- **存储**: 内存存储（Memory Store）/ PostgreSQL（待实现）
- **功能**:
  - 文档 CRUD 操作
  - 操作应用与历史查询
  - 文档订阅（流式）
  - 文档锁管理
  - 冲突检测与解决
  - 统计信息

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                         Gateway                              │
│                      (HTTP/WebSocket)                        │
│                         :8080                                │
└────────────────┬──────────────────────┬─────────────────────┘
                 │                      │
          gRPC   │                      │   gRPC
                 ▼                      ▼
    ┌───────────────────┐    ┌────────────────────┐
    │ Session Service   │    │ StateSync Service  │
    │                   │    │                    │
    │   :9001 (gRPC)   │    │   :9002 (gRPC)    │
    │   :9101 (Metrics)│    │   :9102 (Metrics) │
    └─────────┬─────────┘    └─────────┬──────────┘
              │                        │
              ▼                        ▼
       ┌─────────────┐          ┌─────────────┐
       │   Redis     │          │ PostgreSQL  │
       │  (Session)  │          │ (StateSync) │
       └─────────────┘          └─────────────┘
```

### 服务目录结构

```
cmd/
├── session-service/
│   ├── main.go                 # 服务入口
│   ├── config/
│   │   └── config.go           # 配置定义
│   └── server/
│       ├── server.go           # gRPC 服务器
│       └── handler.go          # 业务处理
│
├── statesync-service/
│   ├── main.go                 # 服务入口
│   ├── config/
│   │   └── config.go           # 配置定义
│   └── server/
│       ├── server.go           # gRPC 服务器
│       ├── handler.go          # 业务处理
│       └── stream_handler.go   # 流式处理
│
configs/
├── session.yaml                # Session Service 配置
└── statesync.yaml              # StateSync Service 配置
```

## Session Service

### 服务接口

#### 1. CreateSession - 创建会话

```protobuf
rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse);
```

**请求示例**:
```json
{
  "user_id": "user-001",
  "client_ip": "192.168.1.100",
  "client_port": 12345,
  "metadata": {
    "device": "laptop",
    "os": "linux"
  },
  "timeout_seconds": 1800
}
```

**响应示例**:
```json
{
  "session": {
    "session_id": "01HXXX...",
    "user_id": "user-001",
    "state": "SESSION_STATE_ACTIVE",
    "created_at": "2024-01-15T10:30:00Z"
  },
  "token": "abc123..."
}
```

#### 2. GetSession - 获取会话

```protobuf
rpc GetSession(GetSessionRequest) returns (GetSessionResponse);
```

#### 3. UpdateSession - 更新会话

```protobuf
rpc UpdateSession(UpdateSessionRequest) returns (UpdateSessionResponse);
```

#### 4. DeleteSession - 删除会话

```protobuf
rpc DeleteSession(DeleteSessionRequest) returns (DeleteSessionResponse);
```

#### 5. ListSessions - 列出会话

```protobuf
rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);
```

#### 6. Heartbeat - 会话心跳

```protobuf
rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
```

### 配置文件

**configs/session.yaml**:
```yaml
Server:
  Host: 0.0.0.0
  Port: 9001

Store:
  Type: memory  # memory, redis
  Redis:
    Addr: localhost:6379
    Password: ""
    DB: 0

Log:
  Level: info
  Format: json

Metrics:
  Enable: true
  Port: 9101

Tracing:
  Enable: false
  ServiceName: session-service
```

## StateSync Service

### 服务接口

#### 文档管理

1. **CreateDocument** - 创建文档
2. **GetDocument** - 获取文档
3. **UpdateDocument** - 更新文档
4. **DeleteDocument** - 删除文档
5. **ListDocuments** - 列出文档

#### 操作管理

6. **ApplyOperation** - 应用操作
7. **GetOperationHistory** - 获取操作历史

#### 订阅管理

8. **SubscribeDocument** - 订阅文档（流式）
9. **UnsubscribeDocument** - 取消订阅

#### 锁管理

10. **AcquireLock** - 获取锁
11. **ReleaseLock** - 释放锁
12. **IsLocked** - 检查锁

#### 统计信息

13. **GetStats** - 获取统计信息

### 流式订阅示例

```go
// 客户端订阅文档更新
stream, err := client.SubscribeDocument(ctx, &pb.SubscribeDocumentRequest{
    DocId:     docID,
    UserId:    userID,
    SessionId: sessionID,
})

// 接收事件
for {
    event, err := stream.Recv()
    if err != nil {
        break
    }
    
    switch event.Type {
    case "operation_applied":
        // 处理操作应用事件
    case "user_joined":
        // 处理用户加入事件
    case "conflict_detected":
        // 处理冲突检测事件
    }
}
```

## 快速开始

### 1. 编译服务

```bash
# 编译所有服务
make build

# 或分别编译
make build-session
make build-statesync
```

### 2. 启动服务

```bash
# 方式 1: 使用启动脚本（推荐）
./scripts/start-all.sh

# 方式 2: 手动启动
./bin/session-service -f configs/session.yaml
./bin/statesync-service -f configs/statesync.yaml
./bin/gateway -f configs/gateway.yaml
```

### 3. 验证服务

```bash
# 检查服务状态
./scripts/status.sh

# 运行端到端测试
./scripts/test-grpc.sh
```

### 4. 停止服务

```bash
./scripts/stop-all.sh
```

## 配置说明

### Session Service 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `Server.Host` | string | `0.0.0.0` | 监听地址 |
| `Server.Port` | int | `9001` | gRPC 端口 |
| `Store.Type` | string | `memory` | 存储类型 (memory/redis) |
| `Store.Redis.Addr` | string | `localhost:6379` | Redis 地址 |
| `Log.Level` | string | `info` | 日志级别 |
| `Metrics.Enable` | bool | `true` | 是否启用指标 |
| `Tracing.Enable` | bool | `false` | 是否启用追踪 |

### StateSync Service 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `Server.Host` | string | `0.0.0.0` | 监听地址 |
| `Server.Port` | int | `9002` | gRPC 端口 |
| `Store.Type` | string | `memory` | 存储类型 (memory/postgres) |
| `Manager.LockTimeout` | duration | `30s` | 锁超时时间 |
| `Manager.AutoResolveConflicts` | bool | `true` | 自动解决冲突 |

## 测试指南

### 使用 grpcurl 测试

#### 1. 安装 grpcurl

```bash
# macOS
brew install grpcurl

# Linux/Go
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

#### 2. 列出服务

```bash
# Session Service
grpcurl -plaintext localhost:9001 list

# StateSync Service
grpcurl -plaintext localhost:9002 list
```

#### 3. 测试接口

**创建会话**:
```bash
grpcurl -plaintext -d '{
  "user_id": "test-user",
  "client_ip": "127.0.0.1",
  "client_port": 8888
}' localhost:9001 session.SessionService/CreateSession
```

**创建文档**:
```bash
grpcurl -plaintext -d '{
  "name": "Test Doc",
  "type": "whiteboard",
  "created_by": "test-user"
}' localhost:9002 aetherflow.statesync.StateSyncService/CreateDocument
```

### 自动化测试脚本

运行完整的端到端测试：

```bash
./scripts/test-grpc.sh
```

测试覆盖：
- ✅ Session Service 6 个接口
- ✅ StateSync Service 9 个接口
- ✅ 健康检查
- ✅ 流式订阅

## 部署方案

### 方案 1: 本地开发

```bash
# 编译并运行
make build
./scripts/start-all.sh
```

### 方案 2: Docker Compose

```bash
# 启动所有服务（包括 Prometheus、Grafana）
docker-compose -f deployments/docker-compose.services.yml up -d

# 查看日志
docker-compose -f deployments/docker-compose.services.yml logs -f

# 停止
docker-compose -f deployments/docker-compose.services.yml down
```

### 方案 3: Kubernetes（待完善）

待实现 Kubernetes Deployment 和 Service 配置。

## 故障排查

### 问题 1: 服务启动失败

**症状**: 无法启动服务，端口被占用

**解决方案**:
```bash
# 检查端口占用
lsof -i :9001
lsof -i :9002

# 杀死占用进程
kill -9 <PID>
```

### 问题 2: gRPC 连接失败

**症状**: Gateway 无法连接到 Session/StateSync Service

**排查步骤**:
```bash
# 1. 检查服务是否运行
./scripts/status.sh

# 2. 测试 gRPC 连接
grpcurl -plaintext localhost:9001 grpc.health.v1.Health/Check

# 3. 检查配置
cat configs/gateway.yaml | grep -A 2 "SessionService\|StateSyncService"
```

### 问题 3: 内存占用过高

**症状**: 服务内存持续增长

**排查步骤**:
```bash
# 1. 查看指标
curl http://localhost:9101/metrics | grep memory

# 2. 检查会话/文档数量
curl http://localhost:9101/metrics | grep total

# 3. 调整清理间隔（configs/*.yaml）
Manager:
  CleanupInterval: 1m  # 缩短清理间隔
```

### 问题 4: 流式订阅中断

**症状**: SubscribeDocument 连接频繁断开

**可能原因**:
1. 网络不稳定
2. 超时配置过短
3. 服务重启

**解决方案**:
- 实现客户端重连机制
- 增加心跳检测
- 调整超时配置

## 监控与可观测性

### Prometheus 指标

**Session Service Metrics** (`http://localhost:9101/metrics`):
- `session_total`: 会话总数
- `session_active`: 活跃会话数
- `session_heartbeat_total`: 心跳总数

**StateSync Service Metrics** (`http://localhost:9102/metrics`):
- `statesync_documents_total`: 文档总数
- `statesync_operations_total`: 操作总数
- `statesync_conflicts_total`: 冲突总数
- `statesync_subscribers_active`: 活跃订阅者数

### 日志

日志文件位置:
- Gateway: `logs/gateway.log`
- Session Service: `logs/session.log`
- StateSync Service: `logs/statesync.log`

实时查看日志:
```bash
tail -f logs/session.log
tail -f logs/statesync.log
```

## 下一步计划

### P0 优先级
- [ ] 实现 Redis Store for Session Service
- [ ] 实现 PostgreSQL Store for StateSync Service

### P1 优先级
- [ ] 完善端到端测试覆盖
- [ ] 添加压力测试
- [ ] 实现服务间认证

### P2 优先级
- [ ] Kubernetes 部署配置
- [ ] 服务熔断与降级
- [ ] 分布式追踪增强

## 参考资料

- [Session Service Proto 定义](../api/proto/session.proto)
- [StateSync Service Proto 定义](../api/proto/statesync.proto)
- [Session Manager 实现](../internal/session/manager.go)
- [StateSync Manager 实现](../internal/statesync/manager.go)
- [gRPC 官方文档](https://grpc.io/docs/)
- [Go gRPC 最佳实践](https://github.com/grpc/grpc-go/blob/master/Documentation/bestpractices.md)
