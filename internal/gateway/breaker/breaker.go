package breaker

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	// ErrCircuitOpen 熔断器打开错误
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrTooManyRequests 请求过多错误
	ErrTooManyRequests = errors.New("too many requests")
)

// State 熔断器状态
type State int

const (
	// StateClosed 关闭状态（正常）
	StateClosed State = iota
	// StateHalfOpen 半开状态（探测）
	StateHalfOpen
	// StateOpen 打开状态（熔断）
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateHalfOpen:
		return "HALF_OPEN"
	case StateOpen:
		return "OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config 熔断器配置
type Config struct {
	// MaxRequests 半开状态下最大请求数
	MaxRequests uint32
	// Interval 统计间隔
	Interval time.Duration
	// Timeout 打开状态超时时间
	Timeout time.Duration
	// ReadyToTrip 判断是否需要熔断的函数
	ReadyToTrip func(counts Counts) bool
	// OnStateChange 状态变更回调
	OnStateChange func(name string, from State, to State)
}

// Counts 请求计数
type Counts struct {
	Requests             uint32  // 总请求数
	TotalSuccesses       uint32  // 总成功数
	TotalFailures        uint32  // 总失败数
	ConsecutiveSuccesses uint32  // 连续成功数
	ConsecutiveFailures  uint32  // 连续失败数
}

// Reset 重置计数
func (c *Counts) Reset() {
	c.Requests = 0
	c.TotalSuccesses = 0
	c.TotalFailures = 0
	c.ConsecutiveSuccesses = 0
	c.ConsecutiveFailures = 0
}

// OnSuccess 成功回调
func (c *Counts) OnSuccess() {
	c.Requests++
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
}

// OnFailure 失败回调
func (c *Counts) OnFailure() {
	c.Requests++
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
}

// ErrorRate 错误率
func (c *Counts) ErrorRate() float64 {
	if c.Requests == 0 {
		return 0.0
	}
	return float64(c.TotalFailures) / float64(c.Requests)
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	name   string
	config Config
	logger *zap.Logger

	mu          sync.RWMutex
	state       State
	generation  uint64
	counts      Counts
	expiry      time.Time
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(name string, config Config, logger *zap.Logger) *CircuitBreaker {
	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}
	if config.Interval == 0 {
		config.Interval = 10 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.ReadyToTrip == nil {
		config.ReadyToTrip = defaultReadyToTrip
	}

	cb := &CircuitBreaker{
		name:   name,
		config: config,
		logger: logger,
		state:  StateClosed,
		expiry: time.Now().Add(config.Interval),
	}

	return cb
}

// defaultReadyToTrip 默认熔断判断：失败率超过50%或连续失败5次
func defaultReadyToTrip(counts Counts) bool {
	return counts.Requests >= 5 && (counts.ErrorRate() >= 0.5 || counts.ConsecutiveFailures >= 5)
}

// Execute 执行函数（带熔断保护）
func (cb *CircuitBreaker) Execute(fn func() error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	err = fn()
	cb.afterRequest(generation, err == nil)
	return err
}

// ExecuteContext 执行函数（带Context）
func (cb *CircuitBreaker) ExecuteContext(ctx context.Context, fn func(context.Context) error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	err = fn(ctx)
	cb.afterRequest(generation, err == nil)
	return err
}

// beforeRequest 请求前检查
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrCircuitOpen
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.config.MaxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++
	return generation, nil
}

// afterRequest 请求后处理
func (cb *CircuitBreaker) afterRequest(generation uint64, success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, currentGeneration := cb.currentState(now)

	// 如果generation不匹配，说明状态已改变，不更新计数
	if generation != currentGeneration {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess 成功处理
func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.OnSuccess()
	case StateHalfOpen:
		cb.counts.OnSuccess()
		if cb.counts.ConsecutiveSuccesses >= cb.config.MaxRequests {
			cb.setState(StateClosed, now)
		}
	}
}

// onFailure 失败处理
func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.OnFailure()
		if cb.config.ReadyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

// currentState 获取当前状态
func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

// setState 设置状态
func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.name, prev, state)
	}

	cb.logger.Info("Circuit breaker state changed",
		zap.String("name", cb.name),
		zap.String("from", prev.String()),
		zap.String("to", state.String()),
		zap.Float64("error_rate", cb.counts.ErrorRate()),
		zap.Uint32("requests", cb.counts.Requests),
		zap.Uint32("failures", cb.counts.TotalFailures),
	)
}

// toNewGeneration 进入新周期
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts.Reset()

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.config.Interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.config.Interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.config.Timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}

// State 获取当前状态
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts 获取当前计数
func (cb *CircuitBreaker) Counts() Counts {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.counts
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.toNewGeneration(time.Now())
	cb.state = StateClosed

	cb.logger.Info("Circuit breaker reset", zap.String("name", cb.name))
}
