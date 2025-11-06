# AetherFlow 项目文件结构

## 项目目录概览

```
AetherFlow/
├── api/                          # API定义和文档
│   ├── openapi/                  # OpenAPI/Swagger规范
│   └── proto/                    # Protocol Buffers定义
├── cmd/                          # 应用程序入口点
│   ├── api-gateway/              # API网关服务入口
│   ├── session-service/          # 会话服务入口
│   └── statesync-service/        # 状态同步服务入口
├── configs/                      # 配置文件
│   ├── etcd/                     # etcd配置
│   ├── grafana/                  # Grafana仪表盘配置
│   └── prometheus/               # Prometheus监控配置
├── deployments/                  # 部署相关文件
│   ├── docker/                   # Docker相关文件
│   ├── helm/                     # Helm Charts
│   └── kubernetes/               # Kubernetes部署文件
│       ├── base/                 # 基础Kubernetes资源
│       └── overlays/             # Kustomize覆盖配置
│           ├── dev/              # 开发环境配置
│           ├── prod/             # 生产环境配置
│           └── staging/          # 预发布环境配置
├── docs/                         # 项目文档
├── examples/                     # 示例代码和用法
├── internal/                     # 私有应用程序代码
│   ├── config/                   # 配置管理
│   ├── discovery/                # 服务发现
│   ├── gateway/                  # API网关实现
│   ├── metrics/                  # 指标收集
│   ├── quantum/                  # Quantum协议实现
│   │   ├── bbr/                  # BBR拥塞控制算法
│   │   ├── fec/                  # 前向纠错实现
│   │   ├── protocol/             # 协议核心实现
│   │   ├── reliability/          # 可靠性机制
│   │   └── transport/            # 传输层实现
│   ├── session/                  # 会话管理服务
│   └── statesync/                # 状态同步服务
├── pkg/                          # 可被外部应用程序使用的库代码
│   ├── errors/                   # 错误处理
│   ├── guuid/                    # GUUID实现
│   ├── logger/                   # 日志库
│   └── utils/                    # 工具函数
├── scripts/                      # 构建、安装、分析等脚本
├── tests/                        # 测试文件
│   ├── benchmarks/               # 性能基准测试
│   ├── e2e/                      # 端到端测试
│   ├── integration/              # 集成测试
│   └── unit/                     # 单元测试
├── ARCHITECTURE.md               # 架构设计文档
├── LICENSE                       # 许可证文件
├── PROJECT_STRUCTURE.md          # 项目结构说明（本文件）
└── README.md                     # 项目说明文档
```

## 详细目录说明

### `/cmd` - 应用程序入口
每个子目录包含一个可执行应用程序的main函数：
- `api-gateway/main.go` - API网关服务启动入口
- `session-service/main.go` - 会话管理服务启动入口
- `statesync-service/main.go` - 状态同步服务启动入口

### `/internal` - 私有代码
包含应用程序的私有代码，不希望被其他应用程序或库导入：

#### `/internal/quantum` - Quantum协议核心
- `protocol/` - 协议包头定义、序列化/反序列化
- `bbr/` - BBR拥塞控制算法实现
- `fec/` - Reed-Solomon前向纠错实现
- `reliability/` - SACK、快速重传等可靠性机制
- `transport/` - UDP传输层封装

#### 微服务实现
- `gateway/` - API网关业务逻辑
- `session/` - 会话管理业务逻辑
- `statesync/` - 状态同步业务逻辑

#### 基础设施
- `discovery/` - 基于etcd的服务发现
- `config/` - 动态配置管理
- `metrics/` - Prometheus指标收集

### `/pkg` - 公共库
可以被外部应用程序使用的库代码：
- `guuid/` - 使用 github.com/Lzww0608/GUUID (UUIDv7标准实现)
- `errors/` - 统一错误处理
- `utils/` - 通用工具函数

**注**: 日志直接使用 go.uber.org/zap，无需自定义封装

### `/api` - API定义
- `proto/` - gRPC服务的Protocol Buffers定义
- `openapi/` - REST API的OpenAPI规范

### `/deployments` - 部署配置
- `docker/` - Dockerfile和docker-compose文件
- `kubernetes/base/` - 基础Kubernetes资源定义
- `kubernetes/overlays/` - 不同环境的配置覆盖
- `helm/` - Helm Charts用于包管理

### `/configs` - 配置文件
- `prometheus/` - Prometheus监控配置和告警规则
- `grafana/` - Grafana仪表盘JSON配置
- `etcd/` - etcd集群配置

### `/tests` - 测试
- `unit/` - 单元测试，与源代码目录结构对应
- `integration/` - 集成测试，测试组件间交互
- `e2e/` - 端到端测试，测试完整用户场景
- `benchmarks/` - 性能基准测试

### `/scripts` - 脚本
构建、部署、开发辅助脚本：
- 构建脚本
- 部署脚本
- 代码生成脚本
- 开发环境设置脚本

### `/docs` - 文档
项目相关文档：
- API文档
- 部署指南
- 开发指南
- 性能调优指南

### `/examples` - 示例
- 客户端使用示例
- 配置示例
- 集成示例

## 文件命名约定

### Go文件
- 使用小写字母和下划线：`quantum_protocol.go`
- 测试文件以`_test.go`结尾：`quantum_protocol_test.go`
- 基准测试文件：`quantum_protocol_bench_test.go`

### 配置文件
- YAML文件使用小写和连字符：`api-gateway-config.yaml`
- JSON文件使用小写和连字符：`grafana-dashboard.json`

### Docker和Kubernetes
- Dockerfile：`Dockerfile`
- Kubernetes清单：`deployment.yaml`, `service.yaml`
- Kustomization文件：`kustomization.yaml`

## 包导入约定

### 内部包导入顺序
```go
import (
    // 标准库
    "context"
    "fmt"
    "net"
    
    // 第三方库
    "github.com/prometheus/client_golang/prometheus"
    "go.etcd.io/etcd/clientv3"
    
    // 项目内部包
    "github.com/aetherflow/pkg/guuid"
    "github.com/aetherflow/internal/quantum/protocol"
)
```

### 包别名
```go
import (
    quantumpb "github.com/aetherflow/api/proto/quantum"
    sessionpb "github.com/aetherflow/api/proto/session"
)
```

## 构建标签

使用构建标签来区分不同的构建目标：
```go
//go:build integration
// +build integration

package tests
```
