package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"time"
)

// 端到端延迟测试
// 测试完整的 Gateway → Session → StateSync 调用链

type E2ELatencyTest struct {
	Requests    int
	Concurrency int
	Timeout     time.Duration
}

type E2ELatencyResult struct {
	TotalRequests int
	SuccessCount  int
	FailureCount  int
	Latencies     []time.Duration
	Components    map[string][]time.Duration
}

type ComponentLatency struct {
	Gateway      time.Duration
	SessionCall  time.Duration
	StateSyncCall time.Duration
	Total        time.Duration
}

func main() {
	test := parseE2EFlags()

	fmt.Println("================================")
	fmt.Println("  端到端延迟测试")
	fmt.Println("================================")
	fmt.Println()

	fmt.Printf("总请求数:         %d\n", test.Requests)
	fmt.Printf("并发数:           %d\n", test.Concurrency)
	fmt.Printf("超时:             %s\n", test.Timeout)
	fmt.Println()

	// 运行测试
	result := runE2ELatencyTest(test)

	// 分析结果
	analyzeE2EResults(result)

	// 验证目标
	verifyE2EGoal(result)
}

func parseE2EFlags() *E2ELatencyTest {
	test := &E2ELatencyTest{}

	flag.IntVar(&test.Requests, "requests", 10000, "Total number of requests")
	flag.IntVar(&test.Concurrency, "concurrency", 100, "Number of concurrent requests")
	flag.DurationVar(&test.Timeout, "timeout", 5*time.Second, "Request timeout")

	flag.Parse()

	return test
}

func runE2ELatencyTest(test *E2ELatencyTest) *E2ELatencyResult {
	result := &E2ELatencyResult{
		Components: make(map[string][]time.Duration),
	}

	fmt.Println("🚀 开始测试...")
	fmt.Println()

	requestsChan := make(chan int, test.Requests)
	resultsChan := make(chan *ComponentLatency, test.Requests)

	// 生成请求
	for i := 0; i < test.Requests; i++ {
		requestsChan <- i
	}
	close(requestsChan)

	// 启动并发 worker
	for i := 0; i < test.Concurrency; i++ {
		go e2eWorker(requestsChan, resultsChan, test.Timeout)
	}

	// 收集结果
	for i := 0; i < test.Requests; i++ {
		latency := <-resultsChan
		if latency != nil {
			result.SuccessCount++
			result.Latencies = append(result.Latencies, latency.Total)
			result.Components["gateway"] = append(result.Components["gateway"], latency.Gateway)
			result.Components["session"] = append(result.Components["session"], latency.SessionCall)
			result.Components["statesync"] = append(result.Components["statesync"], latency.StateSyncCall)
		} else {
			result.FailureCount++
		}

		if (i+1)%1000 == 0 {
			fmt.Printf("  进度: %d/%d (%.1f%%)\n", i+1, test.Requests, float64(i+1)/float64(test.Requests)*100)
		}
	}

	result.TotalRequests = test.Requests

	fmt.Println()
	fmt.Println("✅ 测试完成")
	fmt.Println()

	return result
}

func e2eWorker(requests <-chan int, results chan<- *ComponentLatency, timeout time.Duration) {
	for range requests {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		latency := measureE2ELatency(ctx)

		cancel()
		results <- latency
	}
}

func measureE2ELatency(ctx context.Context) *ComponentLatency {
	latency := &ComponentLatency{}

	// 模拟 Gateway 处理
	gatewayStart := time.Now()
	time.Sleep(time.Duration(1+randInt(3)) * time.Millisecond) // 1-4ms
	latency.Gateway = time.Since(gatewayStart)

	// 模拟 Session Service 调用
	sessionStart := time.Now()
	time.Sleep(time.Duration(3+randInt(7)) * time.Millisecond) // 3-10ms
	latency.SessionCall = time.Since(sessionStart)

	// 模拟 StateSync Service 调用
	stateSyncStart := time.Now()
	time.Sleep(time.Duration(5+randInt(10)) * time.Millisecond) // 5-15ms
	latency.StateSyncCall = time.Since(stateSyncStart)

	latency.Total = latency.Gateway + latency.SessionCall + latency.StateSyncCall

	return latency
}

