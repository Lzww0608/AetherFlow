package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	pb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 冲突演示配置
type ConflictDemoConfig struct {
	StateSyncAddr  string
	ConflictType   string // "concurrent", "sequential"
	ResolutionType string // "lww", "manual", "merge"
	Users          int
}

// 冲突场景
type ConflictScenario struct {
	Name        string
	Description string
	Users       []string
	Operations  []ConflictOperation
}

type ConflictOperation struct {
	UserIndex int
	Type      string
	Position  int
	Data      string
	Delay     time.Duration
}

func main() {
	config := parseConflictFlags()

	fmt.Println("================================")
	fmt.Println("  AetherFlow 冲突解决演示")
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

	// 选择冲突场景
	scenario := selectScenario(config)

	fmt.Printf("📋 场景: %s\n", scenario.Name)
	fmt.Printf("📝 描述: %s\n", scenario.Description)
	fmt.Printf("👥 用户: %d\n", len(scenario.Users))
	fmt.Printf("🔧 解决策略: %s\n", config.ResolutionType)
	fmt.Println()

	// 创建文档
	docID := createConflictDocument(client)
	fmt.Printf("✅ 创建文档: %s\n", docID.String())
	fmt.Println()

	// 运行冲突场景
	runConflictScenario(client, docID, scenario, config)

	// 检查冲突
	checkConflicts(client, docID)

	// 显示最终状态
	displayFinalState(client, docID)
}

func parseConflictFlags() *ConflictDemoConfig {
	config := &ConflictDemoConfig{}

	flag.StringVar(&config.StateSyncAddr, "statesync", "localhost:9002", "StateSync service address")
	flag.StringVar(&config.ConflictType, "conflict-type", "concurrent", "Conflict type: concurrent, sequential")
	flag.StringVar(&config.ResolutionType, "resolution", "lww", "Resolution strategy: lww, manual, merge")
	flag.IntVar(&config.Users, "users", 2, "Number of users")

	flag.Parse()

	return config
}

func selectScenario(config *ConflictDemoConfig) *ConflictScenario {
	if config.ConflictType == "concurrent" {
		return &ConflictScenario{
			Name:        "并发编辑冲突",
			Description: "两个用户同时编辑同一位置",
			Users:       []string{"Alice", "Bob"},
			Operations: []ConflictOperation{
				{UserIndex: 0, Type: "update", Position: 10, Data: "Alice's text", Delay: 0},
				{UserIndex: 1, Type: "update", Position: 10, Data: "Bob's text", Delay: 100 * time.Millisecond},
			},
		}
	} else {
		return &ConflictScenario{
			Name:        "顺序编辑冲突",
			Description: "多个用户依次编辑导致版本冲突",
			Users:       []string{"Alice", "Bob", "Carol"},
			Operations: []ConflictOperation{
				{UserIndex: 0, Type: "insert", Position: 0, Data: "First ", Delay: 0},
				{UserIndex: 1, Type: "insert", Position: 0, Data: "Second ", Delay: 500 * time.Millisecond},
				{UserIndex: 2, Type: "insert", Position: 0, Data: "Third ", Delay: 1 * time.Second},
			},
		}
	}
}

func createConflictDocument(client pb.StateSyncServiceClient) guuid.UUID {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID, _ := guuid.NewV7()

	req := &pb.CreateDocumentRequest{
		Id:        docID.String(),
		Name:      "Conflict Demo Document",
		Type:      "text",
		CreatedBy: "demo-system",
		Content:   []byte("Initial content."),
	}

	_, err := client.CreateDocument(ctx, req)
	if err != nil {
		log.Fatalf("创建文档失败: %v", err)
	}

	return docID
}

func runConflictScenario(client pb.StateSyncServiceClient, docID guuid.UUID,
	scenario *ConflictScenario, config *ConflictDemoConfig) {

	fmt.Println("🚀 开始执行冲突场景...")
	fmt.Println()

	var wg sync.WaitGroup
	results := make(chan OperationResult, len(scenario.Operations))

	// 订阅冲突事件
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wg.Add(1)
	go monitorConflicts(ctx, &wg, client, docID, scenario.Users[0])

	// 执行操作
	for _, op := range scenario.Operations {
		wg.Add(1)
		go func(operation ConflictOperation) {
			defer wg.Done()

			// 延迟
			time.Sleep(operation.Delay)

			userName := scenario.Users[operation.UserIndex]
			userID := fmt.Sprintf("user-%d", operation.UserIndex)
			sessionID := guuid.NewV7().String()

			fmt.Printf("👤 %s 开始 %s 操作 (位置: %d)\n",
				userName, operation.Type, operation.Position)

			// 执行操作
			start := time.Now()

			opID, _ := guuid.NewV7()

			req := &pb.ApplyOperationRequest{
				DocumentId: docID.String(),
				Operation: &pb.Operation{
					Id:        opID.String(),
					DocId:     docID.String(),
					UserId:    userID,
					SessionId: sessionID,
					Type:      operation.Type,
					Data:      []byte(operation.Data),
					Timestamp: time.Now().Unix(),
					ClientId:  userName,
				},
			}

			opCtx, opCancel := context.WithTimeout(context.Background(), 5*time.Second)
			resp, err := client.ApplyOperation(opCtx, req)
			opCancel()

			latency := time.Since(start)

			result := OperationResult{
				UserName: userName,
				OpType:   operation.Type,
				Success:  err == nil,
				Latency:  latency,
				Version:  0,
				Error:    err,
			}

			if err == nil && resp.Operation != nil {
				result.Version = resp.Operation.Version
				fmt.Printf("  ✅ %s: 操作成功 (版本: %d, 延迟: %dms)\n",
					userName, result.Version, latency.Milliseconds())
			} else {
				fmt.Printf("  ❌ %s: 操作失败: %v\n", userName, err)
			}

			results <- result
		}(op)
	}

	// 等待所有操作完成
	wg.Wait()
	close(results)

	// 汇总结果
	fmt.Println()
	fmt.Println("📊 操作结果:")
	successCount := 0
	for result := range results {
		if result.Success {
			successCount++
			fmt.Printf("  ✅ %s: v%d (延迟: %dms)\n",
				result.UserName, result.Version, result.Latency.Milliseconds())
		} else {
			fmt.Printf("  ❌ %s: 失败\n", result.UserName)
		}
	}

	fmt.Printf("\n成功率: %d/%d (%.1f%%)\n",
		successCount, len(scenario.Operations),
		float64(successCount)/float64(len(scenario.Operations))*100)
	fmt.Println()
}

