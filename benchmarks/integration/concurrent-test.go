package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	sessionpb "github.com/aetherflow/aetherflow/api/proto/session"
	statesyncpb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 并发性能测试

func main() {
	fmt.Println("================================")
	fmt.Println("  AetherFlow 并发性能测试")
	fmt.Println("================================")
	fmt.Println()

	// 不同并发级别的测试
	concurrencies := []int{10, 50, 100, 200}

	for _, concurrency := range concurrencies {
		fmt.Printf("🔥 并发度: %d\n", concurrency)
		fmt.Println()

		// 测试 Session Service
		fmt.Println("📊 Session Service...")
		sessionLatencies := testSessionConcurrent(concurrency)
		if len(sessionLatencies) > 0 {
			printConcurrentStats("Session", concurrency, sessionLatencies)
		}
		fmt.Println()

		// 测试 StateSync Service
		fmt.Println("📊 StateSync Service...")
		statesyncLatencies := testStateSyncConcurrent(concurrency)
		if len(statesyncLatencies) > 0 {
			printConcurrentStats("StateSync", concurrency, statesyncLatencies)
		}
		fmt.Println()
		fmt.Println("---")
		fmt.Println()
	}
}

func testSessionConcurrent(concurrency int) []time.Duration {
	// 连接池
	conns := make([]*grpc.ClientConn, concurrency)
	clients := make([]sessionpb.SessionServiceClient, concurrency)

	for i := 0; i < concurrency; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, err := grpc.DialContext(ctx, "localhost:9001",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		cancel()

		if err != nil {
			log.Printf("无法连接到 Session Service: %v", err)
			return nil
		}
		conns[i] = conn
		clients[i] = sessionpb.NewSessionServiceClient(conn)
	}
	defer func() {
		for _, conn := range conns {
			conn.Close()
		}
	}()

	requestsPerClient := 50
	totalRequests := concurrency * requestsPerClient

	var mu sync.Mutex
	latencies := []time.Duration{}

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			client := clients[clientID]

			for j := 0; j < requestsPerClient; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

				start := time.Now()
				resp, err := client.CreateSession(ctx, &sessionpb.CreateSessionRequest{
					UserId:     fmt.Sprintf("user-%d-%d", clientID, j),
					ClientIp:   "127.0.0.1",
					ClientPort: uint32(9000 + clientID),
				})
				latency := time.Since(start)
				cancel()

				if err == nil && resp.Session != nil {
					mu.Lock()
					latencies = append(latencies, latency)
					mu.Unlock()

					// 清理
					ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
					client.DeleteSession(ctx2, &sessionpb.DeleteSessionRequest{
						SessionId: resp.Session.SessionId,
					})
					cancel2()
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	qps := float64(len(latencies)) / duration.Seconds()
	fmt.Printf("  完成 %d/%d 请求，耗时 %.2fs，QPS: %.0f\n", len(latencies), totalRequests, duration.Seconds(), qps)

	return latencies
}

func testStateSyncConcurrent(concurrency int) []time.Duration {
	// 连接池
	conns := make([]*grpc.ClientConn, concurrency)
	clients := make([]statesyncpb.StateSyncServiceClient, concurrency)

	for i := 0; i < concurrency; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, err := grpc.DialContext(ctx, "localhost:9002",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		cancel()

		if err != nil {
			log.Printf("无法连接到 StateSync Service: %v", err)
			return nil
		}
		conns[i] = conn
		clients[i] = statesyncpb.NewStateSyncServiceClient(conn)
	}
	defer func() {
		for _, conn := range conns {
			conn.Close()
		}
	}()

	requestsPerClient := 50
	totalRequests := concurrency * requestsPerClient

	var mu sync.Mutex
	latencies := []time.Duration{}

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			client := clients[clientID]

			for j := 0; j < requestsPerClient; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

				start := time.Now()
				resp, err := client.CreateDocument(ctx, &statesyncpb.CreateDocumentRequest{
					Name:      fmt.Sprintf("Doc-%d-%d", clientID, j),
					Type:      "text",
					CreatedBy: fmt.Sprintf("user-%d", clientID),
					Content:   []byte(fmt.Sprintf("Content %d-%d", clientID, j)),
				})
				latency := time.Since(start)
				cancel()

				if err == nil && resp.Document != nil {
					mu.Lock()
					latencies = append(latencies, latency)
					mu.Unlock()

					// 清理
					ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
					client.DeleteDocument(ctx2, &statesyncpb.DeleteDocumentRequest{
						DocId:  resp.Document.Id,
						UserId: fmt.Sprintf("user-%d", clientID),
					})
					cancel2()
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	qps := float64(len(latencies)) / duration.Seconds()
	fmt.Printf("  完成 %d/%d 请求，耗时 %.2fs，QPS: %.0f\n", len(latencies), totalRequests, duration.Seconds(), qps)

	return latencies
}

type Stats struct {
	Min  time.Duration
	Max  time.Duration
	Avg  time.Duration
	P50  time.Duration
	P95  time.Duration
	P99  time.Duration
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

func printConcurrentStats(name string, concurrency int, latencies []time.Duration) {
	stats := calculateStats(latencies)

	fmt.Printf("  P50: %.2fms  P95: %.2fms  P99: %.2fms  Avg: %.2fms  Max: %.2fms\n",
		float64(stats.P50.Microseconds())/1000.0,
		float64(stats.P95.Microseconds())/1000.0,
		float64(stats.P99.Microseconds())/1000.0,
		float64(stats.Avg.Microseconds())/1000.0,
		float64(stats.Max.Microseconds())/1000.0)
}
