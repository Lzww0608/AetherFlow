# AetherFlow 项目架构设计

## 目录
- [系统架构概览](#系统架构概览)
- [核心组件设计](#核心组件设计)
- [Quantum协议栈](#quantum协议栈)
- [微服务架构](#微服务架构)
- [数据流设计](#数据流设计)
- [部署架构](#部署架构)
- [可观测性架构](#可观测性架构)

## 系统架构概览

### 整体架构图
```
┌─────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────┐    ┌──────────────┐ │
│  │   Grafana       │    │   Prometheus    │    │ Alertmanager │ │
│  │   Dashboard     │    │   Monitoring    │    │   Alerts     │ │
│  └─────────────────┘    └─────────────────┘    └──────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    API Gateway                              │ │
│  │              (GoZero + WebSocket)                          │ │
│  │                  Load Balancer                             │ │
│  └─────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────┐    ┌──────────────┐ │
│  │ Session Service │    │StateSync Service│    │ Other Services│ │
│  │   (GoZero)      │    │   (GoZero)      │    │   (GoZero)   │ │
│  │                 │    │                 │    │              │ │
│  │  ┌───────────┐  │    │  ┌───────────┐  │    │              │ │
│  │  │ Quantum   │  │    │  │ Quantum   │  │    │              │ │
│  │  │ Protocol  │  │    │  │ Protocol  │  │    │              │ │
│  │  │ Stack     │  │    │  │ Stack     │  │    │              │ │
│  │  └───────────┘  │    │  └───────────┘  │    │              │ │
│  └─────────────────┘    └─────────────────┘    └──────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    etcd Cluster                             │ │
│  │        (Service Discovery + Configuration)                 │ │
│  │  ┌─────────┐    ┌─────────┐    ┌─────────┐                 │ │
│  │  │ etcd-0  │    │ etcd-1  │    │ etcd-2  │                 │ │
│  │  └─────────┘    └─────────┘    └─────────┘                 │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 技术栈选择
- **编程语言**: Go 1.21+
- **微服务框架**: GoZero
- **协调服务**: etcd v3.5+
- **容器编排**: Kubernetes 1.28+
- **监控**: Prometheus + Grafana + Alertmanager
- **日志**: 结构化JSON日志 + Fluentd/Loki
- **负载均衡**: 客户端负载均衡 (etcd-based)

## 核心组件设计

### 1. Quantum协议栈
```
┌─────────────────────────────────────────┐
│            Application Layer            │
│              (gRPC)                     │
├─────────────────────────────────────────┤
│           Quantum Protocol              │
│  ┌─────────────────────────────────────┐│
│  │        Reliability Layer            ││
│  │  - SACK (Selective ACK)             ││
│  │  - Fast Retransmit                  ││
│  │  - FEC (Forward Error Correction)   ││
│  └─────────────────────────────────────┘│
│  ┌─────────────────────────────────────┐│
│  │      Congestion Control             ││
│  │  - BBR Algorithm                    ││
│  │  - Pacing Rate Control              ││
│  │  - Bandwidth Probing                ││
│  └─────────────────────────────────────┘│
│  ┌─────────────────────────────────────┐│
│  │        Packet Layer                 ││
│  │  - GUUID Management                 ││
│  │  - Sequence Numbering               ││
│  │  - Header Processing                ││
│  └─────────────────────────────────────┘│
├─────────────────────────────────────────┤
│               UDP Layer                 │
└─────────────────────────────────────────┘
```

### 2. 微服务组件架构
```
┌─────────────────────────────────────────────────────────────────┐
│                        API Gateway                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   HTTP/REST     │  │   WebSocket     │  │   Auth/JWT      │ │
│  │   Handler       │  │   Handler       │  │   Middleware    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │              gRPC Client Pool                               │ │
│  │           (Quantum Protocol Based)                         │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Microservices Layer                         │
│                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌──────────────┐ │
│  │ Session Service │    │StateSync Service│    │Other Services│ │
│  │                 │    │                 │    │              │ │
│  │ ┌─────────────┐ │    │ ┌─────────────┐ │    │              │ │
│  │ │gRPC Server  │ │    │ │gRPC Server  │ │    │              │ │
│  │ └─────────────┘ │    │ └─────────────┘ │    │              │ │
│  │ ┌─────────────┐ │    │ ┌─────────────┐ │    │              │ │
│  │ │Service Logic│ │    │ │Service Logic│ │    │              │ │
│  │ └─────────────┘ │    │ └─────────────┘ │    │              │ │
│  │ ┌─────────────┐ │    │ ┌─────────────┐ │    │              │ │
│  │ │etcd Client  │ │    │ │etcd Client  │ │    │              │ │
│  │ └─────────────┘ │    │ └─────────────┘ │    │              │ │
│  └─────────────────┘    └─────────────────┘    └──────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Quantum协议栈

### 协议包头结构
```go
type QuantumHeader struct {
    MagicNumber    uint32    // 4 bytes - 协议标识
    Version        uint8     // 1 byte  - 协议版本
    Flags          uint8     // 1 byte  - 控制标志
    GUUID          [16]byte  // 16 bytes - 全局唯一标识符
    SequenceNumber uint32    // 4 bytes - 序列号
    AckNumber      uint32    // 4 bytes - 确认号
    PayloadLength  uint16    // 2 bytes - 载荷长度
    // SACK blocks follow (variable length)
}
```

### BBR状态机
```
    ┌─────────────┐
    │   STARTUP   │ ──────┐
    └─────────────┘       │ BtlBw found
           │              │
           │ 2*BDP filled │
           ▼              │
    ┌─────────────┐       │
    │    DRAIN    │ ◄─────┘
    └─────────────┘
           │
           │ inflight <= BDP
           ▼
    ┌─────────────┐
    │  PROBE_BW   │ ◄──────┐
    └─────────────┘        │
           │               │
           │ RTprop expired │
           ▼               │
    ┌─────────────┐        │
    │ PROBE_RTT   │ ───────┘
    └─────────────┘
```

### FEC (前向纠错) 架构
```
Original Data Packets: [P1] [P2] [P3] [P4] [P5] [P6] [P7] [P8] [P9] [P10]
                                    │
                                    ▼
Reed-Solomon Encoding (10,3): Generate 3 parity packets
                                    │
                                    ▼
Transmitted: [P1] [P2] [P3] [P4] [P5] [P6] [P7] [P8] [P9] [P10] [R1] [R2] [R3]
                                    │
                                    ▼
Network (with packet loss): [P1] [X] [P3] [X] [P5] [P6] [P7] [P8] [P9] [P10] [R1] [R2] [X]
                                    │
                                    ▼
Receiver: Can recover P2 and P4 using available packets + Reed-Solomon decoding
```

## 微服务架构

### 服务发现机制
```
etcd Key Structure:
/services/
├── api-gateway/
│   ├── instance-1 → "10.244.1.10:8080" (lease: 30s)
│   └── instance-2 → "10.244.1.11:8080" (lease: 30s)
├── session-service/
│   ├── instance-1 → "10.244.2.10:9090" (lease: 30s)
│   ├── instance-2 → "10.244.2.11:9090" (lease: 30s)
│   └── instance-3 → "10.244.2.12:9090" (lease: 30s)
└── statesync-service/
    ├── instance-1 → "10.244.3.10:9091" (lease: 30s)
    └── instance-2 → "10.244.3.11:9091" (lease: 30s)

/config/
├── quantum/
│   ├── fec-ratio → "0.3"
│   ├── bbr-startup-gain → "2.77"
│   └── max-cwnd → "65536"
└── logging/
    └── level → "info"

/locks/
├── state-compaction → (distributed lock)
└── config-update → (distributed lock)
```

### 负载均衡策略
```go
type LoadBalancer interface {
    Select(endpoints []string) string
}

// P2C (Power of Two Choices) 算法实现
type P2CBalancer struct {
    mu    sync.RWMutex
    stats map[string]*EndpointStats
}

type EndpointStats struct {
    ActiveRequests int64
    LastLatency    time.Duration
    ErrorRate      float64
}
```

## 数据流设计

### 实时协作数据流
```
Client A                API Gateway           StateSync Service         Client B
   │                         │                        │                    │
   │ 1. WebSocket Message    │                        │                    │
   │ ────────────────────────▶                        │                    │
   │                         │ 2. gRPC Call           │                    │
   │                         │ (Quantum Protocol)     │                    │
   │                         ├───────────────────────▶│                    │
   │                         │                        │ 3. State Update    │
   │                         │                        │ ────────────────┐  │
   │                         │                        │                 │  │
   │                         │                        │ ◄───────────────┘  │
   │                         │ 4. Broadcast gRPC      │                    │
   │                         │ ◄──────────────────────┤                    │
   │                         │ 5. WebSocket Push      │                    │
   │                         ├───────────────────────────────────────────▶│
   │                         │                        │                    │
```

### Quantum协议数据包流
```
Sender                                                    Receiver
  │                                                          │
  │ 1. Application Data                                      │
  │ ──────────────────────────────────────────────────────▶ │
  │                                                          │
  │ 2. Quantum Packet (Seq=100, GUUID=xxx)                  │
  │ ──────────────────────────────────────────────────────▶ │
  │                                                          │
  │ 3. SACK (Ack=100, SACK_BLOCKS=[])                       │
  │ ◄────────────────────────────────────────────────────── │
  │                                                          │
  │ 4. Next Packet (Seq=101)                                │
  │ ──X────────────────────────────────────────────────────▶ │ (Lost)
  │                                                          │
  │ 5. Packet (Seq=102)                                     │
  │ ──────────────────────────────────────────────────────▶ │
  │                                                          │
  │ 6. SACK (Ack=100, SACK_BLOCKS=[102-102])                │
  │ ◄────────────────────────────────────────────────────── │
  │                                                          │
  │ 7. Fast Retransmit (Seq=101)                            │
  │ ──────────────────────────────────────────────────────▶ │
  │                                                          │
```

## 部署架构

### Kubernetes资源拓扑
```
Namespace: aetherflow-system
├── Deployments:
│   ├── api-gateway (replicas: 3)
│   ├── session-service (replicas: 3, HPA enabled)
│   ├── statesync-service (replicas: 3, HPA enabled)
│   ├── prometheus (replicas: 1)
│   └── grafana (replicas: 1)
├── StatefulSets:
│   └── etcd (replicas: 3)
├── Services:
│   ├── api-gateway-svc (LoadBalancer, ports: 80, 443, 8080)
│   ├── session-service-svc (ClusterIP, port: 9090, protocol: UDP)
│   ├── statesync-service-svc (ClusterIP, port: 9091, protocol: UDP)
│   ├── etcd-svc (ClusterIP, ports: 2379, 2380)
│   ├── prometheus-svc (ClusterIP, port: 9090)
│   └── grafana-svc (LoadBalancer, port: 3000)
├── ConfigMaps:
│   ├── quantum-config
│   ├── prometheus-config
│   └── grafana-dashboards
├── Secrets:
│   ├── etcd-certs
│   ├── jwt-secret
│   └── grafana-admin
└── HorizontalPodAutoscalers:
    ├── session-service-hpa
    └── statesync-service-hpa
```

### 网络策略
```yaml
# 示例网络策略
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: quantum-protocol-policy
spec:
  podSelector:
    matchLabels:
      protocol: quantum
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          role: api-gateway
    - podSelector:
        matchLabels:
          role: microservice
    ports:
    - protocol: UDP
      port: 9090
    - protocol: UDP
      port: 9091
```

## 可观测性架构

### 指标体系
```
Quantum Protocol Metrics:
├── quantum_packets_sent_total (Counter)
├── quantum_packets_received_total (Counter)
├── quantum_packets_lost_total (Counter)
├── quantum_rtt_seconds (Histogram)
├── quantum_cwnd_bytes (Gauge)
├── quantum_inflight_bytes (Gauge)
├── quantum_fec_recoveries_total (Counter)
└── quantum_retransmissions_total (Counter)

Application Metrics:
├── app_active_sessions (Gauge)
├── app_requests_total (Counter)
├── app_request_duration_seconds (Histogram)
├── app_errors_total (Counter)
└── app_concurrent_operations (Gauge)

Infrastructure Metrics:
├── etcd_cluster_health (Gauge)
├── etcd_leader_changes_total (Counter)
├── kubernetes_pod_restarts_total (Counter)
└── kubernetes_node_capacity (Gauge)
```

### 告警规则
```yaml
# 关键告警规则示例
groups:
- name: quantum-protocol
  rules:
  - alert: QuantumHighPacketLoss
    expr: rate(quantum_packets_lost_total[5m]) / rate(quantum_packets_sent_total[5m]) > 0.05
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Quantum protocol packet loss rate is high"
      
  - alert: QuantumHighLatency
    expr: histogram_quantile(0.99, quantum_rtt_seconds) > 0.1
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Quantum protocol P99 latency is too high"

- name: application
  rules:
  - alert: HighErrorRate
    expr: rate(app_errors_total[5m]) / rate(app_requests_total[5m]) > 0.01
    for: 2m
    labels:
      severity: warning
```

### 分布式追踪
```
Request Flow with GUUID Tracing:
┌─────────────────────────────────────────────────────────────────┐
│ Client Request                                                  │
│ GUUID: 550e8400-e29b-41d4-a716-446655440000                   │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│ API Gateway Log                                                 │
│ {"level":"info","guuid":"550e8400-e29b-41d4-a716-446655440000", │
│  "component":"api-gateway","action":"request_received"}         │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│ StateSync Service Log                                           │
│ {"level":"info","guuid":"550e8400-e29b-41d4-a716-446655440000", │
│  "component":"statesync","action":"state_updated"}              │
└─────────────────────────────────────────────────────────────────┘
```

## 性能目标与SLA

### 关键性能指标 (KPI)
- **延迟目标**: P99 < 50ms (端到端)
- **吞吐量目标**: > 10,000 ops/sec per service instance
- **可用性目标**: 99.9% uptime
- **数据包丢失恢复**: < 10ms (通过FEC)
- **服务发现延迟**: < 100ms
- **自动伸缩响应时间**: < 30s

### 容量规划
```
Resource Requirements per Service Instance:
├── CPU: 500m - 2000m (with HPA)
├── Memory: 512Mi - 2Gi (with HPA)
├── Network: 1Gbps burst capability
└── Storage: 10Gi (for logs and temporary data)

etcd Cluster:
├── CPU: 1000m per instance
├── Memory: 2Gi per instance
├── Storage: 50Gi SSD per instance
└── Network: Low latency, high bandwidth
```

这个架构设计提供了一个完整的、可实施的技术蓝图，涵盖了从底层网络协议到顶层云原生部署的所有关键组件和设计决策。
