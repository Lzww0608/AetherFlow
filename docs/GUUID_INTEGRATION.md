# GUUID集成说明

## 概述

AetherFlow项目使用了 [github.com/Lzww0608/GUUID](https://github.com/Lzww0608/GUUID) 作为全局唯一标识符的实现。这是一个高性能、符合标准的UUID库，专门支持最新的UUIDv7规范。

## 为什么选择UUIDv7？

相比传统的UUID版本和其他分布式ID生成方案（如Snowflake），UUIDv7具有以下优势：

### UUIDv7 vs 传统UUID

| 特性 | UUIDv4 (随机) | UUIDv7 (时间排序) |
|------|--------------|------------------|
| 排序性 | ❌ 完全随机 | ✅ 按时间排序 |
| 数据库性能 | 差（B-tree碎片） | 优（顺序插入） |
| 时间信息 | 无 | 毫秒级时间戳 |
| 冲突概率 | 极低 | 极低 |
| 大小 | 128-bit | 128-bit |

### UUIDv7 vs Snowflake

| 特性 | Snowflake | UUIDv7 |
|------|-----------|--------|
| ID大小 | 64-bit | 128-bit |
| 排序性 | ✅ 时间排序 | ✅ 时间排序 |
| 节点管理 | ❌ 需要中心化分配Worker ID | ✅ 完全去中心化 |
| 冲突避免 | 固定序列号（4096/ms限制） | 概率性（74-bit随机数） |
| 运维成本 | 高（需要协调节点ID） | 零（无需协调） |
| 生成速率 | 有硬限制 | 无限制 |

## 在Quantum协议中的应用

UUIDv7在AetherFlow的Quantum协议中用于：

### 1. 连接标识 (Connection ID)

每个Quantum连接都有一个唯一的UUIDv7标识符，用于：
- UDP包的多路复用和解复用
- 无状态服务器上的连接识别
- 连接会话管理

```go
// 创建新连接时自动生成UUIDv7
conn, err := quantum.Dial("udp", "server:9090", config)
guid := conn.GUID() // 返回 guuid.UUID (UUIDv7)
```

### 2. 分布式追踪 (Trace ID)

UUIDv7作为统一的追踪标识贯穿整个分布式系统：
- 请求链路追踪
- 日志关联
- 性能分析
- 安全审计

```go
// 每个协议包都携带GUUID
type Header struct {
    MagicNumber    uint32
    Version        uint8
    Flags          Flags
    GUUID          guuid.UUID  // 16字节 UUIDv7
    SequenceNumber uint32
    AckNumber      uint32
    // ...
}
```

### 3. 时间排序的优势

由于UUIDv7内置时间戳（毫秒精度），它提供了天然的排序能力：

```
连接按时间顺序：
- 018d1234-5678-7abc-def0-123456789012  (早)
- 018d1234-5679-1234-5678-abcdef012345
- 018d1234-567a-9876-5432-fedcba987654  (晚)
```

这对以下场景特别有用：
- 数据库主键（B-tree性能优化）
- 事件排序
- 时间线重建
- 调试和问题追踪

## UUIDv7结构

UUIDv7的128位结构如下：

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    unix_ts_ms (48 bits)                       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|unix_ts_ms |  ver  |       rand_a (12 bits)                    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|var|                  rand_b (62 bits)                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     rand_b (continued)                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

- **unix_ts_ms (48 bits)**: Unix时间戳毫秒数
- **ver (4 bits)**: 版本号 (7)
- **rand_a (12 bits)**: 随机数A (用于毫秒内排序)
- **var (2 bits)**: 变体标识
- **rand_b (62 bits)**: 随机数B (保证唯一性)

## API使用

### 生成UUIDv7

```go
import guuid "github.com/Lzww0608/GUUID"

// 生成新的UUIDv7
uuid, err := guuid.NewV7()
if err != nil {
    log.Fatal(err)
}

// 转换为字符串
str := uuid.String()  // "018d1234-5678-7abc-def0-123456789012"
```

### 解析和验证

```go
// 从字符串解析
uuid, err := guuid.Parse("018d1234-5678-7abc-def0-123456789012")

// 验证UUID类型
if uuid.Version() == 7 {
    // 这是UUIDv7
}

// 比较
if uuid1 == uuid2 {
    // 相同
}

// 检查空值
if uuid == guuid.Nil {
    // 空UUID
}
```

### 在Quantum协议中使用

```go
// 创建连接时自动生成
conn, err := quantum.Dial("udp", "server:9090", config)

// 获取连接的GUID
guid := conn.GUID()
fmt.Printf("Connection ID: %s\n", guid.String())

// 创建数据包（自动使用连接的GUID）
err = conn.Send([]byte("Hello"))
```

## 性能特性

根据GUUID库的benchmark测试：

| 操作 | 性能 | 说明 |
|------|------|------|
| NewV7() | ~100-200 ns/op | 生成新UUIDv7 |
| String() | ~20-50 ns/op | 转换为字符串 |
| Parse() | ~50-100 ns/op | 从字符串解析 |
| 内存分配 | 0 allocs/op | 生成时零内存分配 |

### 并发安全

- ✅ 完全线程安全
- ✅ 无锁设计（使用原子操作）
- ✅ 高并发场景下性能稳定

### 冲突概率

UUIDv7的冲突概率极低：
- 同一毫秒内：74-bit随机空间，概率 < 2^-74
- 不同毫秒：时间戳+随机数，实际上不可能冲突
- 全局唯一性：在分布式系统中无需协调

## 标准符合性

GUUID库完全符合以下标准：
- ✅ RFC 4122 (UUID标准)
- ✅ RFC 9562 Draft (UUIDv7规范)
- ✅ 字符串格式与UUID标准兼容

## 最佳实践

### 1. 用于数据库主键

```go
type Connection struct {
    ID        guuid.UUID `gorm:"primaryKey;type:uuid"`
    CreatedAt time.Time
    // ...
}
```

### 2. 用于分布式追踪

```go
// 在日志中包含GUID
logger.Info("Processing request",
    zap.String("trace_id", conn.GUID().String()),
    zap.String("action", "send_data"),
)
```

### 3. 用于时间排序

```go
// UUIDv7可以直接用于时间排序
SELECT * FROM connections 
ORDER BY id ASC;  -- 按创建时间排序
```
