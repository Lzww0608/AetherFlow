package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// Redis key prefixes
	sessionKeyPrefix    = "session:"       // session:{sessionID}
	connIndexKeyPrefix  = "conn_idx:"      // conn_idx:{connID} -> sessionID
	userIndexKeyPrefix  = "user_idx:"      // user_idx:{userID} -> Set of sessionIDs
	sessionSetKey       = "sessions:all"   // Set of all sessionIDs
	sessionCountKey     = "sessions:count" // Counter
)

// RedisStore Redis-based session store
type RedisStore struct {
	client  *redis.Client
	logger  *zap.Logger
	ttl     time.Duration // Default TTL for sessions
}

// RedisStoreConfig Redis store configuration
type RedisStoreConfig struct {
	Client *redis.Client
	Logger *zap.Logger
	TTL    time.Duration // Default TTL, 0 means use session's ExpiresAt
}

// NewRedisStore creates a new Redis-based session store
func NewRedisStore(config *RedisStoreConfig) (*RedisStore, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}

	if config.TTL == 0 {
		config.TTL = 30 * time.Minute // Default TTL
	}

	store := &RedisStore{
		client: config.Client,
		logger: config.Logger,
		ttl:    config.TTL,
	}

	return store, nil
}

// Create creates a new session in Redis
func (s *RedisStore) Create(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	// Serialize session to JSON
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Calculate TTL
	ttl := s.calculateTTL(session)

	// Redis transaction
	pipe := s.client.Pipeline()

	// 1. Store session data
	sessionKey := sessionKeyPrefix + session.SessionID.String()
	pipe.Set(ctx, sessionKey, data, ttl)

	// 2. Create connection ID index
	connIndexKey := connIndexKeyPrefix + session.ConnectionID.String()
	pipe.Set(ctx, connIndexKey, session.SessionID.String(), ttl)

	// 3. Add to user index
	userIndexKey := userIndexKeyPrefix + session.UserID
	pipe.SAdd(ctx, userIndexKey, session.SessionID.String())
	pipe.Expire(ctx, userIndexKey, ttl)

	// 4. Add to global session set
	pipe.SAdd(ctx, sessionSetKey, session.SessionID.String())

	// 5. Increment counter
	pipe.Incr(ctx, sessionCountKey)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("Failed to create session in Redis",
			zap.String("session_id", session.SessionID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to create session: %w", err)
	}

	s.logger.Debug("Session created in Redis",
		zap.String("session_id", session.SessionID.String()),
		zap.Duration("ttl", ttl))

	return nil
}

// Get retrieves a session by ID from Redis
func (s *RedisStore) Get(ctx context.Context, sessionID guuid.UUID) (*Session, error) {
	sessionKey := sessionKeyPrefix + sessionID.String()

	data, err := s.client.Get(ctx, sessionKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found: %s", sessionID.String())
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// Update updates an existing session in Redis
func (s *RedisStore) Update(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	// Check if session exists
	sessionKey := sessionKeyPrefix + session.SessionID.String()
	exists, err := s.client.Exists(ctx, sessionKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("session not found: %s", session.SessionID.String())
	}

	// Serialize session to JSON
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Calculate TTL
	ttl := s.calculateTTL(session)

	// Update session data with new TTL
	err = s.client.Set(ctx, sessionKey, data, ttl).Err()
	if err != nil {
		s.logger.Error("Failed to update session in Redis",
			zap.String("session_id", session.SessionID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Update connection index TTL
	connIndexKey := connIndexKeyPrefix + session.ConnectionID.String()
	s.client.Expire(ctx, connIndexKey, ttl)

	// Update user index TTL
	userIndexKey := userIndexKeyPrefix + session.UserID
	s.client.Expire(ctx, userIndexKey, ttl)

	s.logger.Debug("Session updated in Redis",
		zap.String("session_id", session.SessionID.String()),
		zap.Duration("ttl", ttl))

	return nil
}

// Delete deletes a session from Redis
func (s *RedisStore) Delete(ctx context.Context, sessionID guuid.UUID) error {
	// Get session first to retrieve indexes
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	// Redis transaction
	pipe := s.client.Pipeline()

	// 1. Delete session data
	sessionKey := sessionKeyPrefix + sessionID.String()
	pipe.Del(ctx, sessionKey)

	// 2. Delete connection index
	connIndexKey := connIndexKeyPrefix + session.ConnectionID.String()
	pipe.Del(ctx, connIndexKey)

	// 3. Remove from user index
	userIndexKey := userIndexKeyPrefix + session.UserID
	pipe.SRem(ctx, userIndexKey, sessionID.String())

	// 4. Remove from global session set
	pipe.SRem(ctx, sessionSetKey, sessionID.String())

	// 5. Decrement counter
	pipe.Decr(ctx, sessionCountKey)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("Failed to delete session from Redis",
			zap.String("session_id", sessionID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to delete session: %w", err)
	}

	s.logger.Debug("Session deleted from Redis",
		zap.String("session_id", sessionID.String()))

	return nil
}

// List lists sessions with optional filtering and pagination
func (s *RedisStore) List(ctx context.Context, filter *SessionFilter) ([]*Session, int, error) {
	if filter == nil {
		filter = &SessionFilter{}
	}

	var sessionIDs []string

	// Get session IDs based on filter
	if filter.UserID != nil && *filter.UserID != "" {
		// Filter by user ID
		userIndexKey := userIndexKeyPrefix + *filter.UserID
		members, err := s.client.SMembers(ctx, userIndexKey).Result()
		if err != nil && err != redis.Nil {
			return nil, 0, fmt.Errorf("failed to get user sessions: %w", err)
		}
		sessionIDs = members
	} else {
		// Get all sessions
		members, err := s.client.SMembers(ctx, sessionSetKey).Result()
		if err != nil && err != redis.Nil {
			return nil, 0, fmt.Errorf("failed to get all sessions: %w", err)
		}
		sessionIDs = members
	}

	// Fetch sessions
	var sessions []*Session
	for _, idStr := range sessionIDs {
		sessionID, err := guuid.Parse(idStr)
		if err != nil {
			s.logger.Warn("Invalid session ID in index", zap.String("id", idStr))
			continue
		}

		session, err := s.Get(ctx, sessionID)
		if err != nil {
			// Session might have expired
			s.logger.Debug("Failed to get session", zap.String("id", idStr), zap.Error(err))
			continue
		}

		// Apply filters
		if filter.State != nil && session.State != *filter.State {
			continue
		}

		sessions = append(sessions, session)
	}

	// Total count before pagination
	total := len(sessions)

	// Apply pagination
	if filter.Limit > 0 {
		start := filter.Offset
		if start > len(sessions) {
			start = len(sessions)
		}

		end := start + filter.Limit
		if end > len(sessions) {
			end = len(sessions)
		}

		sessions = sessions[start:end]
	}

	return sessions, total, nil
}

// GetByConnectionID retrieves a session by connection ID
func (s *RedisStore) GetByConnectionID(ctx context.Context, connID guuid.UUID) (*Session, error) {
	connIndexKey := connIndexKeyPrefix + connID.String()

	sessionIDStr, err := s.client.Get(ctx, connIndexKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found for connection: %s", connID.String())
		}
		return nil, fmt.Errorf("failed to get connection index: %w", err)
	}

	sessionID, err := guuid.Parse(sessionIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid session ID in index: %w", err)
	}

	return s.Get(ctx, sessionID)
}

// GetByUserID retrieves all sessions for a user
func (s *RedisStore) GetByUserID(ctx context.Context, userID string) ([]*Session, error) {
	userIndexKey := userIndexKeyPrefix + userID

	sessionIDStrs, err := s.client.SMembers(ctx, userIndexKey).Result()
	if err != nil {
		if err == redis.Nil {
			return []*Session{}, nil
		}
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	sessions := make([]*Session, 0, len(sessionIDStrs))
	for _, idStr := range sessionIDStrs {
		sessionID, err := guuid.Parse(idStr)
		if err != nil {
			s.logger.Warn("Invalid session ID in user index",
				zap.String("user_id", userID),
				zap.String("session_id", idStr))
			continue
		}

		session, err := s.Get(ctx, sessionID)
		if err != nil {
			// Session might have expired
			s.logger.Debug("Failed to get session for user",
				zap.String("user_id", userID),
				zap.String("session_id", idStr),
				zap.Error(err))
			continue
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeleteExpired deletes expired sessions (handled by Redis TTL automatically)
func (s *RedisStore) DeleteExpired(ctx context.Context) (int, error) {
	// Redis handles expiration automatically via TTL
	// This method is kept for interface compatibility
	// We can scan for sessions that have passed their ExpiresAt but still exist

	// Get all session IDs
	sessionIDs, err := s.client.SMembers(ctx, sessionSetKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get session set: %w", err)
	}

	count := 0
	now := time.Now()

	for _, idStr := range sessionIDs {
		sessionID, err := guuid.Parse(idStr)
		if err != nil {
			continue
		}

		session, err := s.Get(ctx, sessionID)
		if err != nil {
			// Already expired or deleted
			// Clean up from set
			s.client.SRem(ctx, sessionSetKey, idStr)
			count++
			continue
		}

		// Check if expired
		if session.ExpiresAt.Before(now) {
			if err := s.Delete(ctx, sessionID); err != nil {
				s.logger.Error("Failed to delete expired session",
					zap.String("session_id", idStr),
					zap.Error(err))
			} else {
				count++
			}
		}
	}

	if count > 0 {
		s.logger.Info("Cleaned up expired sessions",
			zap.Int("count", count))
	}

	return count, nil
}

// Count returns the total number of sessions
func (s *RedisStore) Count(ctx context.Context) (int, error) {
	count, err := s.client.SCard(ctx, sessionSetKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get session count: %w", err)
	}

	return int(count), nil
}

// Close closes the Redis connection
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// calculateTTL calculates the TTL for a session
func (s *RedisStore) calculateTTL(session *Session) time.Duration {
	now := time.Now()
	remaining := session.ExpiresAt.Sub(now)

	// If session has already expired, use default TTL
	if remaining <= 0 {
		return s.ttl
	}

	// Use the remaining time until expiration
	return remaining
}

// Ping checks if Redis is reachable
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Clear clears all sessions (for testing purposes)
func (s *RedisStore) Clear(ctx context.Context) error {
	s.logger.Warn("Clearing all sessions from Redis")

	// Get all session IDs
	sessionIDs, err := s.client.SMembers(ctx, sessionSetKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get session set: %w", err)
	}

	// Delete all sessions
	for _, idStr := range sessionIDs {
		sessionID, err := guuid.Parse(idStr)
		if err != nil {
			continue
		}
		s.Delete(ctx, sessionID)
	}

	// Clear global keys
	pipe := s.client.Pipeline()
	pipe.Del(ctx, sessionSetKey)
	pipe.Del(ctx, sessionCountKey)
	_, err = pipe.Exec(ctx)

	return err
}
