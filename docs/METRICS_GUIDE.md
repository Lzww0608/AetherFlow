# Prometheus 指标指南

本指南介绍 AetherFlow 中的 Prometheus 指标收集和监控。

## 指标概述

AetherFlow 提供了完整的 Prometheus 指标，涵盖以下方面：

### 1. HTTP 请求指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_http_requests_total` | Counter | HTTP 请求总数 | method, path, status_code |
| `aetherflow_gateway_http_request_duration_seconds` | Histogram | HTTP 请求延迟 | method, path |
| `aetherflow_gateway_http_request_size_bytes` | Histogram | HTTP 请求大小 | method, path |
| `aetherflow_gateway_http_response_size_bytes` | Histogram | HTTP 响应大小 | method, path |
| `aetherflow_gateway_http_active_requests` | Gauge | 活跃 HTTP 请求数 | method, path |

### 2. gRPC 请求指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_grpc_requests_total` | Counter | gRPC 请求总数 | service, method, status |
| `aetherflow_gateway_grpc_request_duration_seconds` | Histogram | gRPC 请求延迟 | service, method |
| `aetherflow_gateway_grpc_stream_messages_total` | Counter | gRPC 流消息总数 | service, method, direction |
| `aetherflow_gateway_grpc_active_streams` | Gauge | 活跃 gRPC 流数 | service, method |

### 3. WebSocket 指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_websocket_connections_total` | Counter | WebSocket 连接总数 | status |
| `aetherflow_gateway_websocket_active_connections` | Gauge | 活跃 WebSocket 连接数 | - |
| `aetherflow_gateway_websocket_messages_total` | Counter | WebSocket 消息总数 | type, direction |
| `aetherflow_gateway_websocket_message_size_bytes` | Histogram | WebSocket 消息大小 | type |

### 4. 业务指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_sessions_total` | Counter | 会话总数 | action |
| `aetherflow_gateway_active_sessions` | Gauge | 活跃会话数 | - |
| `aetherflow_gateway_documents_total` | Counter | 文档总数 | action |
| `aetherflow_gateway_active_documents` | Gauge | 活跃文档数 | - |
| `aetherflow_gateway_operations_total` | Counter | 操作总数 | type, status |
| `aetherflow_gateway_conflicts_total` | Counter | 冲突总数 | resolution |

### 5. 系统指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_errors_total` | Counter | 错误总数 | type, code |
| `aetherflow_gateway_panics_total` | Counter | Panic 总数 | location |
| `aetherflow_gateway_goroutines` | Gauge | Goroutine 数量 | - |

### 6. 熔断器指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_circuit_breaker_state` | Gauge | 熔断器状态 (0=closed, 1=open, 2=half-open) | name |
| `aetherflow_gateway_circuit_breaker_trips_total` | Counter | 熔断器跳闸总数 | name |

### 7. 缓存指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_cache_hits_total` | Counter | 缓存命中总数 | cache |
| `aetherflow_gateway_cache_misses_total` | Counter | 缓存未命中总数 | cache |
| `aetherflow_gateway_cache_evictions_total` | Counter | 缓存驱逐总数 | cache |

### 8. 链路追踪指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `aetherflow_gateway_traces_total` | Counter | 追踪总数 | sampled |
| `aetherflow_gateway_spans_total` | Counter | Span 总数 | operation |

## 配置

在 `configs/gateway.yaml` 中配置 Prometheus：

```yaml
Prometheus:
  Host: 0.0.0.0
  Port: 9091
  Path: /metrics
```

## 查询示例

### 请求速率

```promql
# 每秒请求数
rate(aetherflow_gateway_http_requests_total[1m])

# 按状态码分组
sum by (status_code) (rate(aetherflow_gateway_http_requests_total[1m]))
```

### 延迟百分位数

```promql
# P50 延迟
histogram_quantile(0.50, rate(aetherflow_gateway_http_request_duration_seconds_bucket[1m]))

# P95 延迟
histogram_quantile(0.95, rate(aetherflow_gateway_http_request_duration_seconds_bucket[1m]))

# P99 延迟
histogram_quantile(0.99, rate(aetherflow_gateway_http_request_duration_seconds_bucket[1m]))
```

### 错误率

```promql
# 总体错误率
rate(aetherflow_gateway_errors_total[1m])

# HTTP 4xx 错误率
sum(rate(aetherflow_gateway_http_requests_total{status_code=~"4.."}[1m]))

# HTTP 5xx 错误率
sum(rate(aetherflow_gateway_http_requests_total{status_code=~"5.."}[1m]))
```

### 资源使用

```promql
# Goroutine 数量
aetherflow_gateway_goroutines

# 活跃连接数
aetherflow_gateway_websocket_active_connections

# 活跃会话数
aetherflow_gateway_active_sessions
```

### 熔断器状态

```promql
# 熔断器状态
aetherflow_gateway_circuit_breaker_state

# 熔断器跳闸率
rate(aetherflow_gateway_circuit_breaker_trips_total[1m])
```

### 缓存性能

```promql
# 缓存命中率
rate(aetherflow_gateway_cache_hits_total[1m]) / (
  rate(aetherflow_gateway_cache_hits_total[1m]) + 
  rate(aetherflow_gateway_cache_misses_total[1m])
)
```

## 告警规则

在 `configs/prometheus/alert-rules.yml` 中定义告警规则：

```yaml
groups:
  - name: aetherflow_gateway
    rules:
      # 高错误率告警
      - alert: HighErrorRate
        expr: |
          rate(aetherflow_gateway_errors_total[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors/s"

      # 高延迟告警
      - alert: HighLatency
        expr: |
          histogram_quantile(0.95, 
            rate(aetherflow_gateway_http_request_duration_seconds_bucket[5m])
          ) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "P95 latency is {{ $value }}s"

      # 熔断器打开告警
      - alert: CircuitBreakerOpen
        expr: |
          aetherflow_gateway_circuit_breaker_state == 1
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Circuit breaker {{ $labels.name }} is open"
          description: "Circuit breaker has been open for 1 minute"

      # 高 Goroutine 数量告警
      - alert: HighGoroutineCount
        expr: |
          aetherflow_gateway_goroutines > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High goroutine count"
          description: "Goroutine count is {{ $value }}"
```

## Grafana 仪表盘

导入 Grafana 仪表盘：

```bash
# 导入仪表盘
curl -X POST http://localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @configs/grafana/dashboard-gateway.json
```

## 最佳实践

### 1. 指标命名

- 使用统一的前缀 `aetherflow_gateway_`
- 使用下划线分隔单词
- 使用有意义的标签

### 2. 标签使用

- 避免高基数标签（如 user_id, trace_id）
- 使用有限的标签值
- 标签值应该是枚举类型

### 3. 性能考虑

- Histogram 使用合适的 bucket
- Counter 只增不减
- Gauge 可增可减
- 定期清理不再使用的指标

### 4. 查询优化

- 使用 `rate()` 计算速率
- 使用 `histogram_quantile()` 计算百分位数
- 避免查询过长的时间范围

## 故障排查

### 问题：指标未显示

1. 检查 Prometheus 配置
2. 检查 Gateway 是否暴露指标端点
3. 检查防火墙规则

### 问题：指标不准确

1. 检查时间同步
2. 检查采样频率
3. 检查聚合规则

### 问题：性能问题

1. 减少指标数量
2. 降低采样频率
3. 使用 remote write

## 参考资料

- [Prometheus 文档](https://prometheus.io/docs/)
- [Grafana 文档](https://grafana.com/docs/)
- [Prometheus 最佳实践](https://prometheus.io/docs/practices/)
