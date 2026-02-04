package breaker

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 熔断器管理器
type Manager struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	logger   *zap.Logger
}

// NewManager 创建熔断器管理器
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger,
	}
}

// GetOrCreate 获取或创建熔断器
func (m *Manager) GetOrCreate(name string, config Config) *CircuitBreaker {
	m.mu.RLock()
	breaker, exists := m.breakers[name]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	breaker, exists = m.breakers[name]
	if exists {
		return breaker
	}

	// 设置状态变更回调
	config.OnStateChange = func(name string, from State, to State) {
		m.logger.Info("Circuit breaker state changed",
			zap.String("breaker", name),
			zap.String("from", from.String()),
			zap.String("to", to.String()),
		)
	}

	breaker = NewCircuitBreaker(name, config, m.logger)
	m.breakers[name] = breaker

	m.logger.Info("Circuit breaker created",
		zap.String("name", name),
		zap.Duration("interval", config.Interval),
		zap.Duration("timeout", config.Timeout),
		zap.Uint32("max_requests", config.MaxRequests),
	)

	return breaker
}

// Get 获取熔断器
func (m *Manager) Get(name string) *CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.breakers[name]
}

// Reset 重置熔断器
func (m *Manager) Reset(name string) {
	m.mu.RLock()
	breaker, exists := m.breakers[name]
	m.mu.RUnlock()

	if exists {
		breaker.Reset()
	}
}

// ResetAll 重置所有熔断器
func (m *Manager) ResetAll() {
	m.mu.RLock()
	breakers := make([]*CircuitBreaker, 0, len(m.breakers))
	for _, breaker := range m.breakers {
		breakers = append(breakers, breaker)
	}
	m.mu.RUnlock()

	for _, breaker := range breakers {
		breaker.Reset()
	}

	m.logger.Info("All circuit breakers reset")
}

// GetStats 获取所有熔断器统计
func (m *Manager) GetStats() map[string]BreakerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]BreakerStats)
	for name, breaker := range m.breakers {
		counts := breaker.Counts()
		stats[name] = BreakerStats{
			Name:                 name,
			State:                breaker.State().String(),
			Requests:             counts.Requests,
			TotalSuccesses:       counts.TotalSuccesses,
			TotalFailures:        counts.TotalFailures,
			ConsecutiveSuccesses: counts.ConsecutiveSuccesses,
			ConsecutiveFailures:  counts.ConsecutiveFailures,
			ErrorRate:            counts.ErrorRate(),
		}
	}

	return stats
}

// BreakerStats 熔断器统计信息
type BreakerStats struct {
	Name                 string  `json:"name"`
	State                string  `json:"state"`
	Requests             uint32  `json:"requests"`
	TotalSuccesses       uint32  `json:"total_successes"`
	TotalFailures        uint32  `json:"total_failures"`
	ConsecutiveSuccesses uint32  `json:"consecutive_successes"`
	ConsecutiveFailures  uint32  `json:"consecutive_failures"`
	ErrorRate            float64 `json:"error_rate"`
}

// DefaultConfig 创建默认配置
func DefaultConfig() Config {
	return Config{
		MaxRequests: 5,
		Interval:    10 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			// 默认策略：至少5个请求且错误率>=50% 或 连续失败>=5次
			return counts.Requests >= 5 && (counts.ErrorRate() >= 0.5 || counts.ConsecutiveFailures >= 5)
		},
	}
}

// AggressiveConfig 创建激进配置（更快熔断）
func AggressiveConfig() Config {
	return Config{
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			// 激进策略：至少3个请求且错误率>=30% 或 连续失败>=3次
			return counts.Requests >= 3 && (counts.ErrorRate() >= 0.3 || counts.ConsecutiveFailures >= 3)
		},
	}
}

// ConservativeConfig 创建保守配置（更慢熔断）
func ConservativeConfig() Config {
	return Config{
		MaxRequests: 10,
		Interval:    20 * time.Second,
		Timeout:     120 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			// 保守策略：至少10个请求且错误率>=70% 或 连续失败>=10次
			return counts.Requests >= 10 && (counts.ErrorRate() >= 0.7 || counts.ConsecutiveFailures >= 10)
		},
	}
}
