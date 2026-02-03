package grpcclient

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewQuantumDialer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	dialer := NewQuantumDialer(logger)

	if dialer == nil {
		t.Fatal("Expected quantum dialer to be created")
	}

	if dialer.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestQuantumDialer_DialOption(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	dialer := NewQuantumDialer(logger)

	dialOption := dialer.DialOption()
	if dialOption == nil {
		t.Fatal("Expected dial option to be created")
	}
}

func TestGetDialOptions_TCP(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	quantumDialer := NewQuantumDialer(logger)

	opts := GetDialOptions("tcp", quantumDialer)
	if opts != nil {
		t.Error("Expected nil options for TCP transport")
	}
}

func TestGetDialOptions_Quantum(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	quantumDialer := NewQuantumDialer(logger)

	opts := GetDialOptions("quantum", quantumDialer)
	if opts == nil {
		t.Fatal("Expected dial options for Quantum transport")
	}

	if len(opts) != 1 {
		t.Errorf("Expected 1 dial option, got %d", len(opts))
	}
}

func TestGetDialOptions_Default(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	quantumDialer := NewQuantumDialer(logger)

	// 测试未知的transport类型，应该返回nil（使用TCP）
	opts := GetDialOptions("unknown", quantumDialer)
	if opts != nil {
		t.Error("Expected nil options for unknown transport")
	}
}
