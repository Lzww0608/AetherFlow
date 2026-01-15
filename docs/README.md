# AetherFlow 文档中心

本文档中心提供了AetherFlow项目的完整技术文档、开发指南和部署手册。

## 📚 文档导航

### 核心技术文档

| 文档 | 描述 | 适合读者 |
|------|------|----------|
| [QUANTUM_IMPLEMENTATION.md](./QUANTUM_IMPLEMENTATION.md) | Quantum协议的完整实现文档，包括协议设计、BBR拥塞控制、FEC前向纠错等 | 开发者、架构师 |
| [ARCHITECTURE.md](../ARCHITECTURE.md) | 系统架构设计文档，包括微服务架构、数据流设计、部署架构等 | 架构师、技术决策者 |

### 项目概览文档

| 文档 | 描述 | 适合读者 |
|------|------|----------|
| [README.md](../README.md) | 项目快速开始指南，介绍项目愿景、特性和快速上手 | 新用户、开发者 |
| [PROJECT_SUMMARY.md](../PROJECT_SUMMARY.md) | 项目总结和当前进度，包含已完成和待完成的功能 | 项目经理、开发者 |
| [ROADMAP.md](../ROADMAP.md) | 详细的开发路线图，包括各阶段的计划和任务 | 开发者、项目经理 |
| [PROJECT_STRUCTURE.md](../PROJECT_STRUCTURE.md) | 项目文件结构说明，帮助理解目录组织 | 开发者 |

### 组件文档

| 文档 | 描述 | 位置 |
|------|------|------|
| Session Service文档 | 会话管理服务的详细文档 | [internal/session/README.md](../internal/session/README.md) |
| Quantum协议示例 | Quantum协议的使用示例代码 | [examples/quantum/README.md](../examples/quantum/README.md) |
| Session Service示例 | 会话服务的使用示例代码 | [examples/session/README.md](../examples/session/README.md) |

---

## 🚀 快速开始

### 新手入门路线

1. **了解项目**: 阅读 [README.md](../README.md)
2. **理解架构**: 阅读 [ARCHITECTURE.md](../ARCHITECTURE.md)
3. **查看进度**: 阅读 [PROJECT_SUMMARY.md](../PROJECT_SUMMARY.md)
4. **了解规划**: 阅读 [ROADMAP.md](../ROADMAP.md)

### 开发者入门路线

1. **理解项目结构**: 阅读 [PROJECT_STRUCTURE.md](../PROJECT_STRUCTURE.md)
2. **学习核心协议**: 阅读 [QUANTUM_IMPLEMENTATION.md](./QUANTUM_IMPLEMENTATION.md)
3. **运行示例**: 查看 [examples/](../examples/) 目录下的示例
4. **开始开发**: 参考 [ROADMAP.md](../ROADMAP.md) 中的开发计划

### 面试准备路线

1. **项目概览**: [README.md](../README.md) + [PROJECT_SUMMARY.md](../PROJECT_SUMMARY.md)
2. **技术深度**: [QUANTUM_IMPLEMENTATION.md](./QUANTUM_IMPLEMENTATION.md)
3. **架构设计**: [ARCHITECTURE.md](../ARCHITECTURE.md)
4. **亮点总结**: 查看 [PROJECT_SUMMARY.md](../PROJECT_SUMMARY.md) 中的"面试展示要点"

---

## 📖 技术主题索引

### Quantum协议栈

