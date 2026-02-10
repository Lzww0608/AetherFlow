# AetherFlow

一个技术密集型、云原生的低延迟数据同步架构方案，专为实时协作应用设计。

## 🌟 项目亮点

- **🚀 Quantum协议**: 自主实现的可靠UDP传输协议，集成BBR拥塞控制和FEC前向纠错
- **⚡ 超低延迟**: P99 < 50ms端到端延迟，专为实时协作优化
- **🏗️ 微服务架构**: 基于GoZero的云原生微服务，高可扩展性
- **☁️ 云原生**: 完整的Kubernetes部署，etcd服务发现，HPA自动伸缩
- **📊 可观测性**: Prometheus + Grafana + Jaeger链路追踪 + 结构化日志

## 📖 文档

| 文档 | 描述 |
|------|------|
| [PROJECT_SUMMARY.md](./PROJECT_SUMMARY.md) | **项目总结和当前进度** - 已完成和待完成功能 |
| [ROADMAP.md](./ROADMAP.md) | **开发路线图** - 详细的开发计划和任务 |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | **系统架构设计** - 微服务架构、数据流、部署架构 |
| [docs/README.md](./docs/README.md) | **文档中心** - 完整的技术文档索引 |

## 🎯 快速开始

### 环境要求

- Go 1.21+
- Docker (用于容器化部署)
- Kubernetes 1.28+ (用于生产部署)

### 本地开发

```bash
# 克隆项目
git clone https://github.com/your-repo/aetherflow.git
cd aetherflow

# 下载依赖
go mod download

# 运行测试
go test ./...

# 运行示例 - Quantum协议
cd examples/quantum
go run main.go

# 运行示例 - Session服务
cd examples/session
go run main.go
```

### 链路追踪快速开始

```bash
# 1. 启动 Jaeger
docker run -d --name jaeger \
  -p 16686:16686 -p 14268:14268 \
  jaegertracing/all-in-one:latest

# 2. 配置 Gateway (configs/gateway.yaml)
#    Tracing:
#      Enable: true
#      Exporter: jaeger
#      SampleRate: 1.0

# 3. 启动 Gateway
cd cmd/gateway
go run main.go -f ../../configs/gateway.yaml

# 4. 测试追踪
./scripts/test-tracing.sh

# 5. 查看 Jaeger UI
# 访问: http://localhost:16686
```

**详细文档**: 
- [链路追踪完整文档](internal/gateway/tracing/README.md)
- [快速开始指南](docs/TRACING_QUICK_START.md)
- [使用示例](examples/tracing/README.md)

## 📊 项目进度

### ✅ 已完成 (Phase 1-2)

#### Phase 1: Quantum协议核心 (100%)
- ✅ GUUID (UUIDv7) - 全局唯一标识符
- ✅ 协议头部 - 32字节紧凑包头
- ✅ 可靠性机制 - SACK + 快速重传 + 自适应RTO
- ✅ BBR拥塞控制 - 完整BBR状态机
- ✅ FEC前向纠错 - Reed-Solomon (10,3)方案
- ✅ 连接管理 - 三次握手、多goroutine并发
- ✅ 测试覆盖 - 平均65%+

#### Phase 2.1: Session Service (100%)
- ✅ 会话数据模型 - UUIDv7 + 状态机
- ✅ SessionManager - 完整生命周期管理
- ✅ 存储抽象 - Store接口 + MemoryStore实现
- ✅ gRPC API - 完整服务定义
- ✅ 单元测试 - 19个测试用例

### 🚧 进行中 (Phase 3)

#### Phase 3: 微服务核心功能 (开发中)
- 🚧 StateSync Service - 状态同步、冲突解决
- 🚧 API Gateway - GoZero集成、WebSocket支持
- 🚧 etcd集成 - 服务发现、负载均衡

### 📅 计划中 (Phase 4-6)

- Phase 4: etcd服务发现 + 客户端负载均衡
- Phase 5: 完整Kubernetes部署 + HPA
- Phase 6: 性能优化 + 生产就绪

**详细计划**: 查看 [ROADMAP.md](./ROADMAP.md)

## 🏗️ 项目结构

```
AetherFlow/
├── api/                    # API定义
│   ├── proto/             # Protocol Buffers
│   └── openapi/           # OpenAPI规范
├── cmd/                   # 应用入口
│   ├── api-gateway/
│   ├── session-service/
│   └── statesync-service/
├── internal/              # 私有代码
│   ├── quantum/          # Quantum协议核心
│   │   ├── protocol/     # 协议头部
│   │   ├── bbr/          # BBR拥塞控制
│   │   ├── fec/          # 前向纠错
│   │   ├── reliability/   # 可靠性机制
│   │   └── transport/    # UDP传输层
│   ├── session/          # 会话管理服务
│   └── statesync/        # 状态同步服务 (待实现)
├── pkg/                   # 公共库
│   ├── guuid/            # UUIDv7实现
│   └── utils/            # 工具函数
├── configs/              # 配置文件
├── deployments/          # 部署配置
│   ├── docker/
│   ├── kubernetes/
│   └── helm/
├── docs/                 # 文档
├── examples/             # 示例代码
└── tests/               # 测试
```

