# 链路追踪模块

## 概述

AetherFlow 链路追踪模块基于 OpenTelemetry 实现，支持 Jaeger 和 Zipkin 两种导出器，提供完整的分布式追踪能力。

## 功能特性

### 核心功能
- ✅ **OpenTelemetry 集成** - 使用业界标准的追踪框架
- ✅ **多导出器支持** - Jaeger / Zipkin
- ✅ **可配置采样** - 支持全采样、不采样、比例采样
- ✅ **自动上下文传播** - W3C Trace Context + Baggage
- ✅ **HTTP 追踪** - 自动追踪 HTTP 请求
- ✅ **gRPC 追踪** - 自动追踪 gRPC 调用（一元和流式）
- ✅ **批量发送** - 减少网络开销
- ✅ **优雅关闭** - 确保数据完整性

### 追踪信息
- Trace ID - 全局唯一追踪标识
- Span ID - 单个操作标识
- 父子关系 - Span 之间的调用关系
- 属性 - HTTP 方法、路径、状态码等
- 事件 - 操作中的关键事件
- 错误 - 自动记录异常信息

## 架构设计

### 组件结构

```
tracing/
├── tracer.go              # 追踪器核心实现
├── tracer_test.go         # 单元测试
├── README.md              # 本文档
│
middleware/
├── tracing.go             # HTTP 追踪中间件
│
grpcclient/
├── tracing_interceptor.go # gRPC 追踪拦截器
└── tracing_options.go     # gRPC 追踪配置
```

### 数据流

```
客户端请求
    ↓
HTTP 追踪中间件（提取上下文）
    ↓
创建 Span（记录请求信息）
    ↓
业务处理
    ↓
gRPC 调用（注入上下文）
    ↓
gRPC 追踪拦截器
    ↓
创建子 Span（记录 RPC 信息）
    ↓
返回响应（记录状态码）
    ↓
批量发送到 Jaeger/Zipkin
```

## 配置

### YAML 配置（configs/gateway.yaml）

```yaml
Tracing:
  Enable: true                              # 是否启用追踪
  ServiceName: aetherflow-gateway           # 服务名称
  Endpoint: http://localhost:14268/api/traces  # Jaeger endpoint
  Exporter: jaeger                          # 导出器: jaeger 或 zipkin
  SampleRate: 1.0                           # 采样率 (0.0-1.0)
  Environment: development                   # 环境
  BatchTimeout: 5                            # 批量发送超时（秒）
  MaxQueueSize: 2048                         # 最大队列大小
```

### 采样策略

| 采样率 | 说明 | 使用场景 |
|--------|------|----------|
| 1.0 | 全采样（100%） | 开发环境、调试 |
| 0.1 | 10% 采样 | 生产环境（中等流量） |
| 0.01 | 1% 采样 | 生产环境（高流量） |
| 0.0 | 不采样 | 禁用追踪 |

## 使用方法

### 1. 启动 Jaeger（推荐）

使用 Docker 快速启动：

```bash
docker run -d --name jaeger \
  -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
  -p 5775:5775/udp \
  -p 6831:6831/udp \
  -p 6832:6832/udp \
  -p 5778:5778 \
  -p 16686:16686 \
  -p 14268:14268 \
  -p 14250:14250 \
  -p 9411:9411 \
  jaegertracing/all-in-one:latest
```

访问 Jaeger UI：http://localhost:16686

### 2. 启动 Zipkin（可选）

```bash
docker run -d -p 9411:9411 openzipkin/zipkin
```

访问 Zipkin UI：http://localhost:9411

### 3. 配置 Gateway

编辑 `configs/gateway.yaml`：

```yaml
Tracing:
  Enable: true
  ServiceName: aetherflow-gateway
  Endpoint: http://localhost:14268/api/traces
  Exporter: jaeger
  SampleRate: 1.0
  Environment: development
```

### 4. 启动 Gateway

```bash
cd cmd/gateway
go run main.go -f ../../configs/gateway.yaml
```

### 5. 发送测试请求

```bash
# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"test123"}'

# 查看响应头中的 Trace ID
# X-Trace-ID: 1234567890abcdef1234567890abcdef
# X-Span-ID: 1234567890abcdef
```

### 6. 在 Jaeger UI 中查看追踪

1. 打开 http://localhost:16686
2. 在 Service 下拉框选择 `aetherflow-gateway`
3. 点击 "Find Traces" 查看追踪列表
4. 点击某个 Trace 查看详细信息

## 代码示例

### 手动创建 Span

