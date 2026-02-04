package discovery

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// ServiceResolver 服务解析器
type ServiceResolver struct {
	etcdClient *EtcdClient
	logger     *zap.Logger
	
	mu        sync.RWMutex
	services  map[string][]string // serviceName -> []address
	listeners map[string][]UpdateListener
}

// UpdateListener 服务地址更新监听器
type UpdateListener func(serviceName string, addresses []string)

// NewServiceResolver 创建服务解析器
func NewServiceResolver(etcdClient *EtcdClient, logger *zap.Logger) *ServiceResolver {
	return &ServiceResolver{
		etcdClient: etcdClient,
		logger:     logger,
		services:   make(map[string][]string),
		listeners:  make(map[string][]UpdateListener),
	}
}

// Discover 发现服务
func (r *ServiceResolver) Discover(serviceName string) error {
	prefix := fmt.Sprintf("/services/%s/", serviceName)

	// 监听服务变化
	err := r.etcdClient.Watch(prefix, func(eventType string, key, value string) {
		r.handleServiceChange(serviceName, eventType, key, value)
	})

	if err != nil {
		return fmt.Errorf("failed to watch service: %w", err)
	}

	r.logger.Info("Started discovering service",
		zap.String("service", serviceName),
		zap.String("prefix", prefix),
	)

	return nil
}

// handleServiceChange 处理服务变化
func (r *ServiceResolver) handleServiceChange(serviceName, eventType, key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	addresses, exists := r.services[serviceName]
	if !exists {
		addresses = []string{}
	}

	switch eventType {
	case "PUT":
		// 添加或更新服务地址
		found := false
		for i, addr := range addresses {
			if addr == value {
				found = true
				break
			}
			// 如果是同一个key但地址变了，更新它
			if fmt.Sprintf("/services/%s/%s", serviceName, addr) == key {
				addresses[i] = value
				found = true
				break
			}
		}
		if !found {
			addresses = append(addresses, value)
		}

	case "DELETE":
		// 删除服务地址
		newAddresses := []string{}
		for _, addr := range addresses {
			if addr != value && fmt.Sprintf("/services/%s/%s", serviceName, addr) != key {
				newAddresses = append(newAddresses, addr)
			}
		}
		addresses = newAddresses
	}

	r.services[serviceName] = addresses

	r.logger.Info("Service addresses updated",
		zap.String("service", serviceName),
		zap.String("event", eventType),
		zap.Strings("addresses", addresses),
	)

	// 通知所有监听器
	if listeners, ok := r.listeners[serviceName]; ok {
		for _, listener := range listeners {
			listener(serviceName, addresses)
		}
	}
}

// GetAddresses 获取服务地址列表
func (r *ServiceResolver) GetAddresses(serviceName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addresses, exists := r.services[serviceName]
	if !exists {
		return []string{}
	}

	// 返回副本，避免外部修改
	result := make([]string, len(addresses))
	copy(result, addresses)
	return result
}

// AddUpdateListener 添加服务地址更新监听器
func (r *ServiceResolver) AddUpdateListener(serviceName string, listener UpdateListener) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.listeners[serviceName]; !exists {
		r.listeners[serviceName] = []UpdateListener{}
	}

	r.listeners[serviceName] = append(r.listeners[serviceName], listener)

	r.logger.Info("Added update listener",
		zap.String("service", serviceName),
	)

	// 如果服务已有地址，立即通知
	if addresses, exists := r.services[serviceName]; exists && len(addresses) > 0 {
		listener(serviceName, addresses)
	}
}

// RemoveUpdateListener 移除服务地址更新监听器
func (r *ServiceResolver) RemoveUpdateListener(serviceName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.listeners, serviceName)

	r.logger.Info("Removed update listeners",
		zap.String("service", serviceName),
	)
}

// GetServiceCount 获取已发现的服务数量
func (r *ServiceResolver) GetServiceCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.services)
}

// GetAllServices 获取所有已发现的服务
func (r *ServiceResolver) GetAllServices() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]string)
	for name, addresses := range r.services {
		addressesCopy := make([]string, len(addresses))
		copy(addressesCopy, addresses)
		result[name] = addressesCopy
	}

	return result
}