- **协议设计**: [QUANTUM_IMPLEMENTATION.md](./QUANTUM_IMPLEMENTATION.md#协议包头结构-32字节--sack块)
- **可靠性机制**: SACK、快速重传、自适应RTO
- **拥塞控制**: BBR算法完整实现
- **前向纠错**: Reed-Solomon FEC
- **连接管理**: 三次握手、并发处理、优雅关闭

### 微服务架构

- **API网关**: GoZero框架、WebSocket、gRPC
- **会话服务**: 生命周期管理、心跳保活、存储抽象
- **状态同步**: 实时协作、冲突解决、操作日志
- **服务发现**: etcd集成、客户端负载均衡

### 云原生技术

- **容器化**: Docker多阶段构建
- **编排**: Kubernetes部署、StatefulSet、HPA
- **监控**: Prometheus指标、Grafana仪表盘、Alertmanager告警
- **配置**: ConfigMap、Secret、动态配置热更新

### 开发实践

- **项目结构**: [PROJECT_STRUCTURE.md](../PROJECT_STRUCTURE.md)
- **代码规范**: Go最佳实践
- **测试策略**: 单元测试、集成测试、性能测试
- **文档编写**: Markdown规范、API文档

---

## 🔍 按问题查找文档

### 我想了解...

| 问题 | 参考文档 |
|------|----------|
| AetherFlow是什么? | [README.md](../README.md) |
| 为什么要选择UDP而不是TCP? | [QUANTUM_IMPLEMENTATION.md](./QUANTUM_IMPLEMENTATION.md) |
| Quantum协议如何保证可靠性? | [QUANTUM_IMPLEMENTATION.md](./QUANTUM_IMPLEMENTATION.md#可靠性机制快速重传与选择性确认sack) |
| BBR拥塞控制是如何工作的? | [QUANTUM_IMPLEMENTATION.md](./QUANTUM_IMPLEMENTATION.md#bbr拥塞控制) |
| 如何实现实时状态同步? | [ROADMAP.md](../ROADMAP.md#311-数据模型设计) |
| 如何处理并发编辑冲突? | [ROADMAP.md](../ROADMAP.md#313-冲突解决机制) |
| 项目目前的完成度如何? | [PROJECT_SUMMARY.md](../PROJECT_SUMMARY.md) |
| 接下来要开发什么? | [ROADMAP.md](../ROADMAP.md) |
| 如何部署到Kubernetes? | [ROADMAP.md](../ROADMAP.md#51-kubernetes资源完善) |
| 如何进行性能测试? | [ROADMAP.md](../ROADMAP.md#61-性能基准测试) |

---

## 📝 文档维护

### 文档更新频率

- **PROJECT_SUMMARY.md**: 每个Phase完成后更新
- **ROADMAP.md**: 每月或重大变更时更新
- **组件文档**: 功能变更时及时更新
- **技术文档**: 架构变更时更新

### 贡献指南

1. 保持文档结构清晰
2. 使用简洁明了的语言
3. 添加代码示例和图表
4. 保持文档与代码同步
5. 使用Markdown格式

### 文档规范

- 标题层级: `#` `##` `###` `####`
- 代码块: 使用语言标识
- 链接: 使用相对路径
- 图表: 使用ASCII或Mermaid

---

## 🔗 外部资源

### 相关技术

- [Golang官方文档](https://golang.org/doc/)
- [GoZero框架文档](https://go-zero.dev/)
- [etcd官方文档](https://etcd.io/docs/)
- [Kubernetes文档](https://kubernetes.io/docs/)
- [Prometheus文档](https://prometheus.io/docs/)
- [Protocol Buffers](https://developers.google.com/protocol-buffers)

### 参考论文

- [BBR Congestion Control](https://queue.acm.org/detail.cfm?id=3022184)
- [QUIC: A UDP-Based Multiplexed and Secure Transport](https://datatracker.ietf.org/doc/html/rfc9000)
- [Reed-Solomon Forward Error Correction](https://en.wikipedia.org/wiki/Reed%E2%80%93Solomon_error_correction)
- [RFC 9562: UUIDv7 Specification](https://www.ietf.org/rfc/rfc9562.html)

---

## 📧 联系方式

如有文档相关问题或建议,请通过以下方式联系:

- 项目Issues: [GitHub Issues](https://github.com/your-repo/aetherflow/issues)
- 邮件: aetherflow@example.com
- 文档维护者: AetherFlow Team

---

**最后更新**: 2026-01-15
**文档版本**: v0.2.0
