package websocket

import (
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage(MessageTypePing, map[string]string{"test": "data"})
	
	if msg.ID == "" {
		t.Error("Message ID should not be empty")
	}
	
	if msg.Type != MessageTypePing {
		t.Errorf("Expected type %s, got %s", MessageTypePing, msg.Type)
	}
	
	if msg.Data == nil {
		t.Error("Message data should not be nil")
	}
	
	if msg.Timestamp.IsZero() {
		t.Error("Message timestamp should not be zero")
	}
}

func TestNewErrorMessage(t *testing.T) {
	errMsg := "test error"
	msg := NewErrorMessage(errMsg)
	
	if msg.Type != MessageTypeError {
		t.Errorf("Expected type %s, got %s", MessageTypeError, msg.Type)
	}
	
	if msg.Error != errMsg {
		t.Errorf("Expected error %s, got %s", errMsg, msg.Error)
	}
}

func TestMessageJSON(t *testing.T) {
	original := &Message{
		ID:        "test-id",
		Type:      MessageTypePing,
		Timestamp: time.Now(),
		Data:      map[string]string{"key": "value"},
	}
	
	// 转换为JSON
	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}
	
	// 从JSON解析
	parsed, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to parse from JSON: %v", err)
	}
	
	if parsed.ID != original.ID {
		t.Errorf("Expected ID %s, got %s", original.ID, parsed.ID)
	}
	
	if parsed.Type != original.Type {
		t.Errorf("Expected type %s, got %s", original.Type, parsed.Type)
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	invalidJSON := []byte("{invalid json")
	
	_, err := FromJSON(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestMessageTypes(t *testing.T) {
	types := []MessageType{
		MessageTypePing,
		MessageTypePong,
		MessageTypeAuth,
		MessageTypeAuthResult,
		MessageTypeError,
		MessageTypeSubscribe,
		MessageTypeUnsubscribe,
		MessageTypePublish,
		MessageTypeNotify,
	}
	
	for _, msgType := range types {
		msg := NewMessage(msgType, nil)
		if msg.Type != msgType {
			t.Errorf("Expected type %s, got %s", msgType, msg.Type)
		}
	}
}
