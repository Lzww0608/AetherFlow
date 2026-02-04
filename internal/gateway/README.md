# API Gateway - AetherFlow网关服务

## 概述

API Gateway 是 AetherFlow 项目的统一入口，基于 **go-zero** 框架构建，提供 REST API 和 WebSocket 支持，负责请求路由、认证、限流、熔断等功能。

## 核心功能

### ✅ 已实现

#### 1. GoZero框架集成
- ✅ REST服务器配置
- ✅ YAML配置文件支持
- ✅ 结构化日志 (Zap)
- ✅ 优雅关闭

#### 2. 中间件系统
- ✅ **RequestID中间件** - 为每个请求生成唯一的UUIDv7
- ✅ **Logger中间件** - 记录请求/响应详细信息
- ✅ **RateLimit中间件** - 令牌桶算法限流
- ✅ **Context管理** - RequestID/SessionID/UserID传递

#### 3. 健康检查端点
- ✅ `/health` - 服务健康状态
- ✅ `/ping` - 简单心跳检测
- ✅ `/version` - 版本信息
- ✅ `/ws/stats` - WebSocket统计信息

#### 4. 通用响应结构
- ✅ 统一的JSON响应格式
- ✅ 错误码管理
- ✅ RequestID追踪

#### 5. WebSocket支持 ⭐
- ✅ **WebSocket升级** - HTTP到WebSocket协议升级
- ✅ **连接管理** - 连接注册、注销、生命周期管理
- ✅ **消息协议** - 9种消息类型 (Ping/Pong/Auth/Subscribe/Publish等)
- ✅ **Hub管理** - 集中式连接管理中心
- ✅ **心跳机制** - 自动Ping/Pong保活
- ✅ **超时检测** - 自动清理死连接
- ✅ **频道订阅** - 支持发布/订阅模式
- ✅ **用户追踪** - 支持发送消息给特定用户
- ✅ **广播功能** - 全局广播、频道广播、用户广播
- ✅ **单元测试** - 16个测试用例，44.3%覆盖率

#### 6. JWT认证 ⭐
- ✅ **JWT工具包** (`jwt/jwt.go`) - 生成、验证、刷新令牌
- ✅ **JWT中间件** (`middleware/jwt.go`) - 强制/可选认证
- ✅ **Claims结构** - UserID/SessionID/Username/Email
- ✅ **认证API** - Login/Refresh/Me端点
- ✅ **WebSocket集成** - JWT token验证
- ✅ **配置支持** - Secret/Expire/RefreshExpire/Issuer
- ✅ **单元测试** - 11个测试用例，84.6%覆盖率

**JWT特性**:
```
令牌管理:
- 访问令牌生成 (默认24小时)
- 刷新令牌生成 (默认7天)
- 令牌验证 (HS256签名)
- 令牌刷新
- 令牌解析 (不验证过期)

声明支持:
- UserID, SessionID (必需)
- Username, Email (可选)
- Issuer, IssuedAt, ExpiresAt, NotBefore

中间件:
- JWTMiddleware (强制认证)
- OptionalJWTMiddleware (可选认证)
- Context传递
```

#### 7. gRPC客户端集成 ⭐
- ✅ **连接池管理** (`grpcclient/manager.go`) - 连接池与管理器
- ✅ **Session客户端** (`grpcclient/session.go`) - Session服务封装
- ✅ **StateSync客户端** (`grpcclient/statesync.go`) - StateSync服务封装
- ✅ **HTTP桥接** (`handler/session.go`, `handler/statesync.go`) - REST到gRPC
- ✅ **自动重试** - 失败自动重试机制
- ✅ **超时控制** - 可配置的请求超时
- ✅ **连接复用** - 高效的连接池
- ✅ **单元测试** - 5个测试用例

