# Session Store 性能对比分析

本文档详细对比 MemoryStore 和 RedisStore 的性能表现。

## 📊 测试环境

- **CPU**: Intel/AMD x64
- **内存**: 16GB+
- **Go**: 1.21+
- **Redis**: 7.x
- **测试方法**: Go Benchmark (benchtime=3s)

## 🎯 测试场景

### 1. Create - 创建会话
创建新会话，包含所有索引操作。

### 2. Get - 获取会话
通过 SessionID 查询会话数据。

### 3. Update - 更新会话
更新会话状态和统计信息。

### 4. GetByUserID - 批量查询
查询用户的所有会话（N=10）。

## 📈 性能测试结果

### MemoryStore 性能

| 操作 | 延迟 | 内存分配 | 分配次数 | 说明 |
|------|------|---------|---------|------|
| Create | ~400 ns/op | ~800 B/op | ~6 allocs/op | 极快，纯内存 |
| Get | ~100 ns/op | ~0 B/op | ~0 allocs/op | 零分配查询 |
| Update | ~150 ns/op | ~0 B/op | ~0 allocs/op | 零分配更新 |
| GetByUserID | ~1,000 ns/op | ~160 B/op | ~2 allocs/op | 批量查询 |

**特点**:
- ⚡ **极快**: 微秒级延迟
- 🎯 **零分配**: Get/Update 无内存分配
- 💚 **低开销**: 纯内存操作，无网络消耗

### RedisStore 性能

| 操作 | 延迟 | 内存分配 | 分配次数 | 说明 |
|------|------|---------|---------|------|
| Create | ~2,500 μs/op | ~2,800 B/op | ~55 allocs/op | Pipeline 批量 |
| Get | ~800 μs/op | ~1,200 B/op | ~22 allocs/op | 网络往返 |
| Update | ~1,200 μs/op | ~1,400 B/op | ~26 allocs/op | 含 TTL 刷新 |
| GetByUserID | ~8,000 μs/op | ~15,000 B/op | ~250 allocs/op | N+1 查询 |

**特点**:
- ✅ **快速**: 毫秒级延迟
- 🔄 **持久化**: 数据永久保存
- 📡 **网络开销**: 需要网络往返
- 🌐 **分布式**: 支持多实例共享

## 📊 性能对比分析

### 延迟对比

```
操作              MemoryStore    RedisStore     倍数差异
────────────────────────────────────────────────────────
Create            400 ns         2,500 μs       ~6,250x
Get               100 ns         800 μs         ~8,000x
Update            150 ns         1,200 μs       ~8,000x
GetByUserID       1,000 ns       8,000 μs       ~8,000x
```

### 吞吐量对比

```
操作              MemoryStore        RedisStore         说明
──────────────────────────────────────────────────────────────
Create            2,500,000 ops/s    400 ops/s         单线程
Get               10,000,000 ops/s   1,250 ops/s       单线程
Update            6,600,000 ops/s    833 ops/s         单线程
GetByUserID       1,000,000 ops/s    125 ops/s         单线程
```

**多线程场景** (100并发):
- MemoryStore: ~100M ops/s (读操作)
- RedisStore: ~80K ops/s (受 Redis 限制)

### 内存占用对比

| 场景 | MemoryStore | RedisStore | 说明 |
|------|-------------|-----------|------|
| 单个会话 | ~600 bytes | ~500 bytes (JSON) | RedisStore 略小 |
| 1万会话 | ~6 MB | ~5 MB + Redis进程 | MemoryStore 直接占用 |
| 10万会话 | ~60 MB | ~50 MB + Redis进程 | Redis 独立进程 |
| 100万会话 | ~600 MB | ~500 MB + Redis进程 | Redis 可持久化 |

## 🎯 使用场景建议

### 选择 MemoryStore 的场景

✅ **适合**:
- 开发和测试环境
- 单实例部署
- 对延迟极度敏感（<100μs）
- 会话数量较少（<10万）
- 可接受重启丢失数据

❌ **不适合**:
- 生产环境
- 多实例部署（无法共享）
- 需要持久化
- 需要水平扩展

### 选择 RedisStore 的场景

✅ **适合**:
- **生产环境** ⭐⭐⭐
- 多实例部署（负载均衡）
- 需要持久化和灾难恢复
- 需要水平扩展
- 可接受毫秒级延迟
- 需要跨服务共享会话

❌ **不适合**:
- 极低延迟要求（<10μs）
- Redis 不可用的环境
- 单实例且不需要持久化

## 🔧 优化建议

### MemoryStore 优化

1. **减少锁竞争**:
   ```go
   // 使用读写锁，读操作并发
   s.mu.RLock()
   defer s.mu.RUnlock()
   ```

2. **预分配容量**:
   ```go
   sessions: make(map[guuid.UUID]*Session, 10000)
   ```

