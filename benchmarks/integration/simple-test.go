package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	sessionpb "github.com/aetherflow/aetherflow/api/proto/session"
	statesyncpb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 简化版的真实集成测试

func main() {
	fmt.Println("================================")
	fmt.Println("  AetherFlow 真实性能测试")
	fmt.Println("================================")
	fmt.Println()

	// 测试 Session Service
	fmt.Println("📊 测试 Session Service...")
	sessionLatencies := testSessionService()
	if len(sessionLatencies) > 0 {
		printStats("Session Service", sessionLatencies)
	} else {
		fmt.Println("⚠️  Session Service 未运行，跳过测试")
	}
	fmt.Println()

	// 测试 StateSync Service
	fmt.Println("📊 测试 StateSync Service...")
	stateSyncLatencies := testStateSyncService()
	if len(stateSyncLatencies) > 0 {
		printStats("StateSync Service", stateSyncLatencies)
	} else {
		fmt.Println("⚠️  StateSync Service 未运行，跳过测试")
	}
	fmt.Println()

	// 端到端测试
	if len(sessionLatencies) > 0 && len(stateSyncLatencies) > 0 {
		fmt.Println("================================")
		fmt.Println("  📊 端到端性能分析")
		fmt.Println("================================")
		fmt.Println()

		sessionStats := calculateStats(sessionLatencies)
		statesyncStats := calculateStats(stateSyncLatencies)

		fmt.Printf("端到端 P99 估算:\n")
		fmt.Printf("  Session:        %.2fms\n", float64(sessionStats.P99.Microseconds())/1000.0)
		fmt.Printf("  StateSync:      %.2fms\n", float64(statesyncStats.P99.Microseconds())/1000.0)
		fmt.Printf("  Gateway:        ~5ms (估算)\n")
		fmt.Printf("  总计:           ~%.2fms\n",
			float64(sessionStats.P99.Microseconds())/1000.0+
				float64(statesyncStats.P99.Microseconds())/1000.0+5.0)
		fmt.Println()

		targetP99 := 50.0
		actualP99 := float64(sessionStats.P99.Microseconds())/1000.0 +
			float64(statesyncStats.P99.Microseconds())/1000.0 + 5.0

		if actualP99 < targetP99 {
			fmt.Printf("✅ 达成目标: P99 < %.0fms\n", targetP99)
			fmt.Printf("   实际: %.2fms (优于目标 %.2fms)\n", actualP99, targetP99-actualP99)
		} else {
			fmt.Printf("❌ 未达目标: P99 < %.0fms\n", targetP99)
			fmt.Printf("   实际: %.2fms (超出 %.2fms)\n", actualP99, actualP99-targetP99)
		}
	}

	fmt.Println()
}

func testSessionService() []time.Duration {
	// 连接 Session Service
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:9001",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())

	if err != nil {
		log.Printf("无法连接到 Session Service: %v", err)
		return nil
	}
	defer conn.Close()

	client := sessionpb.NewSessionServiceClient(conn)

	latencies := []time.Duration{}
	testCount := 100

	fmt.Printf("  运行 %d 次测试...\n", testCount)

	for i := 0; i < testCount; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// 测量创建 Session 的延迟
		start := time.Now()

		resp, err := client.CreateSession(ctx, &sessionpb.CreateSessionRequest{
			UserId:     fmt.Sprintf("user-%d", i),
			ClientIp:   "127.0.0.1",
			ClientPort: 9090,
		})

		latency := time.Since(start)
		cancel()

		if err == nil && resp.Session != nil {
			latencies = append(latencies, latency)

			if i%20 == 0 && i > 0 {
				fmt.Printf("  进度: %d/%d (最近延迟: %.2fms)\n", i, testCount, float64(latency.Microseconds())/1000.0)
			}

			// 清理 - 删除创建的 Session
			ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
			client.DeleteSession(ctx2, &sessionpb.DeleteSessionRequest{
				SessionId: resp.Session.SessionId,
			})
			cancel2()
		}

		// 控制速率
		time.Sleep(20 * time.Millisecond)
	}

	fmt.Printf("  ✅ 完成 %d 次测试\n", len(latencies))

	return latencies
}

func testStateSyncService() []time.Duration {
	// 连接 StateSync Service
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:9002",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())

	if err != nil {
		log.Printf("无法连接到 StateSync Service: %v", err)
		return nil
	}
	defer conn.Close()

	client := statesyncpb.NewStateSyncServiceClient(conn)

	latencies := []time.Duration{}
	testCount := 100

	fmt.Printf("  运行 %d 次测试...\n", testCount)

	for i := 0; i < testCount; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// 测量创建 Document 的延迟
		start := time.Now()

		resp, err := client.CreateDocument(ctx, &statesyncpb.CreateDocumentRequest{
			Name:      fmt.Sprintf("Test Doc %d", i),
			Type:      "text",
			CreatedBy: fmt.Sprintf("user-%d", i),
			Content:   []byte(fmt.Sprintf("Test content %d", i)),
		})

		latency := time.Since(start)
		cancel()

		if err == nil && resp.Document != nil {
			latencies = append(latencies, latency)

			if i%20 == 0 && i > 0 {
				fmt.Printf("  进度: %d/%d (最近延迟: %.2fms)\n", i, testCount, float64(latency.Microseconds())/1000.0)
			}

			// 清理 - 删除创建的 Document
			ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
			client.DeleteDocument(ctx2, &statesyncpb.DeleteDocumentRequest{
				DocId:  resp.Document.Id,
				UserId: fmt.Sprintf("user-%d", i),
			})
			cancel2()
		}

		// 控制速率
		time.Sleep(20 * time.Millisecond)
	}

	fmt.Printf("  ✅ 完成 %d 次测试\n", len(latencies))

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

func printStats(name string, latencies []time.Duration) {
	stats := calculateStats(latencies)

	fmt.Printf("%s 性能统计:\n", name)
	fmt.Printf("  测试次数:       %d\n", len(latencies))
	fmt.Printf("  P50 延迟:       %.2fms\n", float64(stats.P50.Microseconds())/1000.0)
	fmt.Printf("  P95 延迟:       %.2fms\n", float64(stats.P95.Microseconds())/1000.0)
	fmt.Printf("  P99 延迟:       %.2fms\n", float64(stats.P99.Microseconds())/1000.0)
	fmt.Printf("  平均延迟:       %.2fms\n", float64(stats.Avg.Microseconds())/1000.0)
	fmt.Printf("  最小延迟:       %.2fms\n", float64(stats.Min.Microseconds())/1000.0)
	fmt.Printf("  最大延迟:       %.2fms\n", float64(stats.Max.Microseconds())/1000.0)
}
