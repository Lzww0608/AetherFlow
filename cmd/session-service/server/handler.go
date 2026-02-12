package server

import (
	"context"
	"fmt"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	pb "github.com/aetherflow/aetherflow/api/proto/session"
	"github.com/aetherflow/aetherflow/internal/session"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateSession 创建新会话
func (s *Server) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	s.logger.Info("CreateSession called",
		zap.String("user_id", req.UserId),
		zap.String("client_ip", req.ClientIp))

	// 验证请求
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.ClientIp == "" {
		return nil, status.Error(codes.InvalidArgument, "client_ip is required")
	}

	// 生成连接 ID (模拟 Quantum 连接)
	connID, err := guuid.NewV7()
	if err != nil {
		s.logger.Error("Failed to generate connection ID", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to generate connection ID")
	}

	// 创建会话
	sess, token, err := s.manager.CreateSession(
		ctx,
		req.UserId,
		req.ClientIp,
		req.ClientPort,
		connID,
		req.Metadata,
	)
	if err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("Session created successfully",
		zap.String("session_id", sess.SessionID.String()),
		zap.String("user_id", sess.UserID))

	// 转换为 proto
	pbSession, err := sessionToProto(sess)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateSessionResponse{
		Session: pbSession,
		Token:   token,
	}, nil
}

// GetSession 获取会话信息
func (s *Server) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	s.logger.Debug("GetSession called", zap.String("session_id", req.SessionId))

	// 解析会话 ID
	sessionID, err := guuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id format")
	}

	// 获取会话
	sess, err := s.manager.GetSession(ctx, sessionID)
	if err != nil {
		s.logger.Error("Failed to get session", zap.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	// 转换为 proto
	pbSession, err := sessionToProto(sess)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetSessionResponse{
		Session: pbSession,
	}, nil
}

// UpdateSession 更新会话
func (s *Server) UpdateSession(ctx context.Context, req *pb.UpdateSessionRequest) (*pb.UpdateSessionResponse, error) {
	s.logger.Info("UpdateSession called", zap.String("session_id", req.SessionId))

	// 解析会话 ID
	sessionID, err := guuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id format")
	}

	// 转换状态
	var state *session.State
	if req.State != pb.SessionState_SESSION_STATE_UNSPECIFIED {
		s := protoToState(req.State)
		state = &s
	}

	// 转换统计信息
	var stats *session.Stats
	if req.Stats != nil {
		stats = &session.Stats{
			PacketsSent:     req.Stats.PacketsSent,
			PacketsReceived: req.Stats.PacketsReceived,
			BytesSent:       req.Stats.BytesSent,
			BytesReceived:   req.Stats.BytesReceived,
			Retransmissions: req.Stats.Retransmissions,
			CurrentRTTMs:    req.Stats.CurrentRttMs,
		}
	}

	// 更新会话
	sess, err := s.manager.UpdateSession(ctx, sessionID, state, req.Metadata, stats)
	if err != nil {
		s.logger.Error("Failed to update session", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	// 转换为 proto
	pbSession, err := sessionToProto(sess)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.UpdateSessionResponse{
		Session: pbSession,
	}, nil
}

// DeleteSession 删除会话
func (s *Server) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.DeleteSessionResponse, error) {
	s.logger.Info("DeleteSession called",
		zap.String("session_id", req.SessionId),
		zap.String("reason", req.Reason))

	// 解析会话 ID
	sessionID, err := guuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id format")
	}

	// 删除会话
	err = s.manager.DeleteSession(ctx, sessionID, req.Reason)
	if err != nil {
		s.logger.Error("Failed to delete session", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("Session deleted successfully", zap.String("session_id", req.SessionId))

	return &pb.DeleteSessionResponse{
		Success: true,
	}, nil
}

// ListSessions 列出会话
func (s *Server) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	s.logger.Debug("ListSessions called",
		zap.String("user_id", req.UserId),
		zap.Uint32("page", req.Page))

	// 构建过滤器
	filter := &session.SessionFilter{
		UserID: req.UserId,
		Page:   int(req.Page),
		Limit:  int(req.PageSize),
	}

	// 转换状态过滤
	if req.State != pb.SessionState_SESSION_STATE_UNSPECIFIED {
		state := protoToState(req.State)
		filter.State = &state
	}

	// 列出会话
	sessions, total, err := s.manager.ListSessions(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list sessions", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	// 转换为 proto
	pbSessions := make([]*pb.Session, 0, len(sessions))
	for _, sess := range sessions {
		pbSess, err := sessionToProto(sess)
		if err != nil {
			s.logger.Error("Failed to convert session", zap.Error(err))
			continue
		}
		pbSessions = append(pbSessions, pbSess)
	}

	return &pb.ListSessionsResponse{
		Sessions: pbSessions,
		Total:    uint32(total),
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Heartbeat 处理心跳
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.logger.Debug("Heartbeat called", zap.String("session_id", req.SessionId))

	// 解析会话 ID
	sessionID, err := guuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id format")
	}

	// 处理心跳
	remaining, err := s.manager.Heartbeat(ctx, sessionID)
	if err != nil {
		s.logger.Error("Failed to process heartbeat", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.HeartbeatResponse{
		Success:          true,
		ServerTimestamp:  timestamppb.Now(),
		RemainingSeconds: uint32(remaining.Seconds()),
	}, nil
}

// sessionToProto 将内部 Session 转换为 proto
func sessionToProto(sess *session.Session) (*pb.Session, error) {
	if sess == nil {
		return nil, fmt.Errorf("session is nil")
	}

	pbSession := &pb.Session{
		SessionId:    sess.SessionID.String(),
		UserId:       sess.UserID,
		ConnectionId: sess.ConnectionID.String(),
		ClientIp:     sess.ClientIP,
		ClientPort:   sess.ClientPort,
		ServerAddr:   sess.ServerAddr,
		State:        stateToProto(sess.State),
		CreatedAt:    timestamppb.New(sess.CreatedAt),
		LastActiveAt: timestamppb.New(sess.LastActiveAt),
		ExpiresAt:    timestamppb.New(sess.ExpiresAt),
		Metadata:     sess.Metadata,
	}

	// 转换统计信息
	if sess.Stats != nil {
		pbSession.Stats = &pb.SessionStats{
			PacketsSent:     sess.Stats.PacketsSent,
			PacketsReceived: sess.Stats.PacketsReceived,
			BytesSent:       sess.Stats.BytesSent,
			BytesReceived:   sess.Stats.BytesReceived,
			Retransmissions: sess.Stats.Retransmissions,
			CurrentRttMs:    sess.Stats.CurrentRTTMs,
		}
	}

	return pbSession, nil
}

// stateToProto 将内部状态转换为 proto
func stateToProto(state session.State) pb.SessionState {
	switch state {
	case session.StateConnecting:
		return pb.SessionState_SESSION_STATE_CONNECTING
	case session.StateActive:
		return pb.SessionState_SESSION_STATE_ACTIVE
	case session.StateIdle:
		return pb.SessionState_SESSION_STATE_IDLE
	case session.StateDisconnecting:
		return pb.SessionState_SESSION_STATE_DISCONNECTING
	case session.StateClosed:
		return pb.SessionState_SESSION_STATE_CLOSED
	default:
		return pb.SessionState_SESSION_STATE_UNSPECIFIED
	}
}

// protoToState 将 proto 状态转换为内部状态
func protoToState(state pb.SessionState) session.State {
	switch state {
	case pb.SessionState_SESSION_STATE_CONNECTING:
		return session.StateConnecting
	case pb.SessionState_SESSION_STATE_ACTIVE:
		return session.StateActive
	case pb.SessionState_SESSION_STATE_IDLE:
		return session.StateIdle
	case pb.SessionState_SESSION_STATE_DISCONNECTING:
		return session.StateDisconnecting
	case pb.SessionState_SESSION_STATE_CLOSED:
		return session.StateClosed
	default:
		return session.StateConnecting
	}
}