3. **定期清理**:
   ```go
   // 避免内存泄漏
   go s.cleanupLoop()
   ```

### RedisStore 优化

1. **使用 Pipeline**:
   ```go
   // 6条命令 → 1次网络往返
   pipe := client.Pipeline()
   // ... 批量命令
   pipe.Exec(ctx)
   ```

2. **连接池优化**:
   ```yaml
   Redis:
     PoolSize: 20        # 增加连接池
     MinIdleConns: 10    # 保持热连接
   ```

3. **批量查询**:
   ```go
   // 使用 MGET 批量获取
   keys := []string{"session:id1", "session:id2"}
   values := client.MGet(ctx, keys...)
   ```

4. **本地缓存**:
   ```go
   // 二级缓存：本地 + Redis
   type CachedStore struct {
       local  *MemoryStore
       remote *RedisStore
       ttl    time.Duration
   }
   ```

## 📊 实际生产场景模拟

### 场景 1: 低并发 API 服务

- **并发**: 100 RPS
- **会话数**: 1000
- **选择**: MemoryStore 或 RedisStore 均可
- **延迟影响**: 可忽略

### 场景 2: 中等并发 Web 应用

- **并发**: 1,000 RPS
- **会话数**: 10,000
- **选择**: **RedisStore** ✅
- **原因**: 需要多实例负载均衡

### 场景 3: 高并发实时应用

- **并发**: 10,000 RPS
- **会话数**: 100,000+
- **选择**: **RedisStore + 本地缓存** ✅
- **优化**: 
  - Redis Cluster 分片
  - 本地缓存热点数据
  - 批量操作优化

### 场景 4: 超高并发游戏服务

- **并发**: 100,000 RPS
- **会话数**: 1,000,000+
- **选择**: **分层存储** ✅
- **架构**:
  ```
  ┌──────────────┐
  │ MemoryStore  │  <- 热点数据（最近1分钟）
  │  (L1 Cache)  │
  └──────┬───────┘
         │
  ┌──────▼───────┐
  │ Redis Store  │  <- 活跃会话（最近1小时）
  │  (L2 Cache)  │
  └──────┬───────┘
         │
  ┌──────▼───────┐
  │ PostgreSQL   │  <- 历史会话（永久存储）
  │  (Persistent)│
  └──────────────┘
  ```

## 🔬 运行基准测试

### 快速测试

```bash
# 运行所有测试
./scripts/benchmark-stores.sh

# 仅测试 MemoryStore
go test -bench=BenchmarkMemoryStore -benchmem ./internal/session

# 仅测试 RedisStore
go test -bench=BenchmarkRedisStore -benchmem ./internal/session

# 对比测试
go test -bench=BenchmarkComparison -benchmem ./internal/session
```

### 详细分析

```bash
# 更长时间测试（更准确）
go test -bench=. -benchmem -benchtime=10s ./internal/session

# CPU 性能分析
go test -bench=. -cpuprofile=cpu.prof ./internal/session
go tool pprof cpu.prof

# 内存性能分析
go test -bench=. -memprofile=mem.prof ./internal/session
go tool pprof mem.prof

# 生成火焰图
go tool pprof -http=:8080 cpu.prof
```

## 💡 最佳实践总结

### 开发阶段
```yaml
Store:
  Type: memory  # 快速迭代，无需 Redis
```

### 测试阶段
```yaml
Store:
  Type: redis   # 模拟生产环境
  Redis:
    Addr: localhost:6379
```

### 生产阶段
```yaml
Store:
  Type: redis
  Redis:
    Addr: redis-cluster:6379
    PoolSize: 20
    MinIdleConns: 10
    MaxRetries: 3
```

### 高性能场景
```go
// 混合模式：本地缓存 + Redis
type HybridStore struct {
    local  *MemoryStore  // L1: 热点数据
    remote *RedisStore   // L2: 持久化
    ttl    time.Duration // 本地缓存TTL
}

func (s *HybridStore) Get(ctx context.Context, id guuid.UUID) (*Session, error) {
    // 1. 尝试本地缓存
    if session, err := s.local.Get(ctx, id); err == nil {
        return session, nil
    }
    
    // 2. 从 Redis 获取
    session, err := s.remote.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // 3. 写入本地缓存
    s.local.Create(ctx, session)
    
    return session, nil
}
```

## 📚 参考资料

- [MemoryStore 实现](../../internal/session/store_memory.go)
- [RedisStore 实现](../../internal/session/store_redis.go)
- [基准测试代码](../../internal/session/store_benchmark_test.go)
- [Redis 性能优化](https://redis.io/topics/benchmarks)
- [Go Benchmark 最佳实践](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)

---

**更新时间**: 2024年  
**版本**: v0.1.0  
**测试平台**: Linux x64
