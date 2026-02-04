package breaker

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

var (
	// ErrFallbackFailed 降级失败错误
	ErrFallbackFailed = errors.New("fallback failed")
)

// FallbackFunc 降级函数
type FallbackFunc func(ctx context.Context, err error) error

// Fallback 降级策略
type Fallback struct {
	name    string
	breaker *CircuitBreaker
	fn      FallbackFunc
	logger  *zap.Logger
}

// NewFallback 创建降级策略
func NewFallback(name string, breaker *CircuitBreaker, fn FallbackFunc, logger *zap.Logger) *Fallback {
	return &Fallback{
		name:    name,
		breaker: breaker,
		fn:      fn,
		logger:  logger,
	}
}

// Execute 执行（带降级）
func (f *Fallback) Execute(ctx context.Context, mainFn func(context.Context) error) error {
	err := f.breaker.ExecuteContext(ctx, mainFn)
	
	// 如果主函数执行成功，直接返回
	if err == nil {
		return nil
	}
	
	// 如果熔断器打开或执行失败，触发降级
	if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrTooManyRequests) {
		f.logger.Warn("Circuit breaker triggered, executing fallback",
			zap.String("name", f.name),
			zap.Error(err),
		)
	}
	
	// 执行降级函数
	if f.fn != nil {
		fallbackErr := f.fn(ctx, err)
		if fallbackErr != nil {
			f.logger.Error("Fallback execution failed",
				zap.String("name", f.name),
				zap.Error(fallbackErr),
			)
			return fallbackErr
		}
		return nil
	}
	
	// 如果没有降级函数，返回原错误
	return err
}

// DefaultFallbackStrategy 默认降级策略工厂
type DefaultFallbackStrategy struct {
	logger *zap.Logger
}

// NewDefaultFallbackStrategy 创建默认降级策略工厂
func NewDefaultFallbackStrategy(logger *zap.Logger) *DefaultFallbackStrategy {
	return &DefaultFallbackStrategy{
		logger: logger,
	}
}

// CacheFirst 缓存优先策略
func (s *DefaultFallbackStrategy) CacheFirst(cacheKey string, cacheFn func(string) (interface{}, error)) FallbackFunc {
	return func(ctx context.Context, err error) error {
		s.logger.Info("Executing cache-first fallback",
			zap.String("cache_key", cacheKey),
		)
		
		data, cacheErr := cacheFn(cacheKey)
		if cacheErr != nil {
			return cacheErr
		}
		
		// 将缓存数据写入context
		// 实际使用时需要根据业务需求实现
		_ = data
		return nil
	}
}

// DefaultResponse 默认响应策略
func (s *DefaultFallbackStrategy) DefaultResponse(defaultData interface{}) FallbackFunc {
	return func(ctx context.Context, err error) error {
		s.logger.Info("Executing default-response fallback")
		
		// 返回默认数据
		// 实际使用时需要根据业务需求实现
		_ = defaultData
		return nil
	}
}

// FailFast 快速失败策略（不降级）
func (s *DefaultFallbackStrategy) FailFast() FallbackFunc {
	return func(ctx context.Context, err error) error {
		s.logger.Info("Executing fail-fast fallback")
		return err
	}
}

// Silent 静默失败策略
func (s *DefaultFallbackStrategy) Silent() FallbackFunc {
	return func(ctx context.Context, err error) error {
		s.logger.Info("Executing silent fallback",
			zap.Error(err),
		)
		// 忽略错误，返回nil
		return nil
	}
}

// Retry 重试策略
func (s *DefaultFallbackStrategy) Retry(maxRetries int, fn func(context.Context) error) FallbackFunc {
	return func(ctx context.Context, err error) error {
		s.logger.Info("Executing retry fallback",
			zap.Int("max_retries", maxRetries),
		)
		
		for i := 0; i < maxRetries; i++ {
			retryErr := fn(ctx)
			if retryErr == nil {
				s.logger.Info("Retry succeeded",
					zap.Int("attempt", i+1),
				)
				return nil
			}
			
			s.logger.Warn("Retry failed",
				zap.Int("attempt", i+1),
				zap.Error(retryErr),
			)
		}
		
		return err
	}
}
