# AetherFlow 端到端示例与演示

这是 AetherFlow 的完整端到端演示，展示实时协作、冲突解决、低延迟同步等核心功能。

## 📋 目录

- [快速开始](#快速开始)
- [演示场景](#演示场景)
- [架构说明](#架构说明)
- [使用指南](#使用指南)

## 🚀 快速开始

### 前置条件

- Docker & Docker Compose
- Go 1.21+
- 浏览器（支持 WebSocket）

### 一键启动

```bash
# 1. 进入示例目录
cd examples/e2e

# 2. 启动所有服务
./start-all.sh

# 3. 等待服务就绪（约 30 秒）
# 输出会显示各服务的状态和访问地址

# 4. 打开浏览器
open http://localhost:8080
```

### 手动启动（用于开发）

```bash
# 1. 启动基础设施（Redis, PostgreSQL, Jaeger）
docker-compose up -d redis postgres jaeger

# 2. 启动后端服务
./start-services.sh

# 3. 启动 Web 服务器
cd web
python3 -m http.server 8080
```

## 🎬 演示场景

### 场景 1: 实时协作编辑

**演示内容**: 多个用户同时编辑同一个文档

```bash
# 运行协作演示
go run demo-collaboration.go

# 输出示例:
# ✅ User Alice 创建文档: doc-001
# ✅ User Bob 连接到文档
# ✅ User Carol 连接到文档
# 📝 Alice 编辑: "Hello"
# 📝 Bob 编辑: "World"
# ✅ 所有用户收到实时更新
# 📊 操作延迟: P50=15ms, P99=45ms
```

**演示功能**:
- ✅ 实时同步（<50ms 延迟）
- ✅ 多用户在线状态
- ✅ 操作历史记录
- ✅ 版本号管理

### 场景 2: 冲突检测与解决

**演示内容**: 两个用户同时编辑产生冲突，系统自动解决

```bash
# 运行冲突演示
go run demo-conflict.go

# 输出示例:
# ✅ User Alice 和 Bob 同时连接
# ⚠️  检测到冲突: 两个用户同时编辑位置 [10]
# 🔧 应用冲突解决策略: LWW (Last Write Wins)
# ✅ 冲突已解决
# 📝 最终状态: "Hello World" (Bob 的版本)
```

**演示功能**:
- ✅ 冲突检测
- ✅ LWW 策略
- ✅ 冲突历史
- ✅ 自动重试

### 场景 3: Web UI 实时协作

**演示内容**: 通过浏览器体验实时协作

1. **打开多个浏览器标签页** (模拟多用户):
   ```
   http://localhost:8080
   ```

2. **登录不同用户**:
   - Tab 1: 用户名 "Alice"
   - Tab 2: 用户名 "Bob"
   - Tab 3: 用户名 "Carol"

3. **创建或加入文档**:
   - Alice 创建文档 "Meeting Notes"
   - Bob 和 Carol 输入文档 ID 加入

4. **实时编辑**:
   - 在任意标签页中输入文字
   - 观察其他标签页实时显示
   - 查看活跃用户列表
   - 查看操作历史

5. **锁定机制**:
   - Alice 点击"锁定"按钮
   - Bob 和 Carol 看到文档被锁定
   - Alice 释放锁后其他人可编辑

**演示功能**:
- ✅ WebSocket 实时连接
- ✅ 富文本编辑器
- ✅ 在线用户显示
- ✅ 操作历史面板
- ✅ 锁定/解锁
- ✅ 链路追踪可视化

### 场景 4: 性能压测

**演示内容**: 高并发场景下的性能表现

```bash
# 运行性能测试
go run demo-performance.go

# 输出示例:
# 🚀 启动 100 个并发用户
# 📊 创建 1000 个文档
# 📝 执行 10000 次操作
# 
# 性能指标:
# - 平均延迟: 18ms
# - P99 延迟: 45ms
# - 吞吐量: 5500 ops/sec
# - 成功率: 99.98%
```

## 🏗️ 架构说明

### 系统架构

```
┌─────────────┐
│   浏览器     │ (WebSocket)
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────────────┐
│           API Gateway (8000)                     │
│  - WebSocket Handler                             │
│  - JWT 认证                                      │
│  - 链路追踪                                      │
└──────┬──────────────────────┬───────────────────┘
       │                      │
       ▼                      ▼
┌─────────────┐        ┌─────────────┐
│   Session    │        │  StateSync  │
│   Service    │        │   Service   │
│   (9001)     │        │   (9002)    │
└──────┬───────┘        └──────┬──────┘
       │                       │
       ▼                       ▼
  ┌────────┐            ┌──────────┐
  │ Redis  │            │PostgreSQL│
  └────────┘            └──────────┘
```

### 数据流

**1. 用户连接流程**:
```
Browser → WebSocket → Gateway → Session Service → Redis
                                      ↓
                                 Create Session
                                      ↓
                                 Return Token
```

**2. 文档编辑流程**:
```
Browser → Operation → Gateway → StateSync Service
                                      ↓
                                 Apply Operation
                                      ↓
                               Save to PostgreSQL
                                      ↓
                              Broadcast to Users
                                      ↓
Browser ← WebSocket ← Gateway ← Event Stream
```

**3. 冲突解决流程**:
```
User A → Operation 1 ─┐
                      ├→ StateSync → Detect Conflict
User B → Operation 2 ─┘              ↓
                                 Resolve (LWW)
                                     ↓
                              Apply Winner
                                     ↓
                          Broadcast Resolution
```

## 📖 使用指南

### 运行协作演示

```bash
go run demo-collaboration.go \
  --users 3 \
  --duration 60s \
  --operations 100
```

**参数说明**:
- `--users`: 并发用户数（默认 3）
- `--duration`: 运行时长（默认 60s）
- `--operations`: 操作次数（默认 100）

### 运行冲突演示

```bash
go run demo-conflict.go \
  --conflict-type concurrent \
  --resolution lww
```

**参数说明**:
- `--conflict-type`: 冲突类型（concurrent, sequential）
- `--resolution`: 解决策略（lww, manual, merge）

### 自定义配置

创建 `config.yaml`:

```yaml
gateway:
  host: localhost
  port: 8000
  
session:
  host: localhost
  port: 9001
  
statesync:
  host: localhost
  port: 9002

demo:
  users: 5
  documents: 10
  operations: 1000
  conflict_rate: 0.1  # 10% 操作产生冲突
```

运行演示:
```bash
go run demo-collaboration.go -f config.yaml
```

## 🔍 监控与调试

### 查看链路追踪

1. 打开 Jaeger UI:
   ```
   http://localhost:16686
   ```

2. 选择服务: `aetherflow-gateway`

3. 查看 Trace:
   - 创建文档流程
   - 应用操作流程
   - 冲突解决流程

### 查看 Metrics

1. 打开 Prometheus:
   ```
   http://localhost:9090
   ```

2. 查询指标:
   ```promql
   # 操作延迟
   histogram_quantile(0.99, gateway_operation_duration_seconds_bucket)
   
   # 吞吐量
   rate(gateway_operations_total[1m])
   
   # 冲突率
   rate(statesync_conflicts_total[1m]) / rate(statesync_operations_total[1m])
   ```

### 查看日志

```bash
# Gateway 日志
docker-compose logs -f gateway

# Session Service 日志
docker-compose logs -f session-service

# StateSync Service 日志
docker-compose logs -f statesync-service
```

### 数据库查询

```bash
# 查看活跃文档
psql -h localhost -U postgres -d aetherflow -c \
  "SELECT id, name, type, version FROM documents WHERE state = 'active';"

# 查看操作历史
psql -h localhost -U postgres -d aetherflow -c \
  "SELECT id, type, version, status FROM operations ORDER BY timestamp DESC LIMIT 10;"

# 查看冲突
psql -h localhost -U postgres -d aetherflow -c \
  "SELECT id, resolution, resolved_at FROM conflicts WHERE resolved_at IS NULL;"
```

## 🎯 演示要点

### 核心价值展示

1. **低延迟**:
   - 操作延迟 P99 < 50ms
   - WebSocket 实时推送
   - 本地缓存优化

2. **可靠性**:
   - 自动冲突检测
   - 智能冲突解决
   - 操作历史可追溯

3. **可扩展性**:
   - 微服务架构
   - 水平扩展
   - 无状态设计

4. **可观测性**:
   - 完整链路追踪
   - 丰富的 Metrics
   - 结构化日志

### 演示技巧

1. **准备工作**:
   - 提前启动所有服务
   - 检查健康状态
   - 清理测试数据

2. **演示顺序**:
   - 先展示基础功能（创建、编辑）
   - 再展示核心特性（实时同步）
   - 最后展示高级功能（冲突解决）

3. **可视化**:
   - 使用多个浏览器窗口并排显示
   - 打开 Jaeger UI 展示追踪
   - 使用终端显示实时日志

4. **故障演示**:
   - 模拟网络延迟
   - 模拟服务故障
   - 展示自动恢复

## 🐛 故障排查

### 问题 1: 服务启动失败

**检查**:
```bash
# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs gateway
```

**解决**: 确保端口未被占用，检查配置文件

### 问题 2: WebSocket 连接失败

**检查**:
```bash
# 测试 WebSocket 连接
wscat -c ws://localhost:8000/ws
```

**解决**: 检查 Gateway 配置和防火墙

### 问题 3: 数据不同步

**检查**:
```bash
# 查看 StateSync 日志
docker-compose logs statesync-service | grep ERROR
```

**解决**: 检查数据库连接和订阅机制

## 📚 相关文档

- [Gateway 使用指南](../../docs/GATEWAY_GUIDE.md)
- [Session Service 文档](../../docs/SESSION_SERVICE.md)
- [StateSync Service 文档](../../docs/STATESYNC_SERVICE.md)
- [WebSocket API 规范](../../docs/WEBSOCKET_API.md)

## 🎓 扩展实验

### 实验 1: 自定义冲突策略

修改 `demo-conflict.go`，实现自定义冲突解决策略：

```go
type CustomConflictResolver struct{}

func (r *CustomConflictResolver) Resolve(ops []*Operation) (*Operation, error) {
    // 自定义逻辑：优先级最高的用户获胜
    // 或者：合并所有操作
    // 或者：投票机制
}
```

### 实验 2: 压力测试

增加并发用户数，观察性能表现：

```bash
# 100 用户
go run demo-performance.go --users 100

# 1000 用户
go run demo-performance.go --users 1000

# 记录性能指标
```

### 实验 3: 网络延迟模拟

使用 `tc` 命令模拟网络延迟：

```bash
# 添加 100ms 延迟
sudo tc qdisc add dev lo root netem delay 100ms

# 运行演示
go run demo-collaboration.go

# 移除延迟
sudo tc qdisc del dev lo root
```

### 实验 4: 服务故障恢复

模拟服务故障：

```bash
# 1. 启动演示
go run demo-collaboration.go &

# 2. 停止 StateSync Service
docker-compose stop statesync-service

# 3. 观察客户端行为

# 4. 恢复服务
docker-compose start statesync-service

# 5. 验证数据一致性
```

## 📝 总结

这个端到端演示全面展示了 AetherFlow 的核心能力：

- ✅ **实时协作**: WebSocket + 低延迟同步
- ✅ **冲突解决**: 自动检测 + 智能解决
- ✅ **可靠性**: 持久化 + 事务保证
- ✅ **可扩展性**: 微服务 + 水平扩展
- ✅ **可观测性**: 追踪 + 指标 + 日志

通过这个演示，可以直观地理解 AetherFlow 如何解决实时协作场景中的技术挑战。

---

**需要帮助？** 查看 [故障排查](#故障排查) 或提交 Issue。
