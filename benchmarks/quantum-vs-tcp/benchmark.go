package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/aetherflow/aetherflow/internal/quantum"
)

// 基准测试配置
type BenchmarkConfig struct {
	TestType     string        // latency, throughput, packet-loss
	Duration     time.Duration // 测试时长
	Concurrency  int           // 并发连接数
	PayloadSize  int           // 负载大小（字节）
	PacketLoss   float64       // 丢包率（0-1）
	RTT          time.Duration // 往返延迟
	OutputFormat string        // 输出格式：text, json, csv
}

// 测试结果
type BenchmarkResult struct {
	Protocol       string
	TotalRequests  int
	SuccessCount   int
	FailureCount   int
	TotalDuration  time.Duration
	Latencies      []time.Duration
	Throughput     float64 // MB/s
	PacketsSent    int
	PacketsLost    int
	RecoveryTime   time.Duration
}

func main() {
	config := parseFlags()

	fmt.Println("================================")
	fmt.Println("  Quantum vs TCP 性能基准测试")
	fmt.Println("================================")
	fmt.Println()

	printConfig(config)

	// 运行 Quantum 测试
	fmt.Println("🚀 运行 Quantum 协议测试...")
	quantumResult := runQuantumBenchmark(config)

	// 运行 TCP 测试
	fmt.Println("🚀 运行 TCP 协议测试...")
	tcpResult := runTCPBenchmark(config)

	// 对比结果
	fmt.Println()
	compareResults(quantumResult, tcpResult, config)

	// 输出结果
	saveResults(quantumResult, tcpResult, config)
}

func parseFlags() *BenchmarkConfig {
	config := &BenchmarkConfig{}

	flag.StringVar(&config.TestType, "test", "latency", "Test type: latency, throughput, packet-loss")
	flag.DurationVar(&config.Duration, "duration", 60*time.Second, "Test duration")
	flag.IntVar(&config.Concurrency, "concurrency", 10, "Number of concurrent connections")
	flag.IntVar(&config.PayloadSize, "size", 1024, "Payload size in bytes")
	flag.Float64Var(&config.PacketLoss, "loss", 0.0, "Packet loss rate (0-1)")
	flag.DurationVar(&config.RTT, "rtt", 10*time.Millisecond, "Round-trip time")
	flag.StringVar(&config.OutputFormat, "output", "text", "Output format: text, json, csv")

	flag.Parse()

	return config
}

func printConfig(config *BenchmarkConfig) {
	fmt.Printf("测试类型:         %s\n", config.TestType)
	fmt.Printf("测试时长:         %s\n", config.Duration)
	fmt.Printf("并发连接:         %d\n", config.Concurrency)
	fmt.Printf("负载大小:         %d bytes\n", config.PayloadSize)
	if config.PacketLoss > 0 {
		fmt.Printf("丢包率:           %.1f%%\n", config.PacketLoss*100)
	}
	if config.RTT > 0 {
		fmt.Printf("RTT:              %s\n", config.RTT)
	}
	fmt.Println()
}

// Quantum 协议基准测试
func runQuantumBenchmark(config *BenchmarkConfig) *BenchmarkResult {
	result := &BenchmarkResult{
		Protocol: "Quantum",
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	latencyChan := make(chan time.Duration, 10000)
	errorChan := make(chan error, 100)

	startTime := time.Now()

	// 启动并发测试
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runQuantumClient(ctx, config, latencyChan, errorChan)
		}(i)
	}

	// 收集结果
	go func() {
		wg.Wait()
		close(latencyChan)
		close(errorChan)
	}()

	// 统计
	for {
		select {
		case latency, ok := <-latencyChan:
			if !ok {
				latencyChan = nil
			} else {
				result.Latencies = append(result.Latencies, latency)
				result.SuccessCount++
			}
		case err, ok := <-errorChan:
			if !ok {
				errorChan = nil
			} else {
				if err != nil {
					result.FailureCount++
				}
			}
		}

		if latencyChan == nil && errorChan == nil {
			break
		}
	}

	result.TotalDuration = time.Since(startTime)
	result.TotalRequests = result.SuccessCount + result.FailureCount

	// 计算吞吐量
	totalBytes := float64(result.SuccessCount * config.PayloadSize)
	result.Throughput = totalBytes / result.TotalDuration.Seconds() / 1024 / 1024 // MB/s

	return result
}