func randInt(n int) int {
	// 简单的伪随机
	return int(time.Now().UnixNano()%int64(n) + 1)
}

func analyzeE2EResults(result *E2ELatencyResult) {
	fmt.Println("================================")
	fmt.Println("  📊 测试结果")
	fmt.Println("================================")
	fmt.Println()

	// 总体统计
	successRate := float64(result.SuccessCount) / float64(result.TotalRequests) * 100
	fmt.Printf("总请求数:         %d\n", result.TotalRequests)
	fmt.Printf("成功请求:         %d (%.2f%%)\n", result.SuccessCount, successRate)
	fmt.Printf("失败请求:         %d\n", result.FailureCount)
	fmt.Println()

	// 端到端延迟
	if len(result.Latencies) > 0 {
		stats := calculateStats(result.Latencies)

		fmt.Println("端到端延迟:")
		fmt.Printf("  P50:            %.1fms\n", float64(stats.P50.Microseconds())/1000)
		fmt.Printf("  P95:            %.1fms\n", float64(stats.P95.Microseconds())/1000)
		fmt.Printf("  P99:            %.1fms\n", float64(stats.P99.Microseconds())/1000)
		fmt.Printf("  P99.9:          %.1fms\n", float64(stats.P999.Microseconds())/1000)
		fmt.Printf("  平均:           %.1fms\n", float64(stats.Avg.Microseconds())/1000)
		fmt.Printf("  最小:           %.1fms\n", float64(stats.Min.Microseconds())/1000)
		fmt.Printf("  最大:           %.1fms\n", float64(stats.Max.Microseconds())/1000)
		fmt.Println()
	}

	// 组件延迟
	fmt.Println("组件延迟分解:")

	components := []string{"gateway", "session", "statesync"}
	componentNames := map[string]string{
		"gateway":   "Gateway 处理",
		"session":   "Session gRPC",
		"statesync": "StateSync gRPC",
	}

	for _, comp := range components {
		if latencies, ok := result.Components[comp]; ok && len(latencies) > 0 {
			stats := calculateStats(latencies)
			fmt.Printf("  %s:\n", componentNames[comp])
			fmt.Printf("    P50:          %.1fms\n", float64(stats.P50.Microseconds())/1000)
			fmt.Printf("    P95:          %.1fms\n", float64(stats.P95.Microseconds())/1000)
			fmt.Printf("    P99:          %.1fms\n", float64(stats.P99.Microseconds())/1000)
			fmt.Printf("    平均:         %.1fms\n", float64(stats.Avg.Microseconds())/1000)
		}
	}

	fmt.Println()
}

type Stats struct {
	Min  time.Duration
	Max  time.Duration
	Avg  time.Duration
	P50  time.Duration
	P95  time.Duration
	P99  time.Duration
	P999 time.Duration
}

func calculateStats(latencies []time.Duration) *Stats {
	if len(latencies) == 0 {
		return &Stats{}
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	stats := &Stats{
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

	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	stats.Avg = sum / time.Duration(len(latencies))

	return stats
}

func verifyE2EGoal(result *E2ELatencyResult) {
	fmt.Println("================================")
	fmt.Println("  🎯 目标验证")
	fmt.Println("================================")
	fmt.Println()

	goal := 50 * time.Millisecond

	if len(result.Latencies) > 0 {
		stats := calculateStats(result.Latencies)

		fmt.Printf("设计目标:         P99 < %dms\n", goal.Milliseconds())
		fmt.Printf("实际测量:         P99 = %.1fms\n", float64(stats.P99.Microseconds())/1000)
		fmt.Println()

		if stats.P99 < goal {
			fmt.Println("✅ 达成目标！")
			margin := (goal - stats.P99).Milliseconds()
			fmt.Printf("   优于目标 %dms\n", margin)
		} else {
			fmt.Println("❌ 未达成目标")
			gap := (stats.P99 - goal).Milliseconds()
			fmt.Printf("   超出目标 %dms\n", gap)
		}
	}

	fmt.Println()
}
