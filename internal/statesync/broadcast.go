package statesync

import (
	"context"
	"fmt"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"go.uber.org/zap"
)

// Broadcaster 广播器接口
type Broadcaster interface {
	// Subscribe 订阅文档变更
	Subscribe(ctx context.Context, docID guuid.UUID, userID string, sessionID guuid.UUID) (*Subscriber, error)

	// Unsubscribe 取消订阅
	Unsubscribe(subscriberID string) error

	// Broadcast 广播事件到所有订阅者
	Broadcast(ctx context.Context, event *Event) error

	// BroadcastToDocument 广播到特定文档的订阅者
	BroadcastToDocument(ctx context.Context, docID guuid.UUID, event *Event) error

	// BroadcastToUser 广播到特定用户
	BroadcastToUser(ctx context.Context, userID string, event *Event) error

	// GetSubscribers 获取文档的订阅者列表
	GetSubscribers(docID guuid.UUID) []*Subscriber

	// GetSubscriberCount 获取订阅者数量
	GetSubscriberCount(docID guuid.UUID) int

	// Close 关闭广播器
	Close() error
}

// MemoryBroadcaster 内存广播器实现
type MemoryBroadcaster struct {
	mu sync.RWMutex

	// 订阅者管理
	subscribers       map[string]*Subscriber  // subscriberID -> Subscriber
	subscribersByDoc  map[guuid.UUID][]string // docID -> []subscriberID
	subscribersByUser map[string][]string     // userID -> []subscriberID

	// 配置
	channelBufferSize int
	logger            *zap.Logger

	// 状态
	closed bool
}

// NewMemoryBroadcaster 创建内存广播器
func NewMemoryBroadcaster(logger *zap.Logger) *MemoryBroadcaster {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MemoryBroadcaster{
		subscribers:       make(map[string]*Subscriber),
		subscribersByDoc:  make(map[guuid.UUID][]string),
		subscribersByUser: make(map[string][]string),
		channelBufferSize: 100, // 默认缓冲区大小
		logger:            logger,
		closed:            false,
	}
}

// Subscribe 订阅文档变更
func (b *MemoryBroadcaster) Subscribe(ctx context.Context, docID guuid.UUID, userID string, sessionID guuid.UUID) (*Subscriber, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, fmt.Errorf("broadcaster is closed")
	}

	// 生成订阅者ID
	subID, err := guuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate subscriber ID: %w", err)
	}

	// 创建订阅者
	subscriber := &Subscriber{
		ID:        subID.String(),
		UserID:    userID,
		SessionID: sessionID,
		DocID:     docID,
		Channel:   make(chan *Event, b.channelBufferSize),
		CreatedAt: time.Now(),
		Active:    true,
	}

	// 存储订阅者
	b.subscribers[subscriber.ID] = subscriber

	// 更新索引
	b.subscribersByDoc[docID] = append(b.subscribersByDoc[docID], subscriber.ID)
	b.subscribersByUser[userID] = append(b.subscribersByUser[userID], subscriber.ID)

	b.logger.Info("New subscriber added",
		zap.String("subscriber_id", subscriber.ID),
		zap.String("user_id", userID),
		zap.String("doc_id", docID.String()),
	)

	// 发送欢迎事件
	welcomeEvent := &Event{
		ID:        subID,
		Type:      EventTypeUserJoined,
		DocID:     docID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"message": "Subscribed successfully"},
	}

	// 非阻塞发送
	select {
	case subscriber.Channel <- welcomeEvent:
	default:
		b.logger.Warn("Failed to send welcome event: channel full",
			zap.String("subscriber_id", subscriber.ID),
		)
	}

	return subscriber, nil
}

// Unsubscribe 取消订阅
func (b *MemoryBroadcaster) Unsubscribe(subscriberID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscriber, exists := b.subscribers[subscriberID]
	if !exists {
		return fmt.Errorf("subscriber not found")
	}

	// 标记为不活跃
	subscriber.Active = false

	// 关闭通道
	close(subscriber.Channel)

	// 从订阅者映射中删除
	delete(b.subscribers, subscriberID)

	// 从文档索引中删除
	docSubs := b.subscribersByDoc[subscriber.DocID]
	for i, id := range docSubs {
		if id == subscriberID {
			b.subscribersByDoc[subscriber.DocID] = append(docSubs[:i], docSubs[i+1:]...)
			break
		}
	}

	// 从用户索引中删除
	userSubs := b.subscribersByUser[subscriber.UserID]
	for i, id := range userSubs {
		if id == subscriberID {
			b.subscribersByUser[subscriber.UserID] = append(userSubs[:i], userSubs[i+1:]...)
			break
		}
	}

	b.logger.Info("Subscriber removed",
		zap.String("subscriber_id", subscriberID),
		zap.String("user_id", subscriber.UserID),
		zap.String("doc_id", subscriber.DocID.String()),
	)

	return nil
}