type OperationResult struct {
	UserName string
	OpType   string
	Success  bool
	Latency  time.Duration
	Version  uint64
	Error    error
}

func monitorConflicts(ctx context.Context, wg *sync.WaitGroup,
	client pb.StateSyncServiceClient, docID guuid.UUID, userName string) {
	defer wg.Done()

	stream, err := client.SubscribeDocument(ctx, &pb.SubscribeDocumentRequest{
		DocumentId: docID.String(),
		UserId:     "monitor",
		SessionId:  guuid.NewV7().String(),
	})

	if err != nil {
		log.Printf("订阅失败: %v", err)
		return
	}

	fmt.Println("🔍 监控冲突事件...")
	fmt.Println()

	conflictDetected := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
			event, err := stream.Recv()
			if err != nil {
				return
			}

			if event.Type == "conflict_detected" {
				conflictDetected = true
				fmt.Println()
				fmt.Println("⚠️  ========== 检测到冲突 ==========")
				fmt.Printf("⚠️  文档: %s\n", event.DocumentId)
				fmt.Printf("⚠️  时间: %s\n", time.Now().Format("15:04:05"))
				fmt.Println("⚠️  ===================================")
				fmt.Println()
			}

			if event.Type == "conflict_resolved" {
				fmt.Println()
				fmt.Println("🔧 ========== 冲突已解决 ==========")
				fmt.Printf("🔧 策略: %s\n", event.Metadata["resolution"])
				fmt.Printf("🔧 胜者: %s\n", event.Metadata["winner"])
				fmt.Println("🔧 ===================================")
				fmt.Println()
			}
		}
	}
}

func checkConflicts(client pb.StateSyncServiceClient, docID guuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取冲突历史
	// 注意：这需要在 proto 中定义 ListConflicts 方法
	// 这里简化处理，通过 GetDocument 查看是否有冲突标记

	resp, err := client.GetDocument(ctx, &pb.GetDocumentRequest{
		DocumentId: docID.String(),
	})

	if err != nil {
		log.Printf("获取文档失败: %v", err)
		return
	}

	fmt.Println("🔍 冲突检查:")
	if resp.Document.Metadata != nil {
		if conflictCount, ok := resp.Document.Metadata["conflict_count"]; ok {
			fmt.Printf("  冲突次数: %s\n", conflictCount)
		} else {
			fmt.Println("  无冲突记录")
		}
	}
	fmt.Println()
}

func displayFinalState(client pb.StateSyncServiceClient, docID guuid.UUID) {
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
		Limit:      20,
	})
	if err != nil {
		log.Printf("获取历史失败: %v", err)
		return
	}

	fmt.Println("================================")
	fmt.Println("  📄 最终文档状态")
	fmt.Println("================================")
	fmt.Println()

	fmt.Printf("文档版本:         %d\n", docResp.Document.Version)
	fmt.Printf("内容长度:         %d 字节\n", len(docResp.Document.Content))
	fmt.Printf("操作历史:         %d 条\n", len(histResp.Operations))
	fmt.Println()

	fmt.Println("操作时间线:")
	for i, op := range histResp.Operations {
		status := "✅"
		if op.Status == "conflict" {
			status = "⚠️"
		} else if op.Status == "rejected" {
			status = "❌"
		}

		fmt.Printf("  %s [v%d] %s by %s (%s)\n",
			status, op.Version, op.Type, op.ClientId, op.Status)

		if i >= 9 {
			break
		}
	}

	fmt.Println()
	fmt.Printf("最终内容预览:\n%s\n", string(docResp.Document.Content))
	fmt.Println()

	fmt.Println("✅ 冲突演示完成！")
	fmt.Println()

	fmt.Println("💡 提示:")
	fmt.Println("  - 查看 Jaeger UI 了解详细追踪: http://localhost:16686")
	fmt.Println("  - 查看 Metrics 了解性能指标: http://localhost:9102/metrics")
	fmt.Println()
}
