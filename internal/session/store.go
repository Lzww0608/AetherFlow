/*
@Author: Lzww
@LastEditTime: 2025-11-10 21:59:43
@Description: Session store
@Language: Go

┌──────────────────────────────────────┐
│         Business Layer               │
│      (SessionManager)                │
│  - CreateSession()                   │
│  - Heartbeat()                       │
│  - etc.                              │
└──────────────┬───────────────────────┘

	|
	│ 使用Store接口
	│
	|

┌──────────────▼───────────────────────┐
│      Repository Interface            │
│         (Store)                      │
│  - Create()                          │
│  - Get()                             │
│  - Update()                          │
│  - Delete()                          │
└──────────────┬───────────────────────┘

	           │ 具体实现
	           │
	┌──────────┴──────────┐
	│                     │

┌───▼─────┐       ┌───────▼────┐
│ Memory  │       │    etcd    │
│ Store   │       │   Store    │
└─────────┘       └────────────┘
*/
package session

import (
	"context"

	guuid "github.com/Lzww0608/GUUID"
)

// Store defines the interface for session storage
type Store interface {
	// Create creates a new session
	Create(ctx context.Context, session *Session) error

	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID guuid.UUID) (*Session, error)

	// Update updates an existing session
	Update(ctx context.Context, session *Session) error

	// Delete deletes a session
	Delete(ctx context.Context, sessionID guuid.UUID) error

	// List lists sessions based on filter criteria
	List(ctx context.Context, filter *SessionFilter) ([]*Session, int, error)

	// GetByConnectionID retrieves a session by connection ID
	GetByConnectionID(ctx context.Context, connID guuid.UUID) (*Session, error)

	// GetByUserID retrieves all sessions for a user
	GetByUserID(ctx context.Context, userID string) ([]*Session, error)

	// DeleteExpired deletes all expired sessions
	DeleteExpired(ctx context.Context) (int, error)

	// Count returns the total number of sessions
	Count(ctx context.Context) (int, error)
}