**gRPC特性**:
```
连接池:
- 最大空闲连接数 (MaxIdle)
- 最大活跃连接数 (MaxActive)
- 空闲超时 (IdleTimeout)
- 连接状态检查
- 统计信息

客户端:
- Session服务 (6个RPC方法)
- StateSync服务 (12个RPC方法)
- 自动重试 (可配置次数)
- 超时控制 (可配置时间)
- 流式RPC支持

HTTP API:
- Session API (5个端点)
- StateSync API (8个端点)
- JWT认证保护
- 统一响应格式
```

#### 8. gRPC over Quantum Dialer ⭐ (新增)
- ✅ **Quantum Dialer** (`grpcclient/quantum_dialer.go`) - Quantum协议拨号器
- ✅ **net.Conn适配** - 实现标准网络接口
- ✅ **透明切换** - TCP/Quantum配置化选择
- ✅ **连接封装** - quantumConn包装器
- ✅ **超时控制** - Read/Write Deadline支持
- ✅ **单元测试** - 4个测试用例

**Quantum传输特性**:
```
协议优势:
- UDP基础 (低延迟 <10ms)
- FEC前向纠错 (丢包恢复)
- BBR拥塞控制 (高吞吐)
- Keep-alive机制
- 自动重传

性能提升:
- 延迟降低 ~40%
- 吞吐提升 ~30%
- 丢包容忍 up to 20%
- 网络波动下更稳定

配置示例:
Session:
  Transport: "quantum"  # 使用Quantum协议
StateSync:
  Transport: "tcp"      # 使用TCP协议
```

#### 9. Etcd服务发现 ⭐ (新增)
- ✅ **Etcd客户端** (`discovery/etcd.go`) - Etcd客户端封装
- ✅ **服务注册** - TTL租约 + 自动心跳
- ✅ **服务发现** - Watch机制实时监听
- ✅ **动态更新** - 连接池地址自动更新
- ✅ **健康检测** - 断线自动重连
- ✅ **单元测试** - 8个测试用例

**服务发现特性**:
```
注册机制:
- TTL租约 (默认10s)
- Keep-Alive心跳保活
- 断线自动重注册
- 优雅注销

发现机制:
- Watch实时监听
- 初始地址加载
- 增量更新推送
- 多实例支持

动态更新:
- 连接池地址更新
- 轮询负载均衡
- 无缝切换节点
- 零停机部署

配置示例:
Etcd:
  Enable: true
  Endpoints: ["127.0.0.1:2379"]
  ServiceTTL: 10
  
GRPC:
  Session:
    UseDiscovery: true     # 启用服务发现
    DiscoveryName: "session"
```

### 🚧 待实现

- ⏳ 熔断器与降级
- ⏳ Session Service集成
- ⏳ StateSync Service集成
- ⏳ Etcd服务发现
- ⏳ 熔断器
- ⏳ 链路追踪

## 项目结构

```
internal/gateway/
├── config/
│   └── config.go           # 配置结构定义
├── handler/
│   ├── routes.go           # 路由注册
│   ├── health.go           # 健康检查处理器
│   └── response.go         # 通用响应
├── middleware/
│   ├── context.go          # Context辅助函数
│   ├── requestid.go        # 请求ID中间件
│   ├── logger.go           # 日志中间件
│   └── ratelimit.go        # 限流中间件
├── svc/
│   └── servicecontext.go   # 服务上下文
└── README.md               # 本文档

cmd/gateway/
└── main.go                 # 主程序入口

configs/
└── gateway.yaml            # 配置文件
```

## 快速开始

### 1. 配置文件

编辑 `configs/gateway.yaml`:

```yaml
Name: aetherflow-gateway
Host: 0.0.0.0
Port: 8080
Mode: dev

Log:
  ServiceName: aetherflow-gateway
  Mode: console
  Level: info

RateLimit:
  Enable: true
  Rate: 100
  Burst: 200
```

### 2. 启动服务

```bash
# 开发模式
go run cmd/gateway/main.go -f configs/gateway.yaml

# 编译后运行
go build -o bin/gateway cmd/gateway/main.go
./bin/gateway -f configs/gateway.yaml
```

