package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdClient Etcd服务发现客户端
type EtcdClient struct {
	client       *clientv3.Client
	logger       *zap.Logger
	leaseID      clientv3.LeaseID
	keepAliveCh  <-chan *clientv3.LeaseKeepAliveResponse
	mu           sync.RWMutex
	serviceKey   string
	serviceValue string
	closed       bool
	ctx          context.Context
	cancel       context.CancelFunc
}

// Config Etcd配置
type Config struct {
	Endpoints   []string      // Etcd endpoints
	DialTimeout time.Duration // 连接超时
	Username    string        // 用户名（可选）
	Password    string        // 密码（可选）
}

// NewEtcdClient 创建Etcd客户端
func NewEtcdClient(config *Config, logger *zap.Logger) (*EtcdClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	clientConfig := clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: config.DialTimeout,
	}

	if config.Username != "" {
		clientConfig.Username = config.Username
		clientConfig.Password = config.Password
	}

	client, err := clientv3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	etcdClient := &EtcdClient{
		client: client,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	logger.Info("Etcd client created successfully",
		zap.Strings("endpoints", config.Endpoints),
	)

	return etcdClient, nil
}

// Register 注册服务
func (c *EtcdClient) Register(serviceKey, serviceValue string, ttl int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// 创建租约
	lease, err := c.client.Grant(c.ctx, ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	c.leaseID = lease.ID
	c.serviceKey = serviceKey
	c.serviceValue = serviceValue

	// 注册服务
	_, err = c.client.Put(c.ctx, serviceKey, serviceValue, clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 启动心跳保活
	keepAliveCh, err := c.client.KeepAlive(c.ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("failed to keep alive: %w", err)
	}

	c.keepAliveCh = keepAliveCh

	// 启动心跳监听
	go c.watchKeepAlive()

	c.logger.Info("Service registered successfully",
		zap.String("key", serviceKey),
		zap.String("value", serviceValue),
		zap.Int64("ttl", ttl),
		zap.Int64("lease_id", int64(lease.ID)),
	)

	return nil
}

// watchKeepAlive 监听心跳保活响应
func (c *EtcdClient) watchKeepAlive() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case resp, ok := <-c.keepAliveCh:
			if !ok {
				c.logger.Warn("Keep alive channel closed, attempting to re-register")
				c.mu.Lock()
				if !c.closed && c.serviceKey != "" {
					// 尝试重新注册
					if err := c.reRegister(); err != nil {
						c.logger.Error("Failed to re-register service", zap.Error(err))
					}
				}
				c.mu.Unlock()
				return
			}
			if resp != nil {
				c.logger.Debug("Keep alive response received",
					zap.Int64("ttl", resp.TTL),
				)
			}
		}
	}
}

// reRegister 重新注册服务（内部调用，需要持有锁）
func (c *EtcdClient) reRegister() error {
	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// 创建新租约
	lease, err := c.client.Grant(c.ctx, 10) // 默认10秒TTL
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	c.leaseID = lease.ID

	// 重新注册
	_, err = c.client.Put(c.ctx, c.serviceKey, c.serviceValue, clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 重新启动心跳
	keepAliveCh, err := c.client.KeepAlive(c.ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("failed to keep alive: %w", err)
	}

	c.keepAliveCh = keepAliveCh
	go c.watchKeepAlive()

	c.logger.Info("Service re-registered successfully",
		zap.String("key", c.serviceKey),
		zap.Int64("lease_id", int64(lease.ID)),
	)

	return nil
}

// Unregister 注销服务
func (c *EtcdClient) Unregister() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	// 删除服务key
	if c.serviceKey != "" {
		_, err := c.client.Delete(c.ctx, c.serviceKey)
		if err != nil {
			c.logger.Warn("Failed to delete service key", zap.Error(err))
		}
	}

	// 撤销租约
	if c.leaseID != 0 {
		_, err := c.client.Revoke(c.ctx, c.leaseID)
		if err != nil {
			c.logger.Warn("Failed to revoke lease", zap.Error(err))
		}
	}

	c.logger.Info("Service unregistered successfully",
		zap.String("key", c.serviceKey),
	)

	c.serviceKey = ""
	c.serviceValue = ""

	return nil
}

// Watch 监听服务变化
func (c *EtcdClient) Watch(prefix string, handler func(eventType string, key, value string)) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	// 先获取当前所有服务
	resp, err := c.client.Get(c.ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to get services: %w", err)
	}

	// 通知现有服务
	for _, kv := range resp.Kvs {
		handler("PUT", string(kv.Key), string(kv.Value))
	}

	// 监听后续变化
	watchCh := c.client.Watch(c.ctx, prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

	go func() {
		c.logger.Info("Started watching services", zap.String("prefix", prefix))

		for {
			select {
			case <-c.ctx.Done():
				return
			case watchResp, ok := <-watchCh:
				if !ok {
					c.logger.Warn("Watch channel closed")
					return
				}

				if watchResp.Err() != nil {
					c.logger.Error("Watch error", zap.Error(watchResp.Err()))
					continue
				}

				for _, event := range watchResp.Events {
					key := string(event.Kv.Key)
					value := string(event.Kv.Value)

					switch event.Type {
					case clientv3.EventTypePut:
						handler("PUT", key, value)
						c.logger.Info("Service added/updated",
							zap.String("key", key),
							zap.String("value", value),
						)
					case clientv3.EventTypeDelete:
						handler("DELETE", key, "")
						c.logger.Info("Service deleted",
							zap.String("key", key),
						)
					}
				}
			}
		}
	}()

	return nil
}

// Get 获取指定key的值
func (c *EtcdClient) Get(key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return "", fmt.Errorf("client is closed")
	}

	resp, err := c.client.Get(c.ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("key not found")
	}

	return string(resp.Kvs[0].Value), nil
}

// GetWithPrefix 获取指定前缀的所有键值对
func (c *EtcdClient) GetWithPrefix(prefix string) (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	resp, err := c.client.Get(c.ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}

	return result, nil
}

// Close 关闭客户端
func (c *EtcdClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// 注销服务
	if c.serviceKey != "" {
		_, _ = c.client.Delete(context.Background(), c.serviceKey)
	}

	// 撤销租约
	if c.leaseID != 0 {
		_, _ = c.client.Revoke(context.Background(), c.leaseID)
	}

	// 取消context
	c.cancel()

	// 关闭客户端
	err := c.client.Close()

	c.logger.Info("Etcd client closed")

	return err
}