// Broadcast 广播事件到所有订阅者
func (b *MemoryBroadcaster) Broadcast(ctx context.Context, event *Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return fmt.Errorf("broadcaster is closed")
	}

	count := 0
	failed := 0

	for _, subscriber := range b.subscribers {
		if !subscriber.Active {
			continue
		}

		// 非阻塞发送
		select {
		case subscriber.Channel <- event:
			count++
		case <-time.After(100 * time.Millisecond):
			failed++
			b.logger.Warn("Failed to broadcast event: timeout",
				zap.String("subscriber_id", subscriber.ID),
				zap.String("event_type", string(event.Type)),
			)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	b.logger.Debug("Event broadcasted",
		zap.String("event_type", string(event.Type)),
		zap.Int("sent", count),
		zap.Int("failed", failed),
	)

	return nil
}

// BroadcastToDocument 广播到特定文档的订阅者
func (b *MemoryBroadcaster) BroadcastToDocument(ctx context.Context, docID guuid.UUID, event *Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return fmt.Errorf("broadcaster is closed")
	}

	subscriberIDs, exists := b.subscribersByDoc[docID]
	if !exists || len(subscriberIDs) == 0 {
		return nil // 没有订阅者
	}

	count := 0
	failed := 0

	for _, subID := range subscriberIDs {
		subscriber, exists := b.subscribers[subID]
		if !exists || !subscriber.Active {
			continue
		}

		// 非阻塞发送
		select {
		case subscriber.Channel <- event:
			count++
		case <-time.After(100 * time.Millisecond):
			failed++
			b.logger.Warn("Failed to broadcast to document: timeout",
				zap.String("subscriber_id", subscriber.ID),
				zap.String("doc_id", docID.String()),
			)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	b.logger.Debug("Event broadcasted to document",
		zap.String("doc_id", docID.String()),
		zap.String("event_type", string(event.Type)),
		zap.Int("sent", count),
		zap.Int("failed", failed),
	)

	return nil
}

// BroadcastToUser 广播到特定用户
func (b *MemoryBroadcaster) BroadcastToUser(ctx context.Context, userID string, event *Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return fmt.Errorf("broadcaster is closed")
	}

	subscriberIDs, exists := b.subscribersByUser[userID]
	if !exists || len(subscriberIDs) == 0 {
		return nil // 没有订阅者
	}

	count := 0
	failed := 0

	for _, subID := range subscriberIDs {
		subscriber, exists := b.subscribers[subID]
		if !exists || !subscriber.Active {
			continue
		}

		// 非阻塞发送
		select {
		case subscriber.Channel <- event:
			count++
		case <-time.After(100 * time.Millisecond):
			failed++
			b.logger.Warn("Failed to broadcast to user: timeout",
				zap.String("subscriber_id", subscriber.ID),
				zap.String("user_id", userID),
			)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	b.logger.Debug("Event broadcasted to user",
		zap.String("user_id", userID),
		zap.String("event_type", string(event.Type)),
		zap.Int("sent", count),
		zap.Int("failed", failed),
	)

	return nil
}

// GetSubscribers 获取文档的订阅者列表
func (b *MemoryBroadcaster) GetSubscribers(docID guuid.UUID) []*Subscriber {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subscriberIDs, exists := b.subscribersByDoc[docID]
	if !exists {
		return []*Subscriber{}
	}

	result := make([]*Subscriber, 0, len(subscriberIDs))
	for _, subID := range subscriberIDs {
		if subscriber, exists := b.subscribers[subID]; exists && subscriber.Active {
			// 创建副本 (不包含Channel)
			subCopy := &Subscriber{
				ID:        subscriber.ID,
				UserID:    subscriber.UserID,
				SessionID: subscriber.SessionID,
				DocID:     subscriber.DocID,
				CreatedAt: subscriber.CreatedAt,
				Active:    subscriber.Active,
			}
			result = append(result, subCopy)
		}
	}

	return result
}

// GetSubscriberCount 获取订阅者数量
func (b *MemoryBroadcaster) GetSubscriberCount(docID guuid.UUID) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subscriberIDs, exists := b.subscribersByDoc[docID]
	if !exists {
		return 0
	}

	count := 0
	for _, subID := range subscriberIDs {
		if subscriber, exists := b.subscribers[subID]; exists && subscriber.Active {
			count++
		}
	}

	return count
}

// Close 关闭广播器
func (b *MemoryBroadcaster) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// 关闭所有订阅者的通道
	for _, subscriber := range b.subscribers {
		if subscriber.Active {
			subscriber.Active = false
			close(subscriber.Channel)
		}
	}

	// 清空所有映射
	b.subscribers = make(map[string]*Subscriber)
	b.subscribersByDoc = make(map[guuid.UUID][]string)
	b.subscribersByUser = make(map[string][]string)

	b.logger.Info("Broadcaster closed")

	return nil
}

// CleanInactiveSubscribers 清理不活跃的订阅者
func (b *MemoryBroadcaster) CleanInactiveSubscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	count := 0
	inactiveIDs := make([]string, 0)

	// 找出不活跃的订阅者
	for id, subscriber := range b.subscribers {
		if !subscriber.Active {
			inactiveIDs = append(inactiveIDs, id)
		}
	}

	// 删除不活跃的订阅者
	for _, id := range inactiveIDs {
		subscriber := b.subscribers[id]
		delete(b.subscribers, id)

		// 从索引中删除
		docSubs := b.subscribersByDoc[subscriber.DocID]
		for i, subID := range docSubs {
			if subID == id {
				b.subscribersByDoc[subscriber.DocID] = append(docSubs[:i], docSubs[i+1:]...)
				break
			}
		}

		userSubs := b.subscribersByUser[subscriber.UserID]
		for i, subID := range userSubs {
			if subID == id {
				b.subscribersByUser[subscriber.UserID] = append(userSubs[:i], userSubs[i+1:]...)
				break
			}
		}

		count++
	}

	if count > 0 {
		b.logger.Info("Cleaned inactive subscribers",
			zap.Int("count", count),
		)
	}

	return count
}