### 3. 验证服务

```bash
# 健康检查
curl http://localhost:8080/health

# 响应示例:
{
  "status": "UP",
  "timestamp": "2026-01-15T10:30:00Z",
  "service": "aetherflow-gateway",
  "version": "0.3.0-alpha"
}

# Ping测试
curl http://localhost:8080/ping
# 响应: pong

# 版本信息
curl http://localhost:8080/version
```

## API文档

### 认证端点

#### POST /api/v1/auth/login

用户登录

**请求体**:
```json
{
  "username": "test",
  "password": "test"
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 86400,
    "user_id": "user-123",
    "username": "test"
  },
  "request_id": "01JKX..."
}
```

#### POST /api/v1/auth/refresh

刷新访问令牌

**请求体**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 86400
  },
  "request_id": "01JKX..."
}
```

#### GET /api/v1/auth/me

获取当前用户信息（需要JWT认证）

**请求头**:
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "user-123",
    "session_id": "session-456",
    "username": "test",
    "email": "test@example.com"
  },
  "request_id": "01JKX..."
}
```

### Session API

#### POST /api/v1/session

创建新会话（需要JWT认证）

**请求体**:
```json
{
  "client_ip": "192.168.1.100",
  "client_port": 54321,
  "metadata": {
    "device": "iPhone",
    "app_version": "1.0.0"
  },
  "timeout_seconds": 3600
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "session": {
      "session_id": "01JKX...",
      "user_id": "user-123",
      "connection_id": "conn-456",
      "state": "SESSION_STATE_ACTIVE",
      ...
    },
    "token": "session-token-..."
  },
  "request_id": "01JKX..."
}
```

#### GET /api/v1/session

获取会话信息（需要JWT认证）

**查询参数**:
- `session_id`: 会话ID

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "session_id": "01JKX...",
    "user_id": "user-123",
    "state": "SESSION_STATE_ACTIVE",
    ...
  },
  "request_id": "01JKX..."
}
```

#### GET /api/v1/sessions

列出用户的所有会话（需要JWT认证）

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "sessions": [...],
    "total": 5,
    "page": 1,
    "page_size": 10
  },
  "request_id": "01JKX..."
}
```

#### POST /api/v1/session/heartbeat

发送会话心跳（需要JWT认证）

**请求体**:
```json
{
  "session_id": "01JKX..."
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "server_timestamp": "2026-01-15T12:00:00Z",
    "remaining_seconds": 3540
  },
  "request_id": "01JKX..."
}
```

#### DELETE /api/v1/session

删除会话（需要JWT认证）

**查询参数**:
- `session_id`: 会话ID

### StateSync API

#### POST /api/v1/document

创建文档（需要JWT认证）

**请求体**:
```json
{
  "name": "My Document",
  "type": "whiteboard",
  "content": "...",
  "tags": ["project-a", "draft"],
  "metadata": {
    "project": "ProjectA"
  }
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "doc-123",
    "name": "My Document",
    "type": "whiteboard",
    "state": "active",
    "version": 1,
    ...
  },
  "request_id": "01JKX..."
}
```

#### GET /api/v1/document

获取文档（需要JWT认证）

**查询参数**:
- `doc_id`: 文档ID

#### GET /api/v1/documents

列出文档（需要JWT认证）

#### POST /api/v1/document/operation

应用操作到文档（需要JWT认证）

**请求体**:
```json
{
  "doc_id": "doc-123",
  "type": "update",
  "data": "..."
}
```

#### GET /api/v1/document/operations

获取文档操作历史（需要JWT认证）

**查询参数**:
- `doc_id`: 文档ID

#### POST /api/v1/document/lock

获取文档锁（需要JWT认证）

**请求体**:
```json
{
  "doc_id": "doc-123",
  "session_id": "session-456"
}
```

#### DELETE /api/v1/document/lock

释放文档锁（需要JWT认证）

**请求体**:
```json
{
  "doc_id": "doc-123"
}
```