```go
// 在处理函数中
func MyHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 获取 tracer
    tracer := svcCtx.Tracer
    
    // 创建新 span
    ctx, span := tracer.Start(ctx, "my-operation")
    defer span.End()
    
    // 添加属性
    tracer.SetAttributes(ctx,
        attribute.String("user.id", userID),
        attribute.Int("item.count", count),
    )
    
    // 记录事件
    tracer.AddEvent(ctx, "processing.started")
    
    // 执行业务逻辑
    result, err := doSomething(ctx)
    
    // 记录错误
    if err != nil {
        tracer.RecordError(ctx, err)
        return
    }
    
    // 获取 trace ID（用于日志关联）
    traceID := tracer.GetTraceID(ctx)
    log.Info("Operation completed", zap.String("trace_id", traceID))
}
```

### 跨服务传播

追踪信息会自动在以下情况下传播：

1. **HTTP 请求** - 通过 `traceparent` 和 `tracestate` 头
2. **gRPC 调用** - 通过 gRPC metadata
3. **WebSocket** - 可以在连接时传递

## 追踪信息说明

### HTTP Span 属性

| 属性 | 说明 | 示例 |
|------|------|------|
| http.method | HTTP 方法 | GET |
| http.route | 路由路径 | /api/v1/session |
| http.url | 完整 URL | http://localhost:8080/api/v1/session |
| http.status_code | 状态码 | 200 |
| http.user_agent | User-Agent | curl/7.68.0 |
| http.client_ip | 客户端 IP | 127.0.0.1 |

### gRPC Span 属性

| 属性 | 说明 | 示例 |
|------|------|------|
| rpc.system | RPC 系统 | grpc |
| rpc.service | 服务名 | session.SessionService |
| rpc.method | 方法名 | CreateSession |
| rpc.grpc.target | 目标地址 | 127.0.0.1:9001 |
| rpc.grpc.status_code | gRPC 状态 | OK |

## 性能考虑

### 开销

- **CPU 开销**: < 1% (采样率 1.0)
- **内存开销**: ~10MB (批量队列)
- **网络开销**: 异步批量发送，最小化影响

### 优化建议

1. **生产环境降低采样率**
   ```yaml
   SampleRate: 0.1  # 10% 采样
   ```

2. **调整批量发送参数**
   ```yaml
   BatchTimeout: 10    # 增加超时减少网络调用
   MaxQueueSize: 4096  # 增加队列大小
   ```

3. **使用本地 Jaeger Agent**
   ```yaml
   Endpoint: http://localhost:6831  # UDP 协议更快
   ```

## 故障排查

### 问题：追踪数据未显示

1. 检查配置是否启用
   ```yaml
   Tracing:
     Enable: true
   ```

2. 检查 Jaeger 是否运行
   ```bash
   curl http://localhost:14268/api/traces
   ```

3. 检查日志
   ```
   Failed to create tracer: ...
   ```

### 问题：采样率不生效

- 确保 `SampleRate` 在 0.0-1.0 范围内
- 重启服务使配置生效

### 问题：Trace ID 为空

- 确保中间件已正确注册
- 检查 `Tracer.IsEnabled()` 返回 true

## 最佳实践

### 1. 命名规范

- **Span 名称**: 使用 `操作类型 资源` 格式
  - ✅ `GET /api/v1/session`
  - ✅ `CreateSession`
  - ❌ `handler`

### 2. 属性添加

- 只添加有价值的属性
- 避免添加敏感信息（密码、token）
- 使用标准化的属性名

### 3. 错误处理

```go
if err != nil {
    tracer.RecordError(ctx, err,
        attribute.String("error.type", "validation_error"),
        attribute.String("user.id", userID),
    )
    return err
}
```

### 4. 性能监控

- 关注 span 持续时间
- 识别慢查询和瓶颈
- 使用 Jaeger 的 Service Performance Monitoring

## 与其他系统集成

### 日志关联

在日志中包含 Trace ID：

```go
traceID := tracer.GetTraceID(ctx)
logger.Info("Processing request",
    zap.String("trace_id", traceID),
    zap.String("user_id", userID),
)
```

### 指标关联

在 Prometheus 指标中添加 trace ID 标签：

```go
httpRequestDuration.WithLabelValues(
    method,
    path,
    tracer.GetTraceID(ctx),
).Observe(duration)
```

## 测试

运行单元测试：

```bash
cd internal/gateway/tracing
go test -v -cover
```

测试覆盖率：

```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 参考资料

- [OpenTelemetry 文档](https://opentelemetry.io/docs/)
- [Jaeger 文档](https://www.jaegertracing.io/docs/)
- [Zipkin 文档](https://zipkin.io/)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)

## 版本历史

- v1.0.0 (2026-02-10) - 初始版本
  - 支持 Jaeger 和 Zipkin
  - HTTP 和 gRPC 自动追踪
  - 可配置采样策略
