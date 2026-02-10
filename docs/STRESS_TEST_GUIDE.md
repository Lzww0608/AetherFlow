# 压力测试指南

本指南介绍如何对 AetherFlow 进行压力测试。

## 快速开始

### 1. 编译压力测试工具

```bash
cd tools/stress-test
go build -o stress-test main.go
```

### 2. 运行基础测试

```bash
./stress-test -target http://localhost:8080/health -c 10 -d 30s
```

### 3. 使用测试脚本

```bash
# 基础负载测试
./scripts/stress-test.sh basic

# 查看所有场景
./scripts/stress-test.sh
```

## 测试场景

### 场景 1: 基础负载测试

**目标**: 验证系统基本功能
**参数**:
- 并发: 10
- 持续时间: 30秒
- 目标: /health

```bash
./scripts/stress-test.sh basic
```

**预期结果**:
- 成功率: >99%
- P95 延迟: <10ms
- 吞吐量: >1000 req/s

### 场景 2: 中等负载测试

**目标**: 测试常规负载下的性能
**参数**:
- 并发: 50
- 持续时间: 1分钟
- 目标: /health

```bash
./scripts/stress-test.sh medium
```

**预期结果**:
- 成功率: >99%
- P95 延迟: <20ms
- 吞吐量: >5000 req/s

### 场景 3: 高负载测试

**目标**: 测试系统承载能力
**参数**:
- 并发: 100
- 持续时间: 2分钟
- 目标: /health

```bash
./scripts/stress-test.sh heavy
```

**预期结果**:
- 成功率: >98%
- P95 延迟: <50ms
- 吞吐量: >10000 req/s

### 场景 4: 峰值测试

**目标**: 测试瞬时高并发
**参数**:
- 并发: 200
- 持续时间: 30秒
- 目标: /health

```bash
./scripts/stress-test.sh spike
```

**预期结果**:
- 成功率: >95%
- P95 延迟: <100ms
- 无崩溃或重启

### 场景 5: 持续负载测试

**目标**: 测试长时间稳定性
**参数**:
- 并发: 50
- 持续时间: 5分钟
- 目标: /health

```bash
./scripts/stress-test.sh sustained
```

**预期结果**:
- 成功率: >99%
- 延迟稳定（无明显增长）
- 无内存泄漏

### 场景 6: 限流测试

**目标**: 验证限流机制
**参数**:
- 并发: 20
- RPS: 1000
- 持续时间: 1分钟
- 目标: /health

```bash
./scripts/stress-test.sh ratelimit
```

**预期结果**:
- 触发限流 (429状态码)
- 限流后稳定运行
- 无请求丢失

### 场景 7: 认证端点测试

**目标**: 测试认证系统性能
**参数**:
- 并发: 30
- 持续时间: 1分钟
- 目标: /api/v1/auth/login

```bash
./scripts/stress-test.sh auth
```

**预期结果**:
- 成功率: >99%
- P95 延迟: <100ms
- JWT 正常生成

## 高级用法

### 自定义测试

```bash
./tools/stress-test/stress-test \
  -target http://localhost:8080/api/v1/session \
  -c 50 \
  -d 2m \
  -rps 500 \
  -method POST \
  -body '{"user_id":"test","client_ip":"127.0.0.1"}' \
  -timeout 10s
```

### 参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-target` | 目标 URL | http://localhost:8080/health |
| `-c` | 并发数 | 10 |
| `-d` | 持续时间 | 10s |
| `-rps` | 每秒请求数 (0=无限制) | 0 |
| `-timeout` | 请求超时 | 30s |
| `-method` | HTTP 方法 | GET |
| `-body` | 请求体 | "" |
| `-keepalive` | 使用 HTTP keep-alive | true |
| `-skip-verify` | 跳过 TLS 验证 | false |

## 结果分析

### 输出示例

```
============================================================
Stress Test Results
============================================================
Target:           http://localhost:8080/health
Concurrency:      50
Duration:         1m0s
------------------------------------------------------------
Total Requests:   305420
Success:          305420 (100.00%)
Failed:           0 (0.00%)
Throughput:       5090.33 req/s
------------------------------------------------------------
Min Latency:      1.234ms
Max Latency:      45.678ms
Avg Latency:      9.876ms
P50 Latency:      8.234ms
P95 Latency:      18.456ms
P99 Latency:      25.789ms
------------------------------------------------------------
Status Code Distribution:
  200: 305420 (100.00%)
============================================================
```