**详细结构**: 查看 [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md)

## 🎯 核心技术

### Quantum协议

一个从零开始实现的可靠UDP传输协议，专为低延迟场景设计:

```go
// 客户端连接
config := quantum.DefaultConfig()
config.FECEnabled = true
conn, err := quantum.Dial("udp", "server:9090", config)

// 发送数据
err = conn.Send([]byte("Hello, Quantum!"))

// 接收数据
data, err := conn.Receive()
```

**核心特性**:
- 32字节紧凑包头
- SACK选择性确认
- BBR拥塞控制 (STARTUP/DRAIN/PROBE_BW/PROBE_RTT)
- Reed-Solomon FEC (10,3) - 可丢失任意3个包
- 自适应RTO (RFC 6298)

**详细文档**: [QUANTUM_IMPLEMENTATION.md](./docs/QUANTUM_IMPLEMENTATION.md)

### Session Service

完整的用户会话管理:

```go
// 创建会话
session, token, err := manager.CreateSession(
    ctx,
    "user123",
    "192.168.1.100",
    9090,
    connectionID,
    nil,
)

// 心跳保活
remaining, err := manager.Heartbeat(ctx, sessionID)
```

**核心特性**:
- UUIDv7会话ID
- 5种会话状态 (CONNECTING/ACTIVE/IDLE/DISCONNECTING/CLOSED)
- 自动过期清理
- 多级索引 (SessionID/ConnectionID/UserID)

**详细文档**: [internal/session/README.md](./internal/session/README.md)

## 📊 性能指标

| 指标 | 目标值 | 当前状态 |
|------|--------|----------|
| 端到端延迟 | P99 < 50ms | 🟡 待测试 |
| 吞吐量 | > 100 Mbps | 🟡 待测试 |
| 会话查询延迟 | < 1ms | ✅ O(1)索引 |
| 会话创建延迟 | < 5ms | ✅ 高效实现 |
| 数据包恢复 | < 10ms | ✅ FEC实现 |
| 可用性 | 99.9% | 🟡 待部署 |

**测试覆盖**: Quantum协议 ~65%, Session Service ~81%

## 🛠️ 技术栈

### 已使用
- **语言**: Go 1.21+
- **核心库**:
  - `github.com/Lzww0608/GUUID` - UUIDv7
  - `github.com/klauspost/reedsolomon` - FEC
  - `go.uber.org/zap` - 结构化日志

### 规划中
- **框架**: GoZero
- **协调**: etcd v3.5+
- **监控**: Prometheus + Grafana
- **容器**: Docker + Kubernetes 1.28+

## 🚀 下一步

### 优先级 P0 - 立即开始 (Phase 3)
1. **StateSync Service** - 实现状态同步、冲突解决
2. **API Gateway** - GoZero集成、WebSocket支持

### 优先级 P1 - 高优先级 (Phase 4)
3. **etcd服务发现** - 服务注册、客户端负载均衡
4. **监控指标完善** - StateSync/Gateway指标

### 优先级 P2 - 中优先级 (Phase 5-6)
5. **完整Kubernetes部署** - HPA、多环境配置
6. **性能测试和优化** - 压力测试、性能调优

**详细计划**: [ROADMAP.md](./ROADMAP.md)

## 💡 面试展示要点

### 技术深度
- **底层网络编程**: 从零实现可靠UDP协议 (包头设计、BBR、FEC)
- **分布式系统**: etcd服务发现、客户端负载均衡、分布式锁
- **云原生**: Kubernetes原生、HPA自动伸缩、完整监控

### 工程能力
- **项目规划**: 清晰的模块划分和依赖关系
- **代码质量**: 完善的测试覆盖 (平均65%+)
- **文档完整**: 从协议设计到API使用的完整文档链

### 技术决策
- **为什么选择UDP**: 避免TCP队头阻塞，降低延迟
- **为什么使用UUIDv7**: 时间排序、去中心化、标准化
- **为什么选择BBR**: 现代拥塞控制，适应高带宽延迟积网络

**详细总结**: [PROJECT_SUMMARY.md](./PROJECT_SUMMARY.md#面试展示要点)

## 📚 更多文档

- [QUANTUM_IMPLEMENTATION.md](./docs/QUANTUM_IMPLEMENTATION.md) - Quantum协议详解
- [ARCHITECTURE.md](./ARCHITECTURE.md) - 系统架构设计
- [docs/README.md](./docs/README.md) - 文档中心
- [examples/](./examples/) - 示例代码

## 🤝 贡献

欢迎贡献! 请查看 [CONTRIBUTING.md](./CONTRIBUTING.md) 了解如何参与。

## 📄 许可证

MIT License - 详见 [LICENSE](./LICENSE) 文件

## 📧 联系方式

- 项目地址: [GitHub](https://github.com/your-repo/aetherflow)
- 问题反馈: [Issues](https://github.com/your-repo/aetherflow/issues)
- 邮件: aetherflow@example.com

---

**版本**: v0.2.0-alpha
**最后更新**: 2026-01-15
