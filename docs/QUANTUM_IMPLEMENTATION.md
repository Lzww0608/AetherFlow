# Quantum协议实现文档

## 概述

本文档描述了AetherFlow项目中Quantum协议的完整实现。Quantum是一个基于UDP的可靠传输协议,集成了BBR拥塞控制、SACK快速重传和Reed-Solomon前向纠错等先进特性。

## 实现状态

✅ **已完成的组件:**

1. **GUUID (pkg/guuid)** - 全局唯一标识符
   - 16字节唯一标识符
   - 支持时间戳嵌入
   - 用于连接标识和分布式追踪
   - 测试覆盖率: 86.4%

2. **Protocol (internal/quantum/protocol)** - 协议头部定义
   - 完整的包头序列化/反序列化
   - SACK块支持
   - 标志位管理
   - 测试覆盖率: 84.1%

3. **Transport (internal/quantum/transport)** - UDP传输层
   - UDP连接封装
   - 数据包发送/接收
   - 包池优化
   - 连接统计

4. **Reliability (internal/quantum/reliability)** - 可靠性机制
   - 发送缓冲区管理
   - 接收缓冲区管理
   - SACK生成和处理
   - 快速重传检测
   - RTO自适应计算
   - 测试覆盖率: 27.9%

5. **BBR (internal/quantum/bbr)** - 拥塞控制
   - 完整的BBR状态机 (STARTUP, DRAIN, PROBE_BW, PROBE_RTT)
   - 带宽估计
   - RTT探测
   - Pacing控制
   - 动态窗口调整
   - 测试覆盖率: 71.1%

6. **FEC (internal/quantum/fec)** - 前向纠错
   - Reed-Solomon编码/解码
   - 动态分组管理
   - 自动恢复丢失数据包
   - 支持可配置的数据/校验分片比例
   - 测试覆盖率: 78.4%

7. **Connection (internal/quantum)** - 完整连接管理
   - 集成所有协议组件
   - 三次握手连接建立
   - 多goroutine并发处理
   - 自动重传和ACK
   - Keepalive机制

## 架构设计

### 分层架构

```
┌─────────────────────────────────────────┐
│         Application Layer               │
│         (Connection API)                │
├─────────────────────────────────────────┤
│      Congestion Control (BBR)           │
├─────────────────────────────────────────┤
│    Reliability (SACK + Retransmit)      │
├─────────────────────────────────────────┤
│    Forward Error Correction (FEC)       │
├─────────────────────────────────────────┤
│         Protocol (Header)               │
├─────────────────────────────────────────┤
│        Transport (UDP)                  │
└─────────────────────────────────────────┘
```

### 协议包头格式 (32字节 + SACK块)

| 字段             | 偏移 | 大小   | 描述                    |
|-----------------|------|--------|------------------------|
| MagicNumber     | 0    | 4 字节 | 0x51554E54 ("QUNT")    |
| Version         | 4    | 1 字节 | 协议版本号              |
| Flags           | 5    | 1 字节 | 控制标志位              |
| GUUID           | 6    | 16字节 | 连接唯一标识符          |
| SequenceNumber  | 22   | 4 字节 | 数据包序列号            |
| AckNumber       | 26   | 4 字节 | 确认号                  |
| PayloadLength   | 30   | 2 字节 | 载荷长度                |
| SACK Blocks     | 32   | 可变   | 选择性确认块(最多8个)   |

### 控制标志位

- `FlagSYN` (0x01): 连接建立
- `FlagACK` (0x02): 确认
- `FlagFIN` (0x04): 连接终止
- `FlagRST` (0x08): 连接重置
- `FlagFEC` (0x10): FEC启用
- `FlagPSH` (0x20): 推送数据
- `FlagURG` (0x40): 紧急数据
- `FlagECE` (0x80): ECN回显

## 核心特性

### 1. BBR拥塞控制

BBR (Bottleneck Bandwidth and RTT) 是Google开发的先进拥塞控制算法,通过估计瓶颈带宽和最小RTT来控制发送速率。

**状态机:**
- **STARTUP**: 指数增长探测带宽
- **DRAIN**: 排空启动阶段积累的队列
- **PROBE_BW**: 周期性探测可用带宽(稳态)
- **PROBE_RTT**: 主动降低in-flight数据以测量准确RTT

**关键参数:**
- StartupGain: 2.77
- ProbeBW增益周期: [1.25, 0.75, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0]
- ProbeRTT间隔: 10秒

### 2. SACK和快速重传

**选择性确认 (SACK):**
- 支持最多8个SACK块
- 精确报告非连续接收的数据段
- 避免不必要的重传

