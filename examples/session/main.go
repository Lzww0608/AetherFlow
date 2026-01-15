// Package main demonstrates basic usage of the Session Service
package main

import (
	"context"
	"fmt"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/aetherflow/aetherflow/internal/session"
	"go.uber.org/zap"
)

func main() {
	// 创建日志记录器
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("=== AetherFlow Session Service 示例 ===")

	// 创建内存存储
	store := session.NewMemoryStore()

	// 创建会话管理器
	manager := session.NewManager(&session.ManagerConfig{
		Store:           store,
		Logger:          logger,
		DefaultTimeout:  5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	})
	defer manager.Close()

	ctx := context.Background()

	// 场景1: 创建多个用户会话
	logger.Info("\n--- 场景1: 创建用户会话 ---")

	sessions := make([]*session.Session, 0)
	for i := 1; i <= 3; i++ {
		// 模拟Quantum连接ID
		connID, _ := guuid.NewV7()

		sess, token, err := manager.CreateSession(
			ctx,
			fmt.Sprintf("user%d", i),
			fmt.Sprintf("192.168.1.%d", 100+i),
			uint32(50000+i),
			connID,
			map[string]string{
				"device":  "mobile",
				"version": "1.0.0",
				"app":     "AetherFlow",
			},
		)
		if err != nil {
			logger.Fatal("failed to create session", zap.Error(err))
		}

		sessions = append(sessions, sess)

		logger.Info("会话创建成功",
			zap.String("user_id", sess.UserID),
			zap.String("session_id", sess.SessionID.String()),
			zap.String("token", token[:32]+"..."))
	}

	// 场景2: 查询会话
	logger.Info("\n--- 场景2: 查询会话 ---")

	for _, sess := range sessions {
		retrieved, err := manager.GetSession(ctx, sess.SessionID)
		if err != nil {
			logger.Error("failed to get session", zap.Error(err))
			continue
		}

		logger.Info("会话信息",
			zap.String("user_id", retrieved.UserID),
			zap.String("state", retrieved.State.String()),
			zap.String("client_ip", retrieved.ClientIP),
			zap.Time("created_at", retrieved.CreatedAt))
	}

	// 场景3: 更新会话状态
	logger.Info("\n--- 场景3: 更新会话状态 ---")

	activeState := session.StateActive
	updated, err := manager.UpdateSession(
		ctx,
		sessions[0].SessionID,
		&activeState,
		map[string]string{"last_action": "send_message"},
		&session.Stats{
			PacketsSent:     1000,
			PacketsReceived: 950,
			BytesSent:       102400,
			BytesReceived:   97280,
			Retransmissions: 5,
			CurrentRTTMs:    25,
		},
	)
	if err != nil {
		logger.Fatal("failed to update session", zap.Error(err))
	}

	logger.Info("会话状态已更新",
		zap.String("session_id", updated.SessionID.String()),
		zap.String("new_state", updated.State.String()),
		zap.Uint64("packets_sent", updated.Stats.PacketsSent),
		zap.Uint32("rtt_ms", updated.Stats.CurrentRTTMs))

	// 场景4: 心跳保活
	logger.Info("\n--- 场景4: 会话心跳 ---")

	for i := 0; i < 3; i++ {
		remaining, err := manager.Heartbeat(ctx, sessions[0].SessionID)
		if err != nil {
			logger.Error("heartbeat failed", zap.Error(err))
			break
		}

		logger.Info("心跳成功",
			zap.Int("heartbeat_count", i+1),
			zap.Duration("remaining", remaining))

		time.Sleep(500 * time.Millisecond)
	}

	// 场景5: 列出会话
	logger.Info("\n--- 场景5: 列出所有会话 ---")

	allSessions, total, err := manager.ListSessions(ctx, &session.SessionFilter{})
	if err != nil {
		logger.Fatal("failed to list sessions", zap.Error(err))
	}

	logger.Info("会话列表",
		zap.Int("total", total),
		zap.Int("count", len(allSessions)))

	for i, sess := range allSessions {
		logger.Info(fmt.Sprintf("会话 #%d", i+1),
			zap.String("user_id", sess.UserID),
			zap.String("state", sess.State.String()),
			zap.Duration("age", time.Since(sess.CreatedAt)))
	}

	// 场景6: 按用户查询会话
	logger.Info("\n--- 场景6: 按用户查询 ---")

	_, total, err = manager.ListSessions(ctx, &session.SessionFilter{
		UserID: "user1",
	})
	if err != nil {
		logger.Fatal("failed to list user sessions", zap.Error(err))
	}

	logger.Info("用户会话",
		zap.String("user_id", "user1"),
		zap.Int("session_count", total))

	// 场景7: 获取统计信息
	logger.Info("\n--- 场景7: 获取统计信息 ---")

	stats, err := manager.GetStats(ctx)
	if err != nil {
		logger.Fatal("failed to get stats", zap.Error(err))
	}

	logger.Info("系统统计",
		zap.Int("total_sessions", stats["total"].(int)),
		zap.Int("active_sessions", stats["active"].(int)),
		zap.Int("idle_sessions", stats["idle"].(int)))

	// 场景8: 按连接ID查询
	logger.Info("\n--- 场景8: 按连接ID查询 ---")

	sess, err := manager.GetSessionByConnection(ctx, sessions[1].ConnectionID)
	if err != nil {
		logger.Fatal("failed to get session by connection", zap.Error(err))
	}

	logger.Info("通过连接ID找到会话",
		zap.String("connection_id", sessions[1].ConnectionID.String()),
		zap.String("session_id", sess.SessionID.String()),
		zap.String("user_id", sess.UserID))

	// 场景9: 删除会话
	logger.Info("\n--- 场景9: 删除会话 ---")

	err = manager.DeleteSession(ctx, sessions[2].SessionID, "user logout")
	if err != nil {
		logger.Fatal("failed to delete session", zap.Error(err))
	}

	logger.Info("会话已删除",
		zap.String("user_id", sessions[2].UserID))

	// 验证删除
	_, total, _ = manager.ListSessions(ctx, &session.SessionFilter{})
	logger.Info("删除后剩余会话",
		zap.Int("count", total))

	// 场景10: 会话过期演示
	logger.Info("\n--- 场景10: 会话过期演示 ---")
	logger.Info("创建一个短暂的会话...")

	// 创建一个新的管理器，超时时间很短
	shortTimeoutManager := session.NewManager(&session.ManagerConfig{
		Store:           session.NewMemoryStore(),
		Logger:          logger,
		DefaultTimeout:  2 * time.Second,
		CleanupInterval: 1 * time.Second,
	})
	defer shortTimeoutManager.Close()

	connID, _ := guuid.NewV7()
	tempSession, _, _ := shortTimeoutManager.CreateSession(
		ctx,
		"temp_user",
		"192.168.1.200",
		60000,
		connID,
		nil,
	)

	logger.Info("临时会话已创建，等待过期...",
		zap.String("session_id", tempSession.SessionID.String()))

	// 等待会话过期和清理
	time.Sleep(3 * time.Second)

	// 尝试获取已过期的会话
	_, err = shortTimeoutManager.GetSession(ctx, tempSession.SessionID)
	if err != nil {
		logger.Info("会话已过期并被清理",
			zap.String("error", err.Error()))
	}

	logger.Info("\n=== 示例完成 ===")
	logger.Info("所有会话管理功能演示完毕")
}