### 关键指标

#### 1. 成功率

- **优秀**: >99.5%
- **良好**: 98-99.5%
- **需优化**: <98%

#### 2. 延迟（P95）

- **优秀**: <20ms
- **良好**: 20-50ms
- **需优化**: >50ms

#### 3. 吞吐量

- **优秀**: >10000 req/s
- **良好**: 5000-10000 req/s
- **需优化**: <5000 req/s

#### 4. 错误类型

- **超时**: 检查超时配置
- **连接拒绝**: 检查连接池大小
- **429 Too Many Requests**: 触发限流
- **500 Internal Server Error**: 服务器错误

## 监控

### 实时监控

在测试期间监控以下指标：

```bash
# 1. CPU 使用率
top

# 2. 内存使用
free -h

# 3. 网络连接数
netstat -an | grep ESTABLISHED | wc -l

# 4. Goroutine 数量
curl http://localhost:9091/metrics | grep goroutines

# 5. HTTP 请求延迟
curl http://localhost:9091/metrics | grep http_request_duration
```

### Prometheus 查询

```promql
# 请求速率
rate(aetherflow_gateway_http_requests_total[1m])

# P95 延迟
histogram_quantile(0.95, rate(aetherflow_gateway_http_request_duration_seconds_bucket[1m]))

# 错误率
rate(aetherflow_gateway_errors_total[1m])
```

## 性能优化

### 1. 系统调优

```bash
# 增加文件描述符限制
ulimit -n 65535

# 调整 TCP 参数
sysctl -w net.ipv4.tcp_tw_reuse=1
sysctl -w net.ipv4.tcp_fin_timeout=30
```

### 2. Gateway 配置

```yaml
# configs/gateway.yaml

# 增加连接池大小
GRPC:
  Pool:
    MaxIdle: 100
    MaxActive: 1000
    IdleTimeout: 60

# 调整超时
Timeout: 30000

# 调整限流
RateLimit:
  Rate: 10000
  Burst: 20000
```

### 3. Go 运行时调优

```bash
# 设置 GOMAXPROCS
export GOMAXPROCS=8

# 调整 GC
export GOGC=200
```

## 故障排查

### 问题：高错误率

**可能原因**:
1. 服务器资源不足
2. 数据库连接池耗尽
3. 超时配置过小

**解决方案**:
1. 增加服务器资源
2. 调整连接池大小
3. 增加超时时间

### 问题：高延迟

**可能原因**:
1. GC 压力大
2. 网络延迟
3. 数据库慢查询

**解决方案**:
1. 优化内存使用
2. 使用本地网络测试
3. 优化查询

### 问题：内存泄漏

**可能原因**:
1. Goroutine 泄漏
2. 连接未关闭
3. 缓存无限增长

**解决方案**:
1. 检查 Goroutine 数量
2. 确保连接正确关闭
3. 实现缓存驱逐策略

## 最佳实践

### 1. 测试前准备

- 清空日志文件
- 重启服务
- 预热系统（发送少量请求）

### 2. 测试过程

- 逐步增加负载
- 监控系统指标
- 记录异常现象

### 3. 测试后分析

- 分析日志
- 对比基线
- 生成报告

### 4. 持续测试

- 定期执行压力测试
- 建立性能基线
- 追踪性能趋势

## CI/CD 集成

### GitHub Actions

```yaml
name: Stress Test

on:
  pull_request:
  schedule:
    - cron: '0 0 * * *'  # 每天运行

jobs:
  stress-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21
      
      - name: Start Gateway
        run: |
          cd cmd/gateway
          go run main.go &
          sleep 10
      
      - name: Run Stress Test
        run: ./scripts/stress-test.sh basic
      
      - name: Upload Results
        uses: actions/upload-artifact@v2
        with:
          name: stress-test-results
          path: results/
```

## 参考资料

- [Apache Bench (ab)](https://httpd.apache.org/docs/2.4/programs/ab.html)
- [wrk](https://github.com/wg/wrk)
- [Gatling](https://gatling.io/)
- [性能测试最佳实践](https://www.nginx.com/blog/performance-testing-best-practices/)
