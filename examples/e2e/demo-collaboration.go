package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	pb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 演示配置
type DemoConfig struct {
	StateSyncAddr string
	Users         int
	Duration      time.Duration
	Operations    int
}

// 用户模拟
type SimulatedUser struct {
	Name      string
	UserID    string
	SessionID string
	Client    pb.StateSyncServiceClient
	DocID     guuid.UUID
}

// 统计信息
type Stats struct {
	mu             sync.Mutex
	TotalOps       int
	SuccessOps     int
	FailedOps      int
	TotalLatency   time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	ConflictCount  int
	ReceivedEvents int
}

func main() {
	config := parseFlags()

	fmt.Println("================================")
	fmt.Println("  AetherFlow 实时协作演示")
	fmt.Println("================================")
	fmt.Println()

	// 连接 StateSync Service
	conn, err := grpc.Dial(config.StateSyncAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	client := pb.NewStateSyncServiceClient(conn)

	// 创建共享文档
	docID := createSharedDocument(client)
	fmt.Printf("✅ 创建共享文档: %s\n", docID.String())
	fmt.Println()

	// 创建模拟用户
	users := make([]*SimulatedUser, config.Users)
	userNames := []string{"Alice", "Bob", "Carol", "Dave", "Eve", "Frank", "Grace", "Helen"}

	for i := 0; i < config.Users; i++ {
		name := userNames[i%len(userNames)]
		if config.Users > len(userNames) {
			name = fmt.Sprintf("%s-%d", name, i/len(userNames)+1)
		}

		users[i] = &SimulatedUser{
			Name:      name,
			UserID:    fmt.Sprintf("user-%d", i+1),
			SessionID: guuid.NewV7().String(),
			Client:    client,
			DocID:     docID,
		}

		fmt.Printf("👤 用户 %s 加入协作\n", users[i].Name)
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("🚀 开始协作演示...")
	fmt.Println()

	// 启动统计
	stats := &Stats{
		MinLatency: time.Hour,
	}

	// 启动所有用户的订阅
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	// 订阅文档更新
	for _, user := range users {
		wg.Add(1)
		go subscribeUpdates(ctx, &wg, user, stats)
	}

	// 模拟协作编辑
	time.Sleep(2 * time.Second) // 等待订阅建立

	// 启动操作协程
	operationsPerUser := config.Operations / config.Users
	for _, user := range users {
		wg.Add(1)
		go performOperations(ctx, &wg, user, operationsPerUser, stats)
	}

	// 等待完成
	fmt.Println("⏳ 等待所有操作完成...")
	wg.Wait()

	// 打印统计
	printStats(stats, config)

	// 获取最终文档状态
	printDocumentState(client, docID)
}

func parseFlags() *DemoConfig {
	config := &DemoConfig{}

	flag.StringVar(&config.StateSyncAddr, "statesync", "localhost:9002", "StateSync service address")
	flag.IntVar(&config.Users, "users", 3, "Number of concurrent users")
	flag.DurationVar(&config.Duration, "duration", 30*time.Second, "Demo duration")
	flag.IntVar(&config.Operations, "operations", 50, "Total operations")

	flag.Parse()

	return config
}

func createSharedDocument(client pb.StateSyncServiceClient) guuid.UUID {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID, _ := guuid.NewV7()

	req := &pb.CreateDocumentRequest{
		Id:        docID.String(),
		Name:      "Collaboration Demo Document",
		Type:      "text",
		CreatedBy: "demo-system",
		Content:   []byte("# Collaboration Demo\n\nThis is a shared document.\n"),
	}

	_, err := client.CreateDocument(ctx, req)
	if err != nil {
		log.Fatalf("创建文档失败: %v", err)
	}

	return docID
}

func subscribeUpdates(ctx context.Context, wg *sync.WaitGroup, user *SimulatedUser, stats *Stats) {
	defer wg.Done()

	stream, err := user.Client.SubscribeDocument(ctx, &pb.SubscribeDocumentRequest{
		DocumentId: user.DocID.String(),
		UserId:     user.UserID,
		SessionId:  user.SessionID,
	})

	if err != nil {
		log.Printf("❌ %s: 订阅失败: %v\n", user.Name, err)
		return
	}

	fmt.Printf("📡 %s: 已订阅文档更新\n", user.Name)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			event, err := stream.Recv()
			if err != nil {
				return
			}

			stats.mu.Lock()
			stats.ReceivedEvents++
			stats.mu.Unlock()

			// 打印接收到的操作
			if event.Type == "operation_applied" {
				fmt.Printf("  📝 %s 收到更新: 操作类型=%s, 版本=%d\n",
					user.Name, event.Operation.Type, event.Operation.Version)
			}

			if event.Type == "conflict_detected" {
				fmt.Printf("  ⚠️  %s 检测到冲突\n", user.Name)
				stats.mu.Lock()
				stats.ConflictCount++
				stats.mu.Unlock()
			}
		}
	}
}

