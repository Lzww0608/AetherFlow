# 链路追踪使用示例

本示例演示如何在 AetherFlow 中使用链路追踪功能。

## 快速开始

### 1. 启动 Jaeger

使用 Docker Compose 快速启动：

```bash
cd deployments
docker-compose -f docker-compose.tracing.yml up -d jaeger
```

或直接使用 Docker：

```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:latest
```

### 2. 配置 Gateway

编辑 `configs/gateway.yaml`：

```yaml
Tracing:
  Enable: true                              # 启用追踪
  ServiceName: aetherflow-gateway
  Endpoint: http://localhost:14268/api/traces
  Exporter: jaeger
  SampleRate: 1.0                           # 全采样
  Environment: development
```

### 3. 启动 Gateway

```bash
cd cmd/gateway
go run main.go -f ../../configs/gateway.yaml
```

### 4. 发送测试请求

```bash
# 登录获取 token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"test123"}' | jq -r '.data.access_token')

# 创建会话（会生成 trace）
curl -X POST http://localhost:8080/api/v1/session \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "client_ip": "127.0.0.1",
    "client_port": 12345
  }' -v

# 注意响应头中的 X-Trace-ID 和 X-Span-ID
```

### 5. 查看追踪

打开 Jaeger UI：http://localhost:16686

1. 在 Service 下拉框选择 `aetherflow-gateway`
2. 点击 "Find Traces"
3. 点击某个 Trace 查看详细的调用链

## 追踪示例

### HTTP 请求追踪

发送一个 HTTP 请求：

```bash
curl -X GET http://localhost:8080/api/v1/session?session_id=123 \
  -H "Authorization: Bearer $TOKEN" \
  -v
```

在 Jaeger 中可以看到：

```
Trace: GET /api/v1/session
├─ Span: GET /api/v1/session (gateway)
│  ├─ http.method: GET
│  ├─ http.route: /api/v1/session
│  ├─ http.status_code: 200
│  └─ Duration: 45ms
└─ Span: /session.SessionService/GetSession (grpc client)
   ├─ rpc.system: grpc
   ├─ rpc.service: session.SessionService
   ├─ rpc.method: GetSession
   └─ Duration: 40ms
```

### 错误追踪

发送一个会失败的请求：

```bash
curl -X GET http://localhost:8080/api/v1/session?session_id=nonexistent \
  -H "Authorization: Bearer $TOKEN"
```

在 Jaeger 中可以看到 Span 被标记为错误（红色），包含错误信息。

### 分布式追踪

创建文档并应用操作：

```bash
# 创建文档
DOC_ID=$(curl -s -X POST http://localhost:8080/api/v1/document \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test Doc",
    "type": "text",
    "owner_id": "user123"
  }' | jq -r '.data.document_id')

# 应用操作
curl -X POST http://localhost:8080/api/v1/document/operation \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"document_id\": \"$DOC_ID\",
    \"operation_type\": \"insert\",
    \"data\": \"Hello World\",
    \"position\": 0
  }"
```

在 Jaeger 中可以看到完整的调用链：

```
Trace: Document Operation
├─ POST /api/v1/document (45ms)
│  └─ CreateDocument gRPC (40ms)
└─ POST /api/v1/document/operation (60ms)
   ├─ ApplyOperation gRPC (50ms)
   └─ Subscribe Stream (5ms)
```

## 高级用法

### 自定义 Span

在代码中手动创建 Span：

```go
import (
    "github.com/aetherflow/aetherflow/internal/gateway/tracing"
    "go.opentelemetry.io/otel/attribute"
)

func MyHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tracer := svcCtx.Tracer
    
    // 创建自定义 span
    ctx, span := tracer.Start(ctx, "complex-operation")
    defer span.End()
    
    // 添加自定义属性
    tracer.SetAttributes(ctx,
        attribute.String("user.id", "user123"),
        attribute.Int("retry.count", 3),
        attribute.Bool("cache.hit", true),
    )
    
    // 记录事件
    tracer.AddEvent(ctx, "validation.completed")
    
    // 执行操作
    result, err := doSomething(ctx)
    if err != nil {
        // 记录错误
        tracer.RecordError(ctx, err,
            attribute.String("error.type", "validation_error"),
        )
        return
    }
    
    // 获取 trace ID（用于日志关联）
    traceID := tracer.GetTraceID(ctx)
    log.Info("Operation completed", 
        zap.String("trace_id", traceID),
        zap.Any("result", result),
    )
}
```

### 跨服务追踪

如果需要调用外部 HTTP 服务并传递追踪信息：

```go
import (
    "net/http"
)

func CallExternalService(ctx context.Context, tracer *tracing.Tracer) error {
    // 创建 HTTP 请求
    req, _ := http.NewRequestWithContext(ctx, "GET", "http://external-api/data", nil)
    
    // 注入追踪信息到请求头
    headers := make(map[string]string)
    tracer.InjectHTTPHeaders(ctx, headers)
    for k, v := range headers {
        req.Header.Set(k, v)
    }
    
    // 发送请求
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

## 采样策略配置

### 开发环境（全采样）

```yaml
Tracing:
  SampleRate: 1.0
```

### 生产环境（10% 采样）

```yaml
Tracing:
  SampleRate: 0.1
```

### 基于头部的动态采样

可以在中间件中实现基于请求头的动态采样：

```go
// 如果请求头包含 X-Force-Trace: true，强制采样
if r.Header.Get("X-Force-Trace") == "true" {
    // 使用 AlwaysSample sampler
}
```

## 性能测试

### 测试追踪开销

```bash
# 禁用追踪
ab -n 10000 -c 100 http://localhost:8080/health

# 启用追踪
ab -n 10000 -c 100 http://localhost:8080/health

# 比较两者的性能差异
```

一般情况下，开销 < 1% (全采样)。

## 故障排查

### 问题：Jaeger UI 中看不到数据

1. 检查 Gateway 日志
   ```
   INFO tracing/tracer.go:116 Tracing initialized
   ```

2. 检查 Jaeger 是否运行
   ```bash
   curl http://localhost:14268/api/traces
   ```

3. 检查配置
   ```yaml
   Tracing:
     Enable: true
   ```

### 问题：Trace 数据不完整

- 检查采样率：`SampleRate: 1.0` (全采样)
- 检查批量发送设置：`BatchTimeout: 5`
- 查看 Gateway 关闭日志，确保数据已刷新

### 问题：性能下降

- 降低采样率：`SampleRate: 0.1`
- 增加批量超时：`BatchTimeout: 10`
- 增加队列大小：`MaxQueueSize: 4096`

## 与其他工具集成

### Grafana Tempo

如果使用 Tempo 代替 Jaeger：

```yaml
Tracing:
  Exporter: otlp
  Endpoint: http://localhost:4318/v1/traces
```

### Elasticsearch 后端

Jaeger 可以配置使用 Elasticsearch 作为存储后端，实现长期数据保留。

## 更多示例

查看以下文件获取更多示例：

- [完整文档](../../internal/gateway/tracing/README.md)
- [单元测试](../../internal/gateway/tracing/tracer_test.go)
- [中间件实现](../../internal/gateway/middleware/tracing.go)