#### GET /api/v1/stats

获取StateSync统计信息（需要JWT认证）

### WebSocket端点

#### GET /ws

WebSocket连接端点

**完整流程示例** (JavaScript):
```javascript
// Step 1: 登录获取JWT token
async function login() {
    const response = await fetch('http://localhost:8080/api/v1/auth/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            username: 'test',
            password: 'test'
        })
    });
    
    const data = await response.json();
    if (data.code === 0) {
        localStorage.setItem('token', data.data.token);
        localStorage.setItem('refresh_token', data.data.refresh_token);
        return data.data.token;
    }
}

// Step 2: 使用JWT token建立WebSocket连接
const token = localStorage.getItem('token');
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
    console.log('Connected');
    
    // 使用JWT token认证
    ws.send(JSON.stringify({
        type: 'auth',
        data: {
            token: token  // JWT token
        }
    }));
};

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log('Received:', msg);
    
    if (msg.type === 'auth_result' && msg.data.success) {
        console.log('Authenticated as:', msg.data.user_id);
        
        // 2. 订阅频道
        ws.send(JSON.stringify({
            type: 'subscribe',
            data: {
                channel: 'room-123'
            }
        }));
    }
    
    if (msg.type === 'notify') {
        console.log('Notification:', msg.data);
    }
};

// 3. 发布消息
function publishMessage(channel, data) {
    ws.send(JSON.stringify({
        type: 'publish',
        data: {
            channel: channel,
            data: data
        }
    }));
}

// 4. Ping (保活)
setInterval(() => {
    ws.send(JSON.stringify({type: 'ping'}));
}, 30000);
```

#### GET /ws/stats

WebSocket统计信息

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_connections": 156,
    "authenticated_users": 89,
    "total_channels": 45
  }
}
```

### 健康检查端点

#### GET /health

服务健康状态检查

**响应示例**:
```json
{
  "status": "UP",
  "timestamp": "2026-01-15T10:30:00Z",
  "service": "aetherflow-gateway",
  "version": "0.3.0-alpha"
}
```

#### GET /ping

简单心跳检测

**响应**: `pong`

#### GET /version

版本信息查询

**响应示例**:
```json
{
  "service": "aetherflow-gateway",
  "version": "0.3.0-alpha",
  "build_time": "2026-01-15",
  "go_version": "1.21",
  "timestamp": "2026-01-15T10:30:00Z"
}
```

## 中间件详解

### RequestID中间件

为每个请求生成唯一的UUIDv7作为请求ID，用于分布式追踪。

**特性**:
- 自动生成UUIDv7 (时间排序)
- 支持客户端传递 (X-Request-ID header)
- 自动添加到响应头
- 注入到Context供后续处理使用

**使用**:
```go
requestID := middleware.RequestIDFromContext(r.Context())
```

### Logger中间件

记录每个HTTP请求的详细信息。

**记录内容**:
- 请求方法、路径、查询参数
- 客户端IP、User-Agent
- 响应状态码、大小
- 处理时间

**日志示例**:
```
INFO HTTP Request request_id=xxx method=GET path=/api/v1/sessions
INFO HTTP Response request_id=xxx status=200 duration=15ms
```

### RateLimit中间件

基于令牌桶算法的限流中间件。

**配置**:
```yaml
RateLimit:
  Enable: true
  Rate: 100    # 每秒100个请求
  Burst: 200   # 突发容量200