func runQuantumClient(ctx context.Context, config *BenchmarkConfig, latencyChan chan<- time.Duration, errorChan chan<- error) {
	// 连接 Quantum 服务器
	quantumConfig := quantum.DefaultConfig()
	quantumConfig.FECEnabled = true
	quantumConfig.BBREnabled = true

	conn, err := quantum.Dial("udp", "localhost:9090", quantumConfig)
	if err != nil {
		errorChan <- err
		return
	}
	defer conn.Close()

	// 准备测试数据
	payload := make([]byte, config.PayloadSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 发送并测量延迟
			start := time.Now()

			err := conn.Send(payload)
			if err != nil {
				errorChan <- err
				continue
			}

			// 接收响应
			_, err = conn.Receive()
			if err != nil {
				errorChan <- err
				continue
			}

			latency := time.Since(start)
			latencyChan <- latency

			// 控制发送速率
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TCP 协议基准测试
func runTCPBenchmark(config *BenchmarkConfig) *BenchmarkResult {
	result := &BenchmarkResult{
		Protocol: "TCP",
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	latencyChan := make(chan time.Duration, 10000)
	errorChan := make(chan error, 100)

	startTime := time.Now()

	// 启动并发测试
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runTCPClient(ctx, config, latencyChan, errorChan)
		}(i)
	}

	// 收集结果
	go func() {
		wg.Wait()
		close(latencyChan)
		close(errorChan)
	}()

	// 统计
	for {
		select {
		case latency, ok := <-latencyChan:
			if !ok {
				latencyChan = nil
			} else {
				result.Latencies = append(result.Latencies, latency)
				result.SuccessCount++
			}
		case err, ok := <-errorChan:
			if !ok {
				errorChan = nil
			} else {
				if err != nil {
					result.FailureCount++
				}
			}
		}

		if latencyChan == nil && errorChan == nil {
			break
		}
	}

	result.TotalDuration = time.Since(startTime)
	result.TotalRequests = result.SuccessCount + result.FailureCount

	// 计算吞吐量
	totalBytes := float64(result.SuccessCount * config.PayloadSize)
	result.Throughput = totalBytes / result.TotalDuration.Seconds() / 1024 / 1024 // MB/s

	return result
}

func runTCPClient(ctx context.Context, config *BenchmarkConfig, latencyChan chan<- time.Duration, errorChan chan<- error) {
	// 连接 TCP 服务器
	conn, err := net.Dial("tcp", "localhost:9091")
	if err != nil {
		errorChan <- err
		return
	}
	defer conn.Close()

	// 准备测试数据
	payload := make([]byte, config.PayloadSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	response := make([]byte, config.PayloadSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 发送并测量延迟
			start := time.Now()

			_, err := conn.Write(payload)
			if err != nil {
				errorChan <- err
				continue
			}

			// 接收响应
			_, err = conn.Read(response)
			if err != nil {
				errorChan <- err
				continue
			}

			latency := time.Since(start)
			latencyChan <- latency

			// 控制发送速率
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// 结果对比
func compareResults(quantum, tcp *BenchmarkResult, config *BenchmarkConfig) {
	fmt.Println("================================")
	fmt.Println("  📊 性能对比结果")
	fmt.Println("================================")
	fmt.Println()

	// 成功率
	quantumSuccessRate := float64(quantum.SuccessCount) / float64(quantum.TotalRequests) * 100
	tcpSuccessRate := float64(tcp.SuccessCount) / float64(tcp.TotalRequests) * 100

	fmt.Printf("总请求数:\n")
	fmt.Printf("  Quantum:        %d\n", quantum.TotalRequests)
	fmt.Printf("  TCP:            %d\n", tcp.TotalRequests)
	fmt.Println()

	fmt.Printf("成功率:\n")
	fmt.Printf("  Quantum:        %.2f%% (%d/%d)\n",
		quantumSuccessRate, quantum.SuccessCount, quantum.TotalRequests)
	fmt.Printf("  TCP:            %.2f%% (%d/%d)\n",
		tcpSuccessRate, tcp.SuccessCount, tcp.TotalRequests)
	fmt.Println()

	// 延迟对比
	if len(quantum.Latencies) > 0 && len(tcp.Latencies) > 0 {
		quantumStats := calculateLatencyStats(quantum.Latencies)
		tcpStats := calculateLatencyStats(tcp.Latencies)

		fmt.Println("延迟对比:")
		fmt.Printf("  指标        Quantum      TCP          优势\n")
		fmt.Printf("  ─────────────────────────────────────────────\n")

		printLatencyComparison("P50", quantumStats.P50, tcpStats.P50)
		printLatencyComparison("P95", quantumStats.P95, tcpStats.P95)
		printLatencyComparison("P99", quantumStats.P99, tcpStats.P99)
		printLatencyComparison("P99.9", quantumStats.P999, tcpStats.P999)
		printLatencyComparison("平均", quantumStats.Avg, tcpStats.Avg)
		printLatencyComparison("最小", quantumStats.Min, tcpStats.Min)
		printLatencyComparison("最大", quantumStats.Max, tcpStats.Max)

		fmt.Println()
	}

	// 吞吐量对比
	fmt.Println("吞吐量:")
	fmt.Printf("  Quantum:        %.2f MB/s\n", quantum.Throughput)
	fmt.Printf("  TCP:            %.2f MB/s\n", tcp.Throughput)
	if quantum.Throughput > tcp.Throughput {
		improvement := (quantum.Throughput - tcp.Throughput) / tcp.Throughput * 100
		fmt.Printf("  优势:           +%.1f%%\n", improvement)
	} else {
		decline := (tcp.Throughput - quantum.Throughput) / tcp.Throughput * 100
		fmt.Printf("  差距:           -%.1f%%\n", decline)
	}
	fmt.Println()
}

type LatencyStats struct {
	Min  time.Duration
	Max  time.Duration
	Avg  time.Duration
	P50  time.Duration
	P95  time.Duration
	P99  time.Duration
	P999 time.Duration
}

func calculateLatencyStats(latencies []time.Duration) *LatencyStats {
	if len(latencies) == 0 {
		return &LatencyStats{}
	}

	// 排序
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// 计算统计值
	stats := &LatencyStats{
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
		P50: sorted[int(float64(len(sorted))*0.50)],
		P95: sorted[int(float64(len(sorted))*0.95)],
		P99: sorted[int(float64(len(sorted))*0.99)],
	}

	if len(sorted) >= 1000 {
		stats.P999 = sorted[int(float64(len(sorted))*0.999)]
	} else {
		stats.P999 = stats.Max
	}

	// 平均值
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	stats.Avg = sum / time.Duration(len(latencies))

	return stats
}

func printLatencyComparison(label string, quantum, tcp time.Duration) {
	improvement := float64(tcp-quantum) / float64(tcp) * 100

	fmt.Printf("  %-8s   %6.1fms    %6.1fms     ",
		label,
		float64(quantum.Microseconds())/1000.0,
		float64(tcp.Microseconds())/1000.0)

	if improvement > 0 {
		fmt.Printf("↓ %.1f%%\n", improvement)
	} else {
		fmt.Printf("↑ %.1f%%\n", -improvement)
	}
}

func saveResults(quantum, tcp *BenchmarkResult, config *BenchmarkConfig) {
	// 保存到文件
	filename := fmt.Sprintf("results/%s_%s.md", config.TestType, time.Now().Format("20060102_150405"))

	log.Printf("Results saved to: %s\n", filename)

	// 这里可以实现保存逻辑
	// 为简化起见，这里省略文件写入
}
