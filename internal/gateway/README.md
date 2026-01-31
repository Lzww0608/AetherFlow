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

#### 5. WebSocket支持 ⭐ (新增)
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

### 🚧 待实现

- ⏳ JWT认证中间件
- ⏳ gRPC客户端连接池
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

### WebSocket端点

#### GET /ws

WebSocket连接端点

**连接示例** (JavaScript):
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
    console.log('Connected');
    
    // 1. 认证
    ws.send(JSON.stringify({
        type: 'auth',
        data: {
            token: 'your-jwt-token'
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
// 在main.go中设置认证函数
svcCtx.WSServer.SetAuthFunc(func(token string) (userID, sessionID string, err error) {
    // 验证JWT token
    claims, err := verifyJWT(token)
    if err != nil {
        return "", "", err
    }
    
    return claims.UserID, claims.SessionID, nil
})
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

### v0.3.0-alpha (2026-01-15)

**新增**:
- ✅ GoZero框架集成
- ✅ 基础中间件系统
- ✅ 健康检查端点
- ✅ 配置文件支持
- ✅ 限流功能

**下一步计划**:
- JWT认证
- WebSocket支持
- gRPC集成

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