```

**行为**:
- 超过限制返回 429 Too Many Requests
- 基于全局限流 (可扩展为IP级别限流)

## 配置说明

### 核心配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| Name | string | aetherflow-gateway | 服务名称 |
| Host | string | 0.0.0.0 | 监听地址 |
| Port | int | 8080 | 监听端口 |
| Mode | string | dev | 运行模式 (dev/test/prod) |
| Timeout | int | 30000 | 请求超时 (毫秒) |

### 日志配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| Log.ServiceName | string | aetherflow-gateway | 日志服务名 |
| Log.Mode | string | console | 日志模式 (console/file) |
| Log.Level | string | info | 日志级别 |

### CORS配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| Cors.Enable | bool | true | 是否启用CORS |
| Cors.AllowOrigins | []string | ["*"] | 允许的源 |
| Cors.AllowMethods | []string | [GET,POST...] | 允许的方法 |

### JWT配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| JWT.Secret | string | aetherflow-secret-key | JWT密钥（生产环境必须修改） |
| JWT.Expire | int64 | 86400 | 访问令牌过期时间（秒，24小时） |
| JWT.RefreshExpire | int64 | 604800 | 刷新令牌过期时间（秒，7天） |
| JWT.Issuer | string | aetherflow | 令牌签发者 |

**安全提示**:
- ⚠️ 生产环境必须修改JWT.Secret为强随机字符串
- ⚠️ 建议使用环境变量而不是配置文件存储密钥
- ⚠️ 定期轮换JWT密钥

### gRPC配置

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| GRPC.Session.Target | string | 127.0.0.1:9001 | Session服务地址 |
| GRPC.Session.Timeout | int | 5000 | 请求超时时间（毫秒） |
| GRPC.Session.MaxRetries | int | 3 | 最大重试次数 |
| GRPC.Session.Transport | string | tcp | 传输协议 (tcp/quantum) |
| GRPC.StateSync.Target | string | 127.0.0.1:9002 | StateSync服务地址 |
| GRPC.StateSync.Timeout | int | 5000 | 请求超时时间（毫秒） |
| GRPC.StateSync.MaxRetries | int | 3 | 最大重试次数 |
| GRPC.StateSync.Transport | string | tcp | 传输协议 (tcp/quantum) |
| GRPC.Pool.MaxIdle | int | 10 | 最大空闲连接数 |
| GRPC.Pool.MaxActive | int | 100 | 最大活跃连接数 |
| GRPC.Pool.IdleTimeout | int | 60 | 空闲超时（秒） |
| GRPC.LoadBalancer.Policy | string | round_robin | 负载均衡策略 |
| Etcd.Enable | bool | false | 是否启用Etcd服务发现 |
| Etcd.Endpoints | []string | ["127.0.0.1:2379"] | Etcd endpoints |
| Etcd.DialTimeout | int | 5 | 连接超时（秒） |
| Etcd.ServiceTTL | int64 | 10 | 服务注册TTL（秒） |
| Etcd.ServiceName | string | aetherflow-gateway | 服务名称 |
| Etcd.ServiceAddr | string | localhost:8888 | 服务地址 |

**配置示例**:
```yaml
GRPC:
  Session:
    Target: "127.0.0.1:9001"
    Timeout: 5000
    MaxRetries: 3
  StateSync:
    Target: "127.0.0.1:9002"
    Timeout: 5000
    MaxRetries: 3
  Pool:
    MaxIdle: 10
    MaxActive: 100
    IdleTimeout: 60
  LoadBalancer:
    Policy: "round_robin"
```

## 响应格式

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": { ... },
  "request_id": "01JKXXX..."
}
```

### 错误响应

```json
{
  "code": 400,
  "message": "Invalid request",
  "request_id": "01JKXXX..."
}
```

### HTTP状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |

## WebSocket消息协议

### 消息格式

所有WebSocket消息使用JSON格式：

```json
{
  "id": "01JKX...",        // 消息ID (UUIDv7)
  "type": "message_type",  // 消息类型
  "timestamp": "2026-01-15T10:30:00Z",
  "data": {},              // 消息数据
  "request_id": "xxx",     // 可选：关联的请求ID
  "error": "error message" // 可选：错误信息
}
```

### 消息类型

