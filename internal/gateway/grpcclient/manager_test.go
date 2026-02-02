package grpcclient

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.pools == nil {
		t.Error("Expected pools map to be initialized")
	}
}

func TestManager_RegisterPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)

	manager.RegisterPool("test", "127.0.0.1:9000", 5, 10, 30*time.Second)

	manager.mu.RLock()
	pool, ok := manager.pools["test"]
	manager.mu.RUnlock()

	if !ok {
		t.Fatal("Expected pool to be registered")
	}

	if pool.target != "127.0.0.1:9000" {
		t.Errorf("Expected target 127.0.0.1:9000, got %s", pool.target)
	}

	if pool.maxIdle != 5 {
		t.Errorf("Expected maxIdle 5, got %d", pool.maxIdle)
	}

	if pool.maxActive != 10 {
		t.Errorf("Expected maxActive 10, got %d", pool.maxActive)
	}
}

func TestManager_Stats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)

	manager.RegisterPool("test1", "127.0.0.1:9000", 5, 10, 30*time.Second)
	manager.RegisterPool("test2", "127.0.0.1:9001", 3, 8, 30*time.Second)

	stats := manager.Stats()

	if len(stats) != 2 {
		t.Errorf("Expected 2 pools in stats, got %d", len(stats))
	}

	test1Stats, ok := stats["test1"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected test1 stats to be present")
	}

	if test1Stats["target"] != "127.0.0.1:9000" {
		t.Errorf("Expected target 127.0.0.1:9000, got %v", test1Stats["target"])
	}
}

func TestManager_Close(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)

	manager.RegisterPool("test", "127.0.0.1:9000", 5, 10, 30*time.Second)

	err := manager.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}

	manager.mu.RLock()
	poolCount := len(manager.pools)
	manager.mu.RUnlock()

	if poolCount != 0 {
		t.Errorf("Expected 0 pools after close, got %d", poolCount)
	}
}

func TestConnectionPool_Stats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pool := NewConnectionPool("127.0.0.1:9000", 5, 10, 30*time.Second, logger)

	stats := pool.Stats()

	if stats["target"] != "127.0.0.1:9000" {
		t.Errorf("Expected target 127.0.0.1:9000, got %v", stats["target"])
	}

	if stats["idle"] != 0 {
		t.Errorf("Expected idle 0, got %v", stats["idle"])
	}

	if stats["active"] != 0 {
		t.Errorf("Expected active 0, got %v", stats["active"])
	}

	if stats["max_idle"] != 5 {
		t.Errorf("Expected max_idle 5, got %v", stats["max_idle"])
	}

	if stats["max_active"] != 10 {
		t.Errorf("Expected max_active 10, got %v", stats["max_active"])
	}
}
