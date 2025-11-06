# Quantum Protocol Examples

这个目录包含Quantum协议的使用示例。

## 简单客户端/服务器示例

### 运行服务器

```bash
go run simple_server.go
```

### 运行客户端

```bash
go run simple_client.go
```

## 示例说明

- **simple_server.go**: 一个简单的回显服务器,接收消息并返回响应
- **simple_client.go**: 一个客户端,向服务器发送消息并接收响应

## 特性演示

这些示例演示了以下特性:

1. **连接建立**: 三次握手建立连接
2. **数据传输**: 可靠的数据发送和接收
3. **FEC纠错**: 自动的前向纠错
4. **BBR拥塞控制**: 自适应带宽管理
5. **统计信息**: 详细的连接和性能统计

## 配置选项

可以通过修改`quantum.DefaultConfig()`来调整以下参数:

- `FECEnabled`: 是否启用FEC
- `FECDataShards`: FEC数据分片数
- `FECParityShards`: FEC校验分片数
- `SendWindow`: 发送窗口大小
- `RecvWindow`: 接收窗口大小
- `KeepaliveInterval`: Keepalive间隔
- `IdleTimeout`: 空闲超时

## 注意事项

1. 这是基础示例,实际应用中需要更完善的错误处理
2. 服务器当前只处理单个连接,多连接需要额外的连接管理
3. 建议在本地网络测试以获得最佳性能

## 下一步

查看完整的API文档: `/docs/QUANTUM_IMPLEMENTATION.md`