**快速重传:**
- 检测到3个后续包被确认时触发
- 不等待RTO超时
- 显著降低重传延迟

**RTO计算:**
- 使用RFC 6298算法
- SRTT和RTTVAR自适应更新
- 范围: 200ms - 60s

### 3. 前向纠错 (FEC)

**Reed-Solomon编码:**
- 默认配置: (10, 3) - 10个数据分片 + 3个校验分片
- 可配置的数据/校验比例
- 任意10个分片即可恢复全部数据

**优势:**
- 在有损网络上减少重传
- 降低延迟
- 平滑带宽使用

**开销:**
- 默认配置: 30%带宽开销
- 可根据网络质量动态调整

### 4. 连接管理

**连接建立 (三次握手):**
1. Client -> Server: SYN
2. Server -> Client: SYN-ACK
3. Client -> Server: ACK

**数据传输:**
- 异步发送队列
- Pacing控制发送速率
- 自动重传管理
- 有序数据交付

**连接维护:**
- 定期Keepalive (默认10秒)
- 空闲超时检测 (默认60秒)
- 优雅关闭 (FIN握手)

## 性能特性

### 测试结果

所有核心组件已通过单元测试:

```
✅ pkg/guuid              - 7/7 tests passed   (86.4% coverage)
✅ internal/quantum/protocol  - 5/5 tests passed   (84.1% coverage)
✅ internal/quantum/reliability - 5/5 tests passed (27.9% coverage)
✅ internal/quantum/bbr       - 7/7 tests passed   (71.1% coverage)
✅ internal/quantum/fec       - 6/6 tests passed   (78.4% coverage)
```

### 性能目标

- **延迟**: P99 < 50ms (端到端)
- **吞吐量**: > 100 Mbps per connection
- **丢包恢复**: < 10ms (通过FEC)
- **CPU效率**: < 5% per connection @ 1Gbps
- **内存占用**: < 10MB per connection

### 优化特性

1. **零拷贝**: 尽可能减少内存拷贝
2. **包池**: 使用sync.Pool减少GC压力
3. **批量处理**: 批量发送和接收数据包
4. **精确Pacing**: 使用time.Ticker实现平滑发送

## API使用示例

### 创建连接

```go
// 客户端
config := quantum.DefaultConfig()
config.FECEnabled = true
config.FECDataShards = 10
config.FECParityShards = 3

conn, err := quantum.Dial("udp", "server:9090", config)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// 发送数据
err = conn.Send([]byte("Hello, Quantum!"))

// 接收数据
data, err := conn.Receive()
```

### 服务端

```go
// 监听
conn, err := quantum.Listen("udp", ":9090", nil)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// 接收数据
data, err := conn.Receive()

// 发送响应
err = conn.Send([]byte("Response"))
```

## 配置选项

### Connection Config

```go
type Config struct {
    // 窗口大小
    SendWindow uint32  // 发送窗口 (默认: 256 packets)
    RecvWindow uint32  // 接收窗口 (默认: 256 packets)
    
    // 超时设置
    KeepaliveInterval time.Duration  // Keepalive间隔 (默认: 10s)
    IdleTimeout       time.Duration  // 空闲超时 (默认: 60s)
    
    // FEC配置
    FECEnabled       bool  // 是否启用FEC (默认: true)
    FECDataShards    int   // 数据分片数 (默认: 10)
    FECParityShards  int   // 校验分片数 (默认: 3)
    
    // BBR配置
    BBRConfig *bbr.Config  // BBR参数
    
    // Transport配置
    TransportConfig *transport.Config  // UDP参数
}
```

### BBR Config

```go
type Config struct {
    InitialCwnd  uint32        // 初始拥塞窗口 (默认: 10 packets)
    MinRTT       time.Duration // 最小RTT提示 (默认: 10ms)
    MaxBandwidth uint64        // 最大带宽提示 (默认: 100 MB/s)
}
```

## 监控指标

### 连接统计

```go
stats := conn.Statistics()
// PacketsSent: 发送的数据包总数
// PacketsReceived: 接收的数据包总数
// BytesSent: 发送的字节总数
// BytesReceived: 接收的字节总数
// PacketsLost: 丢失的数据包数
// PacketsRecovered: FEC恢复的数据包数
// Retransmissions: 重传次数
```

### BBR统计

```go
bbrStats := conn.BBRStats()
// state: 当前BBR状态
// btl_bw_mbps: 瓶颈带宽 (Mbps)
// rtt_ms: 最小RTT (ms)
// pacing_rate: 当前发送速率 (bytes/sec)
// send_window: 发送窗口大小 (bytes)
// cwnd_packets: 拥塞窗口 (packets)
```