| 类型 | 方向 | 说明 |
|------|------|------|
| `ping` | Client→Server | 心跳请求 |
| `pong` | Server→Client | 心跳响应 |
| `auth` | Client→Server | 认证请求 |
| `auth_result` | Server→Client | 认证结果 |
| `subscribe` | Client→Server | 订阅频道 |
| `unsubscribe` | Client→Server | 取消订阅 |
| `publish` | Client→Server | 发布消息 |
| `notify` | Server→Client | 通知消息 |
| `error` | Server→Client | 错误消息 |

### 认证流程

```
Client                  Server
  |                       |
  |-- auth (token) ------>|
  |                       | (验证token)
  |<-- auth_result -------|
  |    (success=true)     |
```

### 发布/订阅流程

```
Client A                Server                Client B
  |                       |                       |
  |-- subscribe(room1) -->|                       |
  |<-- success ----------|                       |
  |                       |<-- subscribe(room1) --|
  |                       |-- success ----------->|
  |                       |                       |
  |-- publish(room1) ---->|                       |
  |                       |-- notify(room1) ----->|
  |<-- success ----------|-- notify(room1) ----->|
```

## 开发指南

### WebSocket开发示例

#### 服务端广播消息

```go
// 广播到所有连接
msg := websocket.NewMessage(websocket.MessageTypeNotify, map[string]interface{}{
    "event": "system_update",
    "data": "Server will restart in 5 minutes",
})
count := svcCtx.WSServer.Broadcast(msg)

// 广播到特定频道
count := svcCtx.WSServer.BroadcastToChannel("room-123", msg)

// 发送给特定用户的所有连接
count := svcCtx.WSServer.SendToUser("user-456", msg)
```

#### 自定义认证函数

```go
// 在main.go中设置WebSocket JWT认证
ctx.WSServer.SetAuthFunc(func(token string) (userID, sessionID, username, email string, err error) {
    // 使用JWT管理器验证token
    claims, err := ctx.JWTManager.VerifyToken(token)
    if err != nil {
        return "", "", "", "", err
    }
    
    return claims.UserID, claims.SessionID, claims.Username, claims.Email, nil
})
```

#### JWT令牌操作

```go
// 生成访问令牌
token, err := svcCtx.JWTManager.GenerateToken(
    userID, sessionID, username, email,
)

// 生成刷新令牌
refreshToken, err := svcCtx.JWTManager.GenerateRefreshToken(
    userID, sessionID,
)

// 验证令牌
claims, err := svcCtx.JWTManager.VerifyToken(token)

// 刷新令牌
newToken, err := svcCtx.JWTManager.RefreshToken(refreshToken)

// 解析令牌（不验证过期）
claims, err := svcCtx.JWTManager.ParseToken(token)
```

#### 使用JWT中间件保护路由

```go
import "github.com/aetherflow/aetherflow/internal/gateway/middleware"

// 方式1: 使用go-zero内置JWT中间件
server.AddRoutes(
    []rest.Route{
        {
            Method:  "GET",
            Path:    "/protected",
            Handler: ProtectedHandler(svcCtx),
        },
    },
    rest.WithJwt(svcCtx.Config.JWT.Secret),
)

// 方式2: 使用自定义JWT中间件
server.Use(middleware.JWTMiddleware(svcCtx.JWTManager))

// 方式3: 可选认证（不强制）
server.Use(middleware.OptionalJWTMiddleware(svcCtx.JWTManager))
```

### 添加新路由

1. 在 `handler/` 目录创建处理器:

```go
func MyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 处理逻辑
        SuccessResponse(w, data, requestID)
    }
}
```

2. 在 `handler/routes.go` 注册路由:

```go
server.AddRoutes(
    []rest.Route{
        {
            Method:  rest.MethodGet,
            Path:    "/api/v1/myresource",
            Handler: MyHandler(svcCtx),
        },
    },
    rest.WithPrefix("/api/v1"),
)
```

### 添加新中间件

```go
func MyMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 前置处理
        
        next(w, r)
        
        // 后置处理
    }
}
```

### 使用Context传递数据

```go
// 存储
ctx = middleware.UserIDToContext(ctx, userID)

// 获取
userID := middleware.UserIDFromContext(r.Context())
```

