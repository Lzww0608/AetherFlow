package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	sessionpb "github.com/aetherflow/aetherflow/api/proto/session"
	statesyncpb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 真实的集成测试 - 测试实际运行的服务

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
	conn, err := grpc.Dial("localhost:9001",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(3*time.Second))

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

		sessionID, _ := guuid.NewV7()
		connID, _ := guuid.NewV7()

		_, err := client.CreateSession(ctx, &sessionpb.CreateSessionRequest{
			UserId:       fmt.Sprintf("user-%d", i),
			ClientIp:     "127.0.0.1",
			ClientPort:   9090,
			ConnectionId: connID.String(),
			SessionId:    sessionID.String(),
		})

		latency := time.Since(start)
		cancel()

		if err == nil {
			latencies = append(latencies, latency)

			if i%10 == 0 {
				fmt.Printf("  进度: %d/%d (最近延迟: %.2fms)\n", i+1, testCount, float64(latency.Microseconds())/1000.0)
			}

			// 清理 - 删除创建的 Session
			ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
			client.DeleteSession(ctx2, &sessionpb.DeleteSessionRequest{
				SessionId: sessionID.String(),
			})
			cancel2()
		}

		// 控制速率
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("  ✅ 完成 %d 次测试\n", len(latencies))

	return latencies
}

func testStateSyncService() []time.Duration {
	// 连接 StateSync Service
	conn, err := grpc.Dial("localhost:9002",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(3*time.Second))

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

		docID, _ := guuid.NewV7()

		_, err := client.CreateDocument(ctx, &statesyncpb.CreateDocumentRequest{
			Id:        docID.String(),
			Name:      fmt.Sprintf("Test Doc %d", i),
			Type:      "text",
			CreatedBy: fmt.Sprintf("user-%d", i),
			Content:   []byte(fmt.Sprintf("Test content %d", i)),
		})

		latency := time.Since(start)
		cancel()

		if err == nil {
			latencies = append(latencies, latency)

			if i%10 == 0 {
				fmt.Printf("  进度: %d/%d (最近延迟: %.2fms)\n", i+1, testCount, float64(latency.Microseconds())/1000.0)
			}

			// 清理 - 删除创建的 Document
			ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
			client.DeleteDocument(ctx2, &statesyncpb.DeleteDocumentRequest{
				DocumentId: docID.String(),
			})
			cancel2()
		}

		// 控制速率
		time.Sleep(50 * time.Millisecond)
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
