package grpcclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ConnectionPool gRPC连接池
type ConnectionPool struct {
	target      string
	maxIdle     int
	maxActive   int
	idleTimeout time.Duration
	dialOptions []grpc.DialOption
	
	mu          sync.Mutex
	connections []*grpc.ClientConn
	active      int
	logger      *zap.Logger
	
	// 动态地址支持
	dynamicAddresses []string
	addressIndex     int
}

// NewConnectionPool 创建连接池
func NewConnectionPool(target string, maxIdle, maxActive int, idleTimeout time.Duration, dialOptions []grpc.DialOption, logger *zap.Logger) *ConnectionPool {
	return &ConnectionPool{
		target:      target,
		maxIdle:     maxIdle,
		maxActive:   maxActive,
		idleTimeout: idleTimeout,
		dialOptions: dialOptions,
		connections: make([]*grpc.ClientConn, 0, maxIdle),
		logger:      logger,
	}
}

// Get 获取连接
func (p *ConnectionPool) Get(ctx context.Context) (*grpc.ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 尝试从池中获取空闲连接
	if len(p.connections) > 0 {
		conn := p.connections[len(p.connections)-1]
		p.connections = p.connections[:len(p.connections)-1]
		p.active++
		return conn, nil
	}

	// 检查是否达到最大活跃连接数
	if p.active >= p.maxActive {
		return nil, fmt.Errorf("connection pool exhausted (active: %d, max: %d)", p.active, p.maxActive)
	}

	// 创建新连接
	conn, err := p.createConnection(ctx)
	if err != nil {
		return nil, err
	}

	p.active++
	return conn, nil
}

// Put 归还连接
func (p *ConnectionPool) Put(conn *grpc.ClientConn) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.active--

	// 检查连接状态
	state := conn.GetState()
	if state.String() != "READY" && state.String() != "IDLE" {
		conn.Close()
		return
	}

	// 如果池已满，关闭连接
	if len(p.connections) >= p.maxIdle {
		conn.Close()
		return
	}

	// 放回池中
	p.connections = append(p.connections, conn)
}

// createConnection 创建新的gRPC连接
func (p *ConnectionPool) createConnection(ctx context.Context) (*grpc.ClientConn, error) {
	opts := p.dialOptions
	if len(opts) == 0 {
		// 默认使用TCP
		opts = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                10 * time.Second,
				Timeout:             3 * time.Second,
				PermitWithoutStream: true,
			}),
		}
	}

	// 使用GetTarget获取目标地址（支持动态地址轮询）
	target := p.GetTarget()
	
	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", target, err)
	}

	return conn, nil
}

// UpdateAddresses 更新动态地址列表
func (p *ConnectionPool) UpdateAddresses(addresses []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(addresses) == 0 {
		p.logger.Warn("No addresses provided for update")
		return
	}

	p.dynamicAddresses = make([]string, len(addresses))
	copy(p.dynamicAddresses, addresses)
	p.addressIndex = 0

	p.logger.Info("Addresses updated",
		zap.Strings("addresses", addresses),
	)

	// 关闭现有连接，下次Get时会使用新地址
	p.closeAllConnections()
}

// GetTarget 获取当前目标地址（支持轮询）
func (p *ConnectionPool) GetTarget() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果有动态地址，使用轮询
	if len(p.dynamicAddresses) > 0 {
		target := p.dynamicAddresses[p.addressIndex]
		p.addressIndex = (p.addressIndex + 1) % len(p.dynamicAddresses)
		return target
	}

	// 否则使用静态target
	return p.target
}

// closeAllConnections 关闭所有连接（内部调用，需持有锁）
func (p *ConnectionPool) closeAllConnections() {
	for _, conn := range p.connections {
		if err := conn.Close(); err != nil {
			p.logger.Warn("Failed to close connection", zap.Error(err))
		}
	}
	p.connections = p.connections[:0]
	p.active = 0

	p.logger.Info("All connections closed")
}

// Close 关闭连接池
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for _, conn := range p.connections {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	p.connections = nil
	p.active = 0

	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d connections", len(errs))
	}

	return nil
}

// Stats 获取连接池统计信息
func (p *ConnectionPool) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	return map[string]interface{}{
		"target":      p.target,
		"idle":        len(p.connections),
		"active":      p.active,
		"max_idle":    p.maxIdle,
		"max_active":  p.maxActive,
	}
}

// Manager gRPC客户端管理器
type Manager struct {
	pools  map[string]*ConnectionPool
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewManager 创建gRPC客户端管理器
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		pools:  make(map[string]*ConnectionPool),
		logger: logger,
	}
}

// RegisterPool 注册连接池
func (m *Manager) RegisterPool(name, target string, maxIdle, maxActive int, idleTimeout time.Duration, dialOptions ...grpc.DialOption) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool := NewConnectionPool(target, maxIdle, maxActive, idleTimeout, dialOptions, m.logger)
	m.pools[name] = pool

	transportType := "TCP"
	if len(dialOptions) > 0 {
		transportType = "Quantum"
	}

	m.logger.Info("Registered gRPC connection pool",
		zap.String("name", name),
		zap.String("target", target),
		zap.String("transport", transportType),
		zap.Int("max_idle", maxIdle),
		zap.Int("max_active", maxActive),
	)
}

// GetPool 获取连接池
func (m *Manager) GetPool(name string) *ConnectionPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.pools[name]
}

// GetConnection 获取连接
func (m *Manager) GetConnection(ctx context.Context, name string) (*grpc.ClientConn, error) {
	m.mu.RLock()
	pool, ok := m.pools[name]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("pool not found: %s", name)
	}

	return pool.Get(ctx)
}

// PutConnection 归还连接
func (m *Manager) PutConnection(name string, conn *grpc.ClientConn) {
	m.mu.RLock()
	pool, ok := m.pools[name]
	m.mu.RUnlock()

	if !ok {
		if conn != nil {
			conn.Close()
		}
		return
	}

	pool.Put(conn)
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, pool := range m.pools {
		if err := pool.Close(); err != nil {
			m.logger.Error("Failed to close pool",
				zap.String("name", name),
				zap.Error(err),
			)
			errs = append(errs, err)
		}
	}

	m.pools = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d pools", len(errs))
	}

	return nil
}

// Stats 获取所有连接池统计信息
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, pool := range m.pools {
		stats[name] = pool.Stats()
	}

	return stats
}
