package metrics

import (
	"runtime"
	"time"

	"go.uber.org/zap"
)

// Collector 指标收集器
type Collector struct {
	metrics *Metrics
	logger  *zap.Logger
	stopCh  chan struct{}
}

// NewCollector 创建指标收集器
func NewCollector(metrics *Metrics, logger *zap.Logger) *Collector {
	return &Collector{
		metrics: metrics,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
}

// Start 开始收集系统指标
func (c *Collector) Start() {
	go c.collectLoop()
	c.logger.Info("Metrics collector started")
}

// Stop 停止收集
func (c *Collector) Stop() {
	close(c.stopCh)
	c.logger.Info("Metrics collector stopped")
}

// collectLoop 收集循环
func (c *Collector) collectLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collectSystemMetrics()
		case <-c.stopCh:
			return
		}
	}
}

// collectSystemMetrics 收集系统指标
func (c *Collector) collectSystemMetrics() {
	// 收集 goroutine 数量
	numGoroutines := runtime.NumGoroutine()
	c.metrics.GoRoutines.Set(float64(numGoroutines))

	// 记录内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	c.logger.Debug("System metrics collected",
		zap.Int("goroutines", numGoroutines),
		zap.Uint64("heap_alloc", m.HeapAlloc),
		zap.Uint64("heap_sys", m.HeapSys),
		zap.Uint32("num_gc", m.NumGC),
	)
}
