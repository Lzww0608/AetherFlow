package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	sessionpb "github.com/aetherflow/aetherflow/api/proto/session"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TCP (gRPC) 性能测试
// 用于测量 TCP baseline 性能

var (
	concurrency = flag.Int("concurrency", 50, "并发度")
	duration    = flag.String("duration", "30s", "测试时长")
	target      = flag.String("target", "localhost:9001", "目标服务地址")
)

func main() {
	flag.Parse()

	testDuration, err := time.ParseDuration(*duration)
	if err != nil {
		log.Fatalf("无效的 duration: %v", err)
	}

	fmt.Printf("TCP (gRPC) 性能测试\n")
	fmt.Printf("目标: %s\n", *target)
	fmt.Printf("并发度: %d\n", *concurrency)
	fmt.Printf("测试时长: %s\n", testDuration)
	fmt.Println()

	// 连接池
	conns := make([]*grpc.ClientConn, *concurrency)
	clients := make([]sessionpb.SessionServiceClient, *concurrency)

	for i := 0; i < *concurrency; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		conn, err := grpc.DialContext(ctx, *target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		cancel()

		if err != nil {
			log.Fatalf("无法连接到服务: %v", err)
		}
		conns[i] = conn
		clients[i] = sessionpb.NewSessionServiceClient(conn)
	}
	defer func() {
		for _, conn := range conns {
			conn.Close()
		}
	}()

	// 测试
	var mu sync.Mutex
	latencies := []time.Duration{}
	requestCount := 0
	errorCount := 0

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	startTime := time.Now()

	// 启动并发 goroutine
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			client := clients[clientID]

			for {
				select {
				case <-stopChan:
					return
				default:
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

					start := time.Now()
					resp, err := client.CreateSession(ctx, &sessionpb.CreateSessionRequest{
						UserId:     fmt.Sprintf("user-%d-%d", clientID, time.Now().UnixNano()),
						ClientIp:   "127.0.0.1",
						ClientPort: uint32(9000 + clientID),
					})
					latency := time.Since(start)
					cancel()

					mu.Lock()
					if err == nil && resp.Session != nil {
						latencies = append(latencies, latency)
						requestCount++

						// 清理
						ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
						client.DeleteSession(ctx2, &sessionpb.DeleteSessionRequest{
							SessionId: resp.Session.SessionId,
						})
						cancel2()
					} else {
						errorCount++
					}
					mu.Unlock()
				}
			}
		}(i)
	}

	// 等待测试时长
	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()

	elapsed := time.Since(startTime)

	// 统计
	stats := calculateStats(latencies)
	qps := float64(requestCount) / elapsed.Seconds()
	errorRate := float64(errorCount) / float64(requestCount+errorCount) * 100

	// 输出结果
	fmt.Printf("测试完成\n")
	fmt.Printf("总请求数: %d\n", requestCount)
	fmt.Printf("错误数: %d (%.2f%%)\n", errorCount, errorRate)
	fmt.Printf("耗时: %.2fs\n", elapsed.Seconds())
	fmt.Printf("QPS: %.0f\n", qps)
	fmt.Println()

	fmt.Printf("延迟统计:\n")
	fmt.Printf("  P50: %.2fms\n", float64(stats.P50.Microseconds())/1000.0)
	fmt.Printf("  P95: %.2fms\n", float64(stats.P95.Microseconds())/1000.0)
	fmt.Printf("  P99: %.2fms\n", float64(stats.P99.Microseconds())/1000.0)
	fmt.Printf("  Avg: %.2fms\n", float64(stats.Avg.Microseconds())/1000.0)
	fmt.Printf("  Min: %.2fms\n", float64(stats.Min.Microseconds())/1000.0)
	fmt.Printf("  Max: %.2fms\n", float64(stats.Max.Microseconds())/1000.0)
}

type Stats struct {
	Min time.Duration
	Max time.Duration
	Avg time.Duration
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
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

	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	stats.Avg = sum / time.Duration(len(latencies))

	return stats
}
