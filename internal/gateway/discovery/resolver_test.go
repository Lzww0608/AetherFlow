package discovery

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewServiceResolver(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// 创建一个nil客户端（测试环境）
	resolver := NewServiceResolver(nil, logger)
	
	if resolver == nil {
		t.Fatal("Expected resolver to be created")
	}
	
	if resolver.services == nil {
		t.Error("Expected services map to be initialized")
	}
	
	if resolver.listeners == nil {
		t.Error("Expected listeners map to be initialized")
	}
}

func TestServiceResolver_GetAddresses(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	resolver := NewServiceResolver(nil, logger)
	
	// 测试空服务
	addresses := resolver.GetAddresses("test-service")
	if len(addresses) != 0 {
		t.Errorf("Expected empty addresses, got %d", len(addresses))
	}
}

func TestServiceResolver_HandleServiceChange(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	resolver := NewServiceResolver(nil, logger)
	
	serviceName := "test-service"
	
	// 测试添加服务
	resolver.handleServiceChange(serviceName, "PUT", "/services/test-service/node1", "192.168.1.1:9000")
	
	addresses := resolver.GetAddresses(serviceName)
	if len(addresses) != 1 {
		t.Errorf("Expected 1 address, got %d", len(addresses))
	}
	
	if addresses[0] != "192.168.1.1:9000" {
		t.Errorf("Expected 192.168.1.1:9000, got %s", addresses[0])
	}
	
	// 测试添加第二个服务实例
	resolver.handleServiceChange(serviceName, "PUT", "/services/test-service/node2", "192.168.1.2:9000")
	
	addresses = resolver.GetAddresses(serviceName)
	if len(addresses) != 2 {
		t.Errorf("Expected 2 addresses, got %d", len(addresses))
	}
	
	// 测试删除服务
	resolver.handleServiceChange(serviceName, "DELETE", "/services/test-service/node1", "192.168.1.1:9000")
	
	addresses = resolver.GetAddresses(serviceName)
	if len(addresses) != 1 {
		t.Errorf("Expected 1 address after delete, got %d", len(addresses))
	}
}

func TestServiceResolver_AddUpdateListener(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	resolver := NewServiceResolver(nil, logger)
	
	serviceName := "test-service"
	listenerCalled := false
	var receivedAddresses []string
	
	// 添加监听器
	resolver.AddUpdateListener(serviceName, func(svcName string, addresses []string) {
		listenerCalled = true
		receivedAddresses = addresses
	})
	
	// 添加服务，应该触发监听器
	resolver.handleServiceChange(serviceName, "PUT", "/services/test-service/node1", "192.168.1.1:9000")
	
	if !listenerCalled {
		t.Error("Expected listener to be called")
	}
	
	if len(receivedAddresses) != 1 {
		t.Errorf("Expected 1 address in listener, got %d", len(receivedAddresses))
	}
}

func TestServiceResolver_GetServiceCount(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	resolver := NewServiceResolver(nil, logger)
	
	if count := resolver.GetServiceCount(); count != 0 {
		t.Errorf("Expected 0 services, got %d", count)
	}
	
	resolver.handleServiceChange("service1", "PUT", "/services/service1/node1", "192.168.1.1:9000")
	resolver.handleServiceChange("service2", "PUT", "/services/service2/node1", "192.168.1.2:9000")
	
	if count := resolver.GetServiceCount(); count != 2 {
		t.Errorf("Expected 2 services, got %d", count)
	}
}

func TestServiceResolver_GetAllServices(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	resolver := NewServiceResolver(nil, logger)
	
	resolver.handleServiceChange("service1", "PUT", "/services/service1/node1", "192.168.1.1:9000")
	resolver.handleServiceChange("service2", "PUT", "/services/service2/node1", "192.168.1.2:9000")
	resolver.handleServiceChange("service2", "PUT", "/services/service2/node2", "192.168.1.3:9000")
	
	all := resolver.GetAllServices()
	
	if len(all) != 2 {
		t.Errorf("Expected 2 services, got %d", len(all))
	}
	
	if len(all["service1"]) != 1 {
		t.Errorf("Expected 1 address for service1, got %d", len(all["service1"]))
	}
	
	if len(all["service2"]) != 2 {
		t.Errorf("Expected 2 addresses for service2, got %d", len(all["service2"]))
	}
}

func TestServiceResolver_RemoveUpdateListener(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	resolver := NewServiceResolver(nil, logger)
	
	serviceName := "test-service"
	
	// 添加监听器
	resolver.AddUpdateListener(serviceName, func(svcName string, addresses []string) {
		// Do nothing
	})
	
	// 检查监听器已添加
	if len(resolver.listeners[serviceName]) != 1 {
		t.Error("Expected listener to be added")
	}
	
	// 移除监听器
	resolver.RemoveUpdateListener(serviceName)
	
	// 检查监听器已移除
	if _, exists := resolver.listeners[serviceName]; exists {
		t.Error("Expected listener to be removed")
	}
}
