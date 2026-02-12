package server

import (
	"fmt"

	guuid "github.com/Lzww0608/GUUID"
	pb "github.com/aetherflow/aetherflow/api/proto/statesync"
	"github.com/aetherflow/aetherflow/internal/statesync"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SubscribeDocument 订阅文档更新（流式）
func (s *Server) SubscribeDocument(req *pb.SubscribeDocumentRequest, stream pb.StateSyncService_SubscribeDocumentServer) error {
	s.logger.Info("SubscribeDocument called",
		zap.String("doc_id", req.DocId),
		zap.String("user_id", req.UserId),
		zap.String("session_id", req.SessionId))

	// 解析 ID
	docID, err := guuid.Parse(req.DocId)
	if err != nil {
		return fmt.Errorf("invalid doc_id format: %w", err)
	}

	sessionID, err := guuid.Parse(req.SessionId)
	if err != nil {
		return fmt.Errorf("invalid session_id format: %w", err)
	}

	// 订阅文档
	subscriber, err := s.manager.Subscribe(stream.Context(), docID, req.UserId, sessionID)
	if err != nil {
		s.logger.Error("Failed to subscribe", zap.Error(err))
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	s.logger.Info("Subscriber created",
		zap.String("subscriber_id", subscriber.ID),
		zap.String("doc_id", req.DocId))

	// 监听事件并发送到客户端
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Info("Client disconnected",
				zap.String("subscriber_id", subscriber.ID),
				zap.String("doc_id", req.DocId))
			
			// 取消订阅
			_ = s.manager.Unsubscribe(stream.Context(), subscriber.ID, docID, req.UserId)
			return stream.Context().Err()

		case event, ok := <-subscriber.Channel:
			if !ok {
				s.logger.Info("Subscriber channel closed",
					zap.String("subscriber_id", subscriber.ID))
				return nil
			}

			// 转换为 proto 事件
			pbEvent, err := eventToProto(event)
			if err != nil {
				s.logger.Error("Failed to convert event", zap.Error(err))
				continue
			}

			// 发送事件到客户端
			if err := stream.Send(pbEvent); err != nil {
				s.logger.Error("Failed to send event",
					zap.String("subscriber_id", subscriber.ID),
					zap.Error(err))
				
				// 取消订阅
				_ = s.manager.Unsubscribe(stream.Context(), subscriber.ID, docID, req.UserId)
				return err
			}

			s.logger.Debug("Event sent to client",
				zap.String("subscriber_id", subscriber.ID),
				zap.String("event_type", string(event.Type)),
				zap.String("event_id", event.ID.String()))
		}
	}
}

// eventToProto 将内部事件转换为 proto
func eventToProto(event *statesync.Event) (*pb.OperationEvent, error) {
	if event == nil {
		return nil, fmt.Errorf("event is nil")
	}

	pbEvent := &pb.OperationEvent{
		Id:        event.ID.String(),
		Type:      string(event.Type),
		DocId:     event.DocID.String(),
		UserId:    event.UserID,
		Timestamp: timestamppb.New(event.Timestamp),
	}

	// 转换操作（如果存在）
	if event.Operation != nil {
		pbOp, err := operationToProto(event.Operation)
		if err != nil {
			return nil, fmt.Errorf("failed to convert operation: %w", err)
		}
		pbEvent.Operation = pbOp
	}

	// 转换文档（如果存在）
	if event.Document != nil {
		pbDoc, err := documentToProto(event.Document)
		if err != nil {
			return nil, fmt.Errorf("failed to convert document: %w", err)
		}
		pbEvent.Document = pbDoc
	}

	// 转换冲突（如果存在）
	if event.Conflict != nil {
		pbConflict, err := conflictToProto(event.Conflict)
		if err != nil {
			return nil, fmt.Errorf("failed to convert conflict: %w", err)
		}
		pbEvent.Conflict = pbConflict
	}

	// 转换事件数据（如果存在）
	if event.Data != nil {
		// 将 interface{} 转换为 map[string]string（简化处理）
		if dataMap, ok := event.Data.(map[string]string); ok {
			pbEvent.Data = dataMap
		} else {
			// 如果不是 map[string]string，记录日志但不中断
			// 可根据实际需要处理其他类型
		}
	}

	return pbEvent, nil
}

// conflictToProto 将内部冲突转换为 proto
func conflictToProto(conflict *statesync.Conflict) (*pb.Conflict, error) {
	if conflict == nil {
		return nil, fmt.Errorf("conflict is nil")
	}

	pbConflict := &pb.Conflict{
		Id:          conflict.ID.String(),
		DocId:       conflict.DocID.String(),
		Resolution:  string(conflict.Resolution),
		ResolvedBy:  conflict.ResolvedBy,
		Description: conflict.Description,
	}

	// 转换操作列表
	if len(conflict.Ops) > 0 {
		pbConflict.Ops = make([]*pb.Operation, 0, len(conflict.Ops))
		for _, op := range conflict.Ops {
			pbOp, err := operationToProto(op)
			if err != nil {
				return nil, fmt.Errorf("failed to convert operation: %w", err)
			}
			pbConflict.Ops = append(pbConflict.Ops, pbOp)
		}
	}

	// 转换解决后的操作
	if conflict.ResolvedOp != nil {
		pbOp, err := operationToProto(conflict.ResolvedOp)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resolved operation: %w", err)
		}
		pbConflict.ResolvedOp = pbOp
	}

	// 转换时间
	if !conflict.ResolvedAt.IsZero() {
		pbConflict.ResolvedAt = timestamppb.New(conflict.ResolvedAt)
	}

	return pbConflict, nil
}
