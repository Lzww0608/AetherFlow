# 如何运行真实性能测试

## 快速开始

### 1. 编译服务

\`\`\`bash
cd /home/lab2439/Work/lzww/AetherFlow

# 编译 Session 和 StateSync 服务
go build -o bin/session-service cmd/session-service/main.go
go build -o bin/statesync-service cmd/statesync-service/main.go
\`\`\`

### 2. 启动服务

\`\`\`bash
# 启动 Session Service (端口 9001)
./bin/session-service > /tmp/session.log 2>&1 &

# 启动 StateSync Service (端口 9002)
./bin/statesync-service > /tmp/statesync.log 2>&1 &

# 等待服务启动
sleep 2

# 验证服务运行状态
tail -5 /tmp/session.log
tail -5 /tmp/statesync.log
\`\`\`

### 3. 运行性能测试

#### 单线程延迟测试

\`\`\`bash
go run benchmarks/integration/simple-test.go
\`\`\`

**预期结果**:
- P99 延迟 < 2ms
- 测试 100 次请求

#### 并发压力测试

\`\`\`bash
go run benchmarks/integration/concurrent-test.go
\`\`\`

**预期结果**:
- 并发度 200: QPS 90K-115K
- P99 延迟 < 5ms

### 4. 停止服务

\`\`\`bash
killall session-service statesync-service
\`\`\`

## 测试文件说明

- \`simple-test.go\`: 单线程顺序测试，测量最低延迟
- \`concurrent-test.go\`: 多并发压力测试，测量 QPS 和高负载延迟
- \`REAL_BENCHMARK_RESULTS.md\`: 完整的真实测试结果报告

## 测试类型说明

### 1. Integration 测试 (当前已完成)
- **位置**: `integration/`
- **协议**: TCP (gRPC over HTTP/2)
- **环境**: localhost 本地回环
- **目的**: 测量微服务架构的 baseline 性能
- **数据**: ✅ **真实执行**的 gRPC 调用

### 2. Quantum vs TCP 对比 (需要完成)
- **位置**: `quantum-tcp-compare/`
- **协议**: TCP vs Quantum
- **环境**: 使用 `tc netem` 模拟不同网络条件
- **目的**: 量化 Quantum 协议在不同场景下的性能优势
- **数据**: 需要实现 Quantum 客户端后才能获得真实数据

### 3. 之前的模拟测试 (已过时)
- **位置**: `quantum-vs-tcp/` (旧版本)
- **方法**: 使用 `time.Sleep` 模拟
- **数据**: ⚠️ 理论预期值，非真实执行

## 测试结果亮点

### TCP Baseline (已完成)
✅ **Session Service P99**: 0.91ms (单线程) / 4.25ms (200并发)  
✅ **StateSync Service P99**: 1.22ms (单线程) / 3.97ms (200并发)  
✅ **峰值 QPS**: 114,910 (StateSync, 200并发)  
✅ **端到端 P99**: ~7.13ms **(目标 < 50ms)** 🎯

### Quantum vs TCP 对比 (待测试)
查看 `quantum-tcp-compare/README.md` 了解如何进行对比测试。

**预期结果** (基于协议特性):
- 低延迟场景 (RTT < 10ms): Quantum 优势 ~5-10%
- 中延迟场景 (RTT 50ms): Quantum 优势 ~30-40%
- 高延迟场景 (RTT 100ms): Quantum 优势 ~50-60%
- 丢包场景 (2-5% loss): Quantum 优势 ~70-80%
