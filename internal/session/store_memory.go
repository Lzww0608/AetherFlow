/*
@Author: Lzww
@LastEditTime: 2025-11-10 22:03:20
@Description: Session store memory implementation
@Language: Go
*/
package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
)

// MemoryStore is an in-memory implementation of Store
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[guuid.UUID]*Session
	connIdx  map[guuid.UUID]guuid.UUID // connectionID -> sessionID mapping
	userIdx  map[string][]guuid.UUID   // userID -> sessionIDs mapping
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[guuid.UUID]*Session),
		connIdx:  make(map[guuid.UUID]guuid.UUID),
		userIdx:  make(map[string][]guuid.UUID),
	}
}

// Create creates a new session
func (s *MemoryStore) Create(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if session already exists
	if _, exists := s.sessions[session.SessionID]; exists {
		return fmt.Errorf("session already exists: %s", session.SessionID.String())
	}

	// Store session
	s.sessions[session.SessionID] = session

	// Update connection index
	s.connIdx[session.ConnectionID] = session.SessionID

	// Update user index
	s.userIdx[session.UserID] = append(s.userIdx[session.UserID], session.SessionID)

	return nil
}

// Get retrieves a session by ID
func (s *MemoryStore) Get(ctx context.Context, sessionID guuid.UUID) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID.String())
	}

	return session, nil
}

// Update updates an existing session
func (s *MemoryStore) Update(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if session exists
	if _, exists := s.sessions[session.SessionID]; !exists {
		return fmt.Errorf("session not found: %s", session.SessionID.String())
	}

	// Update session
	s.sessions[session.SessionID] = session

	return nil
}

// Delete deletes a session
func (s *MemoryStore) Delete(ctx context.Context, sessionID guuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID.String())
	}

	// Remove from main map
	delete(s.sessions, sessionID)

	// Remove from connection index
	delete(s.connIdx, session.ConnectionID)

	// Remove from user index
	if userSessions, exists := s.userIdx[session.UserID]; exists {
		newSessions := make([]guuid.UUID, 0, len(userSessions))
		for _, sid := range userSessions {
			if sid != sessionID {
				newSessions = append(newSessions, sid)
			}
		}
		if len(newSessions) > 0 {
			s.userIdx[session.UserID] = newSessions
		} else {
			delete(s.userIdx, session.UserID)
		}
	}

	return nil
}

// List lists sessions based on filter criteria
func (s *MemoryStore) List(ctx context.Context, filter *SessionFilter) ([]*Session, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Session

	// Collect sessions
	for _, session := range s.sessions {
		// Apply filters
		if filter.UserID != "" && session.UserID != filter.UserID {
			continue
		}
		if filter.State != nil && session.State != *filter.State {
			continue
		}
		result = append(result, session)
	}

	total := len(result)

	// Apply pagination
	if filter.Limit > 0 {
		start := filter.Offset
		end := start + filter.Limit

		if start > len(result) {
			return []*Session{}, total, nil
		}
		if end > len(result) {
			end = len(result)
		}

		result = result[start:end]
	}

	return result, total, nil
}

// GetByConnectionID retrieves a session by connection ID
func (s *MemoryStore) GetByConnectionID(ctx context.Context, connID guuid.UUID) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionID, exists := s.connIdx[connID]
	if !exists {
		return nil, fmt.Errorf("session not found for connection: %s", connID.String())
	}

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID.String())
	}

	return session, nil
}

// GetByUserID retrieves all sessions for a user
func (s *MemoryStore) GetByUserID(ctx context.Context, userID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs, exists := s.userIdx[userID]
	if !exists {
		return []*Session{}, nil
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sid := range sessionIDs {
		if session, exists := s.sessions[sid]; exists {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// DeleteExpired deletes all expired sessions
func (s *MemoryStore) DeleteExpired(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expired := make([]guuid.UUID, 0)

	// Find expired sessions
	for sid, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			expired = append(expired, sid)
		}
	}

	// Delete expired sessions
	for _, sid := range expired {
		if session, exists := s.sessions[sid]; exists {
			// Remove from main map
			delete(s.sessions, sid)

			// Remove from connection index
			delete(s.connIdx, session.ConnectionID)

			// Remove from user index
			if userSessions, exists := s.userIdx[session.UserID]; exists {
				newSessions := make([]guuid.UUID, 0, len(userSessions))
				for _, usid := range userSessions {
					if usid != sid {
						newSessions = append(newSessions, usid)
					}
				}
				if len(newSessions) > 0 {
					s.userIdx[session.UserID] = newSessions
				} else {
					delete(s.userIdx, session.UserID)
				}
			}
		}
	}

	return len(expired), nil
}

// Count returns the total number of sessions
func (s *MemoryStore) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions), nil
}
