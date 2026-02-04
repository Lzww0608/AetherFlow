package breaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewCircuitBreaker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultConfig()

	cb := NewCircuitBreaker("test", config, logger)

	if cb == nil {
		t.Fatal("Expected circuit breaker to be created")
	}

	if cb.State() != StateClosed {
		t.Errorf("Expected initial state to be CLOSED, got %s", cb.State())
	}
}

func TestCircuitBreaker_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := Config{
		MaxRequests: 3,
		Interval:    time.Second,
		Timeout:     time.Second,
	}

	cb := NewCircuitBreaker("test-success", config, logger)

	// 执行成功的函数
	for i := 0; i < 5; i++ {
		err := cb.Execute(func() error {
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}

	// 状态应该仍然是CLOSED
	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED, got %s", cb.State())
	}

	counts := cb.Counts()
	if counts.TotalSuccesses != 5 {
		t.Errorf("Expected 5 successes, got %d", counts.TotalSuccesses)
	}
}

func TestCircuitBreaker_Failure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := Config{
		MaxRequests: 3,
		Interval:    time.Second,
		Timeout:     time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cb := NewCircuitBreaker("test-failure", config, logger)

	testErr := errors.New("test error")

	// 执行失败的函数（连续3次）
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected test error, got %v", err)
		}
	}

	// 第3次失败后应该触发熔断
	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN after 3 failures, got %s", cb.State())
	}

	// 熔断后的请求应该直接失败
	err := cb.Execute(func() error {
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := Config{
		MaxRequests: 1, // 改为1，简化测试
		Interval:    100 * time.Millisecond,
		Timeout:     200 * time.Millisecond, // 短超时便于测试
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreaker("test-halfopen", config, logger)

	testErr := errors.New("test error")

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %s", cb.State())
	}

	// 等待超时进入半开状态
	time.Sleep(300 * time.Millisecond)

	// 现在应该是半开状态
	if cb.State() != StateHalfOpen {
		t.Errorf("Expected state to be HALF_OPEN, got %s", cb.State())
	}

	// 半开状态下1次成功应该恢复到CLOSED
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error in half-open state, got %v", err)
	}

	// 现在应该恢复到CLOSED
	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED after successful request, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := Config{
		MaxRequests: 2,
		Interval:    100 * time.Millisecond,
		Timeout:     200 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreaker("test-halfopen-fail", config, logger)

	testErr := errors.New("test error")

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// 等待进入半开状态
	time.Sleep(300 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Errorf("Expected state to be HALF_OPEN, got %s", cb.State())
	}

	// 半开状态下失败应该立即回到OPEN
	cb.Execute(func() error {
		return testErr
	})

	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN after failure in half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_TooManyRequests(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := Config{
		MaxRequests: 2,
		Interval:    100 * time.Millisecond,
		Timeout:     200 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreaker("test-toomany", config, logger)

	testErr := errors.New("test error")

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// 等待进入半开状态
	time.Sleep(300 * time.Millisecond)

	// 半开状态下，超过MaxRequests应该返回ErrTooManyRequests
	// 先发送MaxRequests个请求
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	// 第3个请求应该被拒绝
	err := cb.Execute(func() error {
		return nil
	})

	if !errors.Is(err, ErrTooManyRequests) {
		t.Errorf("Expected ErrTooManyRequests, got %v", err)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultConfig()

	cb := NewCircuitBreaker("test-reset", config, logger)

	testErr := errors.New("test error")

	// 触发熔断
	for i := 0; i < 5; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %s", cb.State())
	}

	// 重置
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED after reset, got %s", cb.State())
	}

	counts := cb.Counts()
	if counts.Requests != 0 {
		t.Errorf("Expected 0 requests after reset, got %d", counts.Requests)
	}
}

func TestCircuitBreaker_ExecuteContext(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultConfig()

	cb := NewCircuitBreaker("test-context", config, logger)

	ctx := context.Background()

	// 测试成功
	err := cb.ExecuteContext(ctx, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试失败
	testErr := errors.New("test error")
	err = cb.ExecuteContext(ctx, func(ctx context.Context) error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected test error, got %v", err)
	}
}
