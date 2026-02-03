package grpcclient

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/aetherflow/aetherflow/internal/quantum"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// QuantumDialer 使用Quantum协议的gRPC拨号器
type QuantumDialer struct {
	logger *zap.Logger
}

// NewQuantumDialer 创建Quantum拨号器
func NewQuantumDialer(logger *zap.Logger) *QuantumDialer {
	return &QuantumDialer{
		logger: logger,
	}
}

// Dial 使用Quantum协议拨号
func (d *QuantumDialer) Dial(ctx context.Context, target string) (net.Conn, error) {
	d.logger.Info("Dialing with Quantum protocol", zap.String("target", target))

	// 解析目标地址
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target address: %w", err)
	}

	// 使用Quantum协议建立连接
	config := quantum.DefaultConfig()
	config.FECEnabled = true  // 启用FEC前向纠错
	// BBR拥塞控制默认已启用

	conn, err := quantum.Dial("udp", net.JoinHostPort(host, port), config)
	if err != nil {
		return nil, fmt.Errorf("quantum dial failed: %w", err)
	}

	// 包装成quantumConn实现net.Conn接口
	return &quantumConn{
		conn:   conn,
		logger: d.logger,
	}, nil
}

// DialOption 创建gRPC DialOption
func (d *QuantumDialer) DialOption() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return d.Dial(ctx, addr)
	})
}

// quantumConn 包装Quantum连接以实现net.Conn接口
type quantumConn struct {
	conn         *quantum.Connection
	logger       *zap.Logger
	readDeadline  time.Time
	writeDeadline time.Time
}

// Read 实现net.Conn的Read方法
func (c *quantumConn) Read(b []byte) (int, error) {
	// 如果设置了读超时
	if !c.readDeadline.IsZero() {
		timeout := time.Until(c.readDeadline)
		if timeout <= 0 {
			return 0, fmt.Errorf("read deadline exceeded")
		}
		
		data, err := c.conn.ReceiveWithTimeout(timeout)
		if err != nil {
			return 0, err
		}
		
		n := copy(b, data)
		return n, nil
	}
	
	// 没有超时，使用阻塞接收
	data, err := c.conn.Receive()
	if err != nil {
		return 0, err
	}
	
	n := copy(b, data)
	return n, nil
}

// Write 实现net.Conn的Write方法
func (c *quantumConn) Write(b []byte) (int, error) {
	// Quantum的Send方法发送整个数据包
	if err := c.conn.Send(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Close 实现net.Conn的Close方法
func (c *quantumConn) Close() error {
	return c.conn.Close()
}

// LocalAddr 实现net.Conn的LocalAddr方法
func (c *quantumConn) LocalAddr() net.Addr {
	addr, _ := net.ResolveUDPAddr("udp", c.conn.LocalAddr())
	return addr
}

// RemoteAddr 实现net.Conn的RemoteAddr方法
func (c *quantumConn) RemoteAddr() net.Addr {
	addr, _ := net.ResolveUDPAddr("udp", c.conn.RemoteAddr())
	return addr
}

// SetDeadline 实现net.Conn的SetDeadline方法
func (c *quantumConn) SetDeadline(t time.Time) error {
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

// SetReadDeadline 实现net.Conn的SetReadDeadline方法
func (c *quantumConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

// SetWriteDeadline 实现net.Conn的SetWriteDeadline方法
func (c *quantumConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

// GetDialOptions 根据传输协议获取DialOption
func GetDialOptions(transport string, quantumDialer *QuantumDialer) []grpc.DialOption {
	if transport == "quantum" {
		return []grpc.DialOption{
			quantumDialer.DialOption(),
		}
	}
	// TCP使用默认配置（insecure + keepalive）
	return nil
}