func performOperations(ctx context.Context, wg *sync.WaitGroup, user *SimulatedUser, count int, stats *Stats) {
	defer wg.Done()

	operations := []string{"insert", "delete", "update", "move"}
	contents := []string{
		"Hello from " + user.Name,
		"Collaborative editing is awesome!",
		"Real-time sync works great.",
		"Testing AetherFlow...",
	}

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			// 随机操作类型
			opType := operations[rand.Intn(len(operations))]
			content := contents[rand.Intn(len(contents))]

			// 执行操作
			start := time.Now()

			opID, _ := guuid.NewV7()
			sessionID, _ := guuid.Parse(user.SessionID)

			req := &pb.ApplyOperationRequest{
				DocumentId: user.DocID.String(),
				Operation: &pb.Operation{
					Id:        opID.String(),
					DocId:     user.DocID.String(),
					UserId:    user.UserID,
					SessionId: sessionID.String(),
					Type:      opType,
					Data:      []byte(content),
					Timestamp: time.Now().Unix(),
					ClientId:  user.Name,
				},
			}

			opCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, err := user.Client.ApplyOperation(opCtx, req)
			cancel()

			latency := time.Since(start)

			// 更新统计
			stats.mu.Lock()
			stats.TotalOps++
			if err == nil {
				stats.SuccessOps++
				stats.TotalLatency += latency

				if latency < stats.MinLatency {
					stats.MinLatency = latency
				}
				if latency > stats.MaxLatency {
					stats.MaxLatency = latency
				}

				fmt.Printf("  ✅ %s: %s 操作 (延迟: %dms)\n",
					user.Name, opType, latency.Milliseconds())
			} else {
				stats.FailedOps++
				fmt.Printf("  ❌ %s: 操作失败: %v\n", user.Name, err)
			}
			stats.mu.Unlock()

			// 随机间隔
			time.Sleep(time.Duration(500+rand.Intn(1500)) * time.Millisecond)
		}
	}
}

func printStats(stats *Stats, config *DemoConfig) {
	fmt.Println()
	fmt.Println("================================")
	fmt.Println("  📊 协作统计")
	fmt.Println("================================")
	fmt.Println()

	fmt.Printf("👥 用户数量:        %d\n", config.Users)
	fmt.Printf("⏱️  运行时长:        %s\n", config.Duration)
	fmt.Println()

	fmt.Printf("📝 总操作数:        %d\n", stats.TotalOps)
	fmt.Printf("✅ 成功操作:        %d (%.1f%%)\n",
		stats.SuccessOps,
		float64(stats.SuccessOps)/float64(stats.TotalOps)*100)
	fmt.Printf("❌ 失败操作:        %d\n", stats.FailedOps)
	fmt.Printf("⚠️  冲突数量:        %d\n", stats.ConflictCount)
	fmt.Printf("📡 接收事件:        %d\n", stats.ReceivedEvents)
	fmt.Println()

	if stats.SuccessOps > 0 {
		avgLatency := stats.TotalLatency / time.Duration(stats.SuccessOps)
		fmt.Printf("⚡ 平均延迟:        %dms\n", avgLatency.Milliseconds())
		fmt.Printf("⚡ 最小延迟:        %dms\n", stats.MinLatency.Milliseconds())
		fmt.Printf("⚡ 最大延迟:        %dms\n", stats.MaxLatency.Milliseconds())
		fmt.Println()

		throughput := float64(stats.SuccessOps) / config.Duration.Seconds()
		fmt.Printf("🚀 吞吐量:          %.1f ops/sec\n", throughput)
	}

	fmt.Println()
}

func printDocumentState(client pb.StateSyncServiceClient, docID guuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取文档
	docResp, err := client.GetDocument(ctx, &pb.GetDocumentRequest{
		DocumentId: docID.String(),
	})
	if err != nil {
		log.Printf("获取文档失败: %v", err)
		return
	}

	// 获取操作历史
	histResp, err := client.GetOperationHistory(ctx, &pb.GetOperationHistoryRequest{
		DocumentId: docID.String(),
		Limit:      100,
	})
	if err != nil {
		log.Printf("获取历史失败: %v", err)
		return
	}

	fmt.Println("================================")
	fmt.Println("  📄 文档最终状态")
	fmt.Println("================================")
	fmt.Println()

	fmt.Printf("文档 ID:          %s\n", docResp.Document.Id)
	fmt.Printf("文档名称:         %s\n", docResp.Document.Name)
	fmt.Printf("当前版本:         %d\n", docResp.Document.Version)
	fmt.Printf("活跃用户:         %d\n", len(docResp.Document.ActiveUsers))
	fmt.Printf("操作历史数量:     %d\n", len(histResp.Operations))
	fmt.Println()

	// 显示最近的操作
	fmt.Println("最近的操作:")
	count := 5
	if len(histResp.Operations) < count {
		count = len(histResp.Operations)
	}
	for i := 0; i < count; i++ {
		op := histResp.Operations[i]
		fmt.Printf("  [%d] %s by %s (v%d)\n",
			i+1, op.Type, op.ClientId, op.Version)
	}

	fmt.Println()
	fmt.Println("✅ 演示完成！")
	fmt.Println()
}