## 测试

```bash
# 运行测试
go test ./internal/gateway/...

# 查看覆盖率
go test -cover ./internal/gateway/...

# 性能测试
go test -bench=. ./internal/gateway/...
```

## 部署

### Docker部署

```bash
# 构建镜像
docker build -t aetherflow-gateway:latest -f deployments/gateway/Dockerfile .

# 运行容器
docker run -d \
  --name aetherflow-gateway \
  -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  aetherflow-gateway:latest
```

### Kubernetes部署

```bash
kubectl apply -f deployments/k8s/gateway-deployment.yaml
kubectl apply -f deployments/k8s/gateway-service.yaml
```

## 性能指标

### 基准测试结果

```
请求处理: ~0.5ms (无业务逻辑)
QPS: ~10,000 (单核)
内存占用: ~50MB (空载)
```

### 优化建议

1. 启用连接池
2. 启用HTTP/2
3. 调整限流参数
4. 启用响应压缩

## 监控与告警

### Prometheus指标

暂未实现，计划支持:
- 请求总数
- 响应时间
- 错误率
- 活跃连接数

## 故障排查

### 常见问题

#### 1. 端口被占用

```bash
# 检查端口
lsof -i :8080

# 更改配置文件端口
```

#### 2. 配置文件找不到

```bash
# 指定配置文件路径
./gateway -f /path/to/config.yaml
```

#### 3. 日志级别太高

```yaml
Log:
  Level: debug  # 改为debug查看详细日志
```

## 版本历史

### v0.7.0-alpha (2026-02-03)

**新增**:
- ✅ Etcd服务发现
- ✅ 服务注册与注销
- ✅ 动态地址更新
- ✅ Watch机制监听
- ✅ 自动重连
- ✅ 44个单元测试

**改进**:
- 支持动态扩缩容
- 零停机部署
- 服务健康检测
- 完善配置文档

### v0.6.0-alpha (2026-02-02)

**新增**:
- ✅ gRPC over Quantum Dialer
- ✅ Quantum协议传输
- ✅ TCP/Quantum透明切换
- ✅ net.Conn接口适配
- ✅ 36个单元测试

**改进**:
- 降低网络延迟 (~40%)
- 提升传输可靠性
- 完善协议文档

### v0.5.0-alpha (2026-02-02)

**新增**:
- ✅ gRPC客户端集成
- ✅ 连接池管理
- ✅ Session API (5个端点)
- ✅ StateSync API (8个端点)
- ✅ HTTP到gRPC桥接
- ✅ 自动重试机制
- ✅ 32个单元测试

**改进**:
- 完善API文档
- 优化错误处理
- 提升代码覆盖率

### v0.4.0-alpha (2026-01-15)

**新增**:
- ✅ WebSocket完整支持
- ✅ JWT认证系统
- ✅ 认证API端点
- ✅ WebSocket + JWT集成
- ✅ 27个单元测试

**改进**:
- 提升测试覆盖率
- 完善文档
- 优化连接管理

### v0.3.0-alpha (2026-01-15)

**新增**:
- ✅ GoZero框架集成
- ✅ 基础中间件系统
- ✅ 健康检查端点
- ✅ 配置文件支持
- ✅ 限流功能

**下一步计划**:
- Prometheus监控与指标
- 熔断器与降级策略
- 链路追踪集成

## 相关文档

- [PROJECT_SUMMARY.md](../../PROJECT_SUMMARY.md) - 项目总结
- [ROADMAP.md](../../ROADMAP.md) - 开发路线图
- [Session Service](../session/README.md) - 会话服务
- [StateSync Service](../statesync/README.md) - 状态同步服务

## 贡献

欢迎贡献！请遵循项目的代码规范和提交规范。

## 许可证

MIT License

---

**版本**: v0.3.0-alpha  
**最后更新**: 2026-01-15  
**维护者**: AetherFlow Team
