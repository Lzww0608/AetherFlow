# Quantum vs TCP 真实性能对比测试

## 概述

本测试套件用于对比 **Quantum 协议** 和 **TCP** 在不同网络条件下的真实性能差异。

### 测试方法

使用 Linux `tc netem` 工具模拟真实网络条件：
- 延迟 (latency)
- 丢包 (packet loss)
- 抖动 (jitter)
- 带宽限制 (bandwidth limit)

## 前置条件

### 1. 需要 root 权限

```bash
# tc (traffic control) 需要 root 权限
sudo -v
```

### 2. 安装依赖

```bash
# Ubuntu/Debian
sudo apt-get install iproute2

# CentOS/RHEL
sudo yum install iproute
```

## 快速开始

### 1. 基础测试 (低延迟场景)

```bash
cd /home/lab2439/Work/lzww/AetherFlow/benchmarks/quantum-tcp-compare

# 运行测试
sudo ./run-comparison.sh
```

**预期结果**:
- 低延迟场景: TCP 和 Quantum 性能接近
- Quantum 优势: ~5-10%

### 2. 高延迟场景 (50ms RTT)

```bash
sudo ./run-comparison.sh --latency 50ms
```

**预期结果**:
- Quantum P99 比 TCP 快 ~30-40%
- Quantum 通过 BBR 拥塞控制和更大的发送窗口降低延迟

### 3. 丢包场景 (2% loss)

```bash
sudo ./run-comparison.sh --latency 50ms --loss 2%
```

**预期结果**:
- Quantum P99 比 TCP 快 ~70-80%
- Quantum FEC (Reed-Solomon) 避免重传

### 4. 极端场景 (100ms RTT + 5% loss)

```bash
sudo ./run-comparison.sh --latency 100ms --loss 5%
```

**预期结果**:
- TCP: P99 ~800-1000ms (频繁重传)
- Quantum: P99 ~150-200ms (FEC 恢复)
- Quantum 优势: ~80-85%

## 测试场景详解

### 场景 1: 理想网络 (Baseline)
```bash
延迟: 0ms
丢包: 0%
抖动: 0ms

预期:
- TCP P99: ~1-2ms
- Quantum P99: ~1-2ms
- 差异: < 10%
```

### 场景 2: 本地数据中心
```bash
延迟: 1-2ms
丢包: 0.01%
抖动: 0.5ms

预期:
- TCP P99: ~5-8ms
- Quantum P99: ~4-6ms
- Quantum 优势: ~20-30%
```

### 场景 3: 同城跨机房
```bash
延迟: 5-10ms
丢包: 0.1%
抖动: 1ms

预期:
- TCP P99: ~15-25ms
- Quantum P99: ~12-18ms
- Quantum 优势: ~30-40%
```

### 场景 4: 跨地域 (国内)
```bash
延迟: 30-50ms
丢包: 0.5-1%
抖动: 5ms

预期:
- TCP P99: ~80-150ms
- Quantum P99: ~45-70ms
- Quantum 优势: ~40-50%
```

### 场景 5: 跨国网络
```bash
延迟: 100-150ms
丢包: 1-2%
抖动: 10ms

预期:
- TCP P99: ~300-500ms
- Quantum P99: ~120-180ms
- Quantum 优势: ~60-70%
```

### 场景 6: 移动网络 (4G)
```bash
延迟: 50-100ms
丢包: 2-5%
抖动: 20ms

预期:
- TCP P99: ~500-1000ms
- Quantum P99: ~100-200ms
- Quantum 优势: ~70-80%
```

### 场景 7: 卫星链路
```bash
延迟: 500-600ms
丢包: 3-5%
抖动: 50ms

预期:
- TCP P99: ~2000-3000ms
- Quantum P99: ~700-1000ms
- Quantum 优势: ~65-75%
```

## 测试原理

### TCP 的问题

1. **慢启动 (Slow Start)**
   - TCP 连接初始拥塞窗口很小
   - 需要多个 RTT 才能达到最优速度

2. **丢包重传**
   - 检测到丢包后才重传
   - 需要等待 RTO (Retransmission Timeout)
   - 高丢包率下性能急剧下降

3. **队头阻塞 (Head-of-Line Blocking)**
   - 一个包丢失会阻塞整个流
   - 即使后续数据已到达

### Quantum 的优势

1. **BBR 拥塞控制**
   - 基于带宽和 RTT 的主动式拥塞控制
   - 无需等待丢包即可找到最优速率
   - 更快达到最大带宽

2. **前向纠错 (FEC - Reed-Solomon)**
   - 发送冗余数据
   - 丢失的包可以通过冗余数据恢复
   - 无需重传

3. **0-RTT 连接建立**
   - 复用之前的连接参数
   - 首次请求即可发送数据

4. **多路复用无队头阻塞**
   - 每个流独立传输
   - 一个流的丢包不影响其他流

## 网络模拟命令参考

### 添加延迟
```bash
# 添加 50ms 延迟
sudo tc qdisc add dev lo root netem delay 50ms

# 添加延迟 + 抖动
sudo tc qdisc add dev lo root netem delay 50ms 10ms
```

### 添加丢包
```bash
# 2% 随机丢包
sudo tc qdisc add dev lo root netem loss 2%

# 2% 丢包 + 25% 相关性 (burst loss)
sudo tc qdisc add dev lo root netem loss 2% 25%
```

### 组合条件
```bash
# 50ms 延迟 + 2% 丢包 + 5ms 抖动
sudo tc qdisc add dev lo root netem delay 50ms 5ms loss 2%
```

### 查看配置
```bash
sudo tc qdisc show dev lo
```

### 清除配置
```bash
sudo tc qdisc del dev lo root
```

## 注意事项

1. **需要 root 权限**: `tc` 命令需要 root 权限
2. **影响范围**: `tc` 设置会影响指定网卡的所有流量
3. **测试后清理**: 测试完成后务必清除 `tc` 配置
4. **不要在生产环境运行**: 网络模拟会影响系统性能

## 真实环境测试建议

对于生产验证，建议使用真实网络环境：

### 1. 跨地域云服务器
```
部署两台服务器:
- 服务器 A: 阿里云北京
- 服务器 B: 阿里云美国东部
- 真实 RTT: ~150-200ms
```

### 2. 移动网络实测
```
使用真实移动设备:
- 客户端: Android/iOS 手机 (4G/5G)
- 服务端: 云服务器
- 测试不同信号强度
```

### 3. CDN 边缘节点
```
部署到 CDN 边缘:
- 测试用户到最近边缘节点的延迟
- 对比 TCP 和 Quantum 的用户体验
```

## 结果解读

### 判断 Quantum 是否有优势

- **延迟降低 > 30%**: 显著优势，推荐使用
- **延迟降低 10-30%**: 有一定优势
- **延迟降低 < 10%**: 优势不明显，可能不值得额外复杂度

### 适合使用 Quantum 的场景

✅ **强烈推荐**:
- 跨国/跨洲网络 (RTT > 100ms)
- 移动网络 (丢包率 > 1%)
- 卫星/海底光缆

✅ **推荐**:
- 跨地域网络 (RTT 50-100ms)
- 公网传输 (丢包率 0.5-1%)

⚠️ **可选**:
- 同城跨机房 (RTT 5-20ms)
- 低丢包网络 (< 0.1%)

❌ **不推荐**:
- 本地数据中心 (RTT < 5ms)
- 理想网络环境 (丢包率 ~0%)
