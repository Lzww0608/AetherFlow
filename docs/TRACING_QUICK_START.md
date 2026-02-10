# 链路追踪快速开始指南

本指南帮助你在 5 分钟内启动 AetherFlow 的链路追踪功能。

## 前置条件

- Docker (用于运行 Jaeger)
- Go 1.21+ (用于编译 Gateway)
- curl / httpie (用于测试)

## 步骤 1: 启动 Jaeger

```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:latest
```

验证 Jaeger 是否运行：
```bash
curl http://localhost:14268/api/traces
```

访问 Jaeger UI：http://localhost:16686

## 步骤 2: 配置 Gateway

编辑 `configs/gateway.yaml`，启用追踪：

```yaml
Tracing:
  Enable: true                              # 启用追踪
  ServiceName: aetherflow-gateway           # 服务名称
  Endpoint: http://localhost:14268/api/traces  # Jaeger endpoint
  Exporter: jaeger                          # 导出器类型
  SampleRate: 1.0                           # 100% 采样
  Environment: development                   # 环境
```

## 步骤 3: 启动 Gateway

```bash
cd cmd/gateway
go run main.go -f ../../configs/gateway.yaml
```

查看日志中的追踪初始化信息：
```
INFO tracing/tracer.go:116 Tracing initialized
```

## 步骤 4: 发送测试请求

### 4.1 健康检查

```bash
curl http://localhost:8080/health -v
```

注意响应头中的追踪信息：
```
X-Trace-ID: 1234567890abcdef1234567890abcdef
X-Span-ID: 1234567890abcdef
```

### 4.2 认证请求

```bash
# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}' \
  -v

# 获取 token
TOKEN="<从上面响应中复制 access_token>"
```

### 4.3 创建会话（触发 gRPC 调用）

```bash
curl -X POST http://localhost:8080/api/v1/session \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "client_ip": "127.0.0.1",
    "client_port": 12345
  }' \
  -v
```

## 步骤 5: 查看追踪数据

1. 打开 Jaeger UI：http://localhost:16686

2. 在 **Service** 下拉框选择 `aetherflow-gateway`

3. 点击 **Find Traces** 按钮

4. 你将看到刚才发送的请求的追踪记录

5. 点击某个 Trace 查看详细信息：

```
Trace Timeline:
├─ POST /api/v1/session (Gateway)
│  ├─ Duration: 45ms
│  ├─ http.method: POST
│  ├─ http.status_code: 200
│  └─ Span ID: abc123...
└─ /session.SessionService/CreateSession (gRPC)
   ├─ Duration: 40ms
   ├─ rpc.system: grpc
   ├─ rpc.method: CreateSession
   └─ Span ID: def456...
```

## 步骤 6: 探索更多功能

### 查看错误追踪

发送一个会失败的请求：

```bash
curl -X GET http://localhost:8080/api/v1/session?session_id=invalid \
  -H "Authorization: Bearer $TOKEN"
```

在 Jaeger 中可以看到 Span 被标记为错误（红色图标），包含错误详情。

### 查看服务依赖图

1. 在 Jaeger UI 中点击 **Dependencies**
2. 选择时间范围
3. 查看服务间的调用关系图

### 过滤和搜索

- **按 Trace ID 搜索**: 使用响应头中的 `X-Trace-ID`
- **按持续时间过滤**: `minDuration=100ms`
- **按标签过滤**: `http.status_code=500`

## 生产环境配置

### 降低采样率（推荐）

生产环境不需要追踪每个请求，可以降低采样率：

```yaml
Tracing:
  SampleRate: 0.1  # 10% 采样
```

### 使用远程 Jaeger

```yaml
Tracing:
  Endpoint: http://jaeger-collector.prod.svc.cluster.local:14268/api/traces
  Environment: production
```

### 批量优化

```yaml
Tracing:
  BatchTimeout: 10      # 增加批量超时
  MaxQueueSize: 4096    # 增加队列大小
```

## 故障排查

### Jaeger UI 中看不到数据

1. 检查 Gateway 日志是否有追踪初始化信息
2. 检查 Jaeger 是否可访问：`curl http://localhost:14268/api/traces`
3. 确认配置中 `Enable: true`
4. 检查采样率是否为 0

### 性能问题

1. 降低采样率到 0.1 或更低
2. 检查 Jaeger 的资源使用情况
3. 考虑使用 Jaeger Agent 代替直接发送到 Collector

### 追踪数据不完整

1. 检查 `BatchTimeout` 和 `MaxQueueSize` 配置
2. 确保 Gateway 优雅关闭（等待数据刷新）
3. 检查网络连接是否稳定

## 下一步

- 阅读 [完整文档](../internal/gateway/tracing/README.md)
- 查看 [使用示例](../examples/tracing/README.md)
- 学习 [最佳实践](../docs/TRACING_BEST_PRACTICES.md)
- 了解 [性能优化](../docs/TRACING_PERFORMANCE.md)

## 有用的资源

- [OpenTelemetry 文档](https://opentelemetry.io/docs/)
- [Jaeger 文档](https://www.jaegertracing.io/docs/)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [分布式追踪最佳实践](https://opentracing.io/docs/best-practices/)

---

**提示**: 在生产环境使用前，请务必测试追踪系统的性能影响并适当调整采样率。
