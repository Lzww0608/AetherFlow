/*
@Author: Lzww
@LastEditTime: 2025-11-9 17:14:20
@Description: Session model
@Language: Go
*/
package session

import (
	"time"

	guuid "github.com/Lzww0608/GUUID"
)

// State represents session state
type State int

const (
	// StateConnecting is when session is being established
	StateConnecting State = iota
	// StateActive is when session is active
	StateActive
	// StateIdle is when session is idle
	StateIdle
	// StateDisconnecting is when session is being closed
	StateDisconnecting
	// StateClosed is when session is closed
	StateClosed
)

// String returns string representation of state
func (s State) String() string {
	switch s {
	case StateConnecting:
		return "CONNECTING"
	case StateActive:
		return "ACTIVE"
	case StateIdle:
		return "IDLE"
	case StateDisconnecting:
		return "DISCONNECTING"
	case StateClosed:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

// Session represents a user session
type Session struct {
	// SessionID is the unique session identifier (UUIDv7)
	SessionID guuid.UUID

	// UserID is the user identifier
	UserID string

	// ConnectionID is the Quantum protocol connection GUID
	ConnectionID guuid.UUID

	// ClientIP is the client IP address
	ClientIP string

	// ClientPort is the client port
	ClientPort uint32

	// ServerAddr is the server address handling this session
	ServerAddr string

	// State is the current session state
	State State

	// CreatedAt is when the session was created
	CreatedAt time.Time

	// LastActiveAt is when the session was last active
	LastActiveAt time.Time

	// ExpiresAt is when the session will expire
	ExpiresAt time.Time

	// Metadata contains custom session metadata
	Metadata map[string]string

	// Stats contains session statistics
	Stats *Stats
}

// Stats represents session statistics
type Stats struct {
	PacketsSent     uint64
	PacketsReceived uint64
	BytesSent       uint64
	BytesReceived   uint64
	Retransmissions uint64
	CurrentRTTMs    uint32
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsActive checks if the session is active
func (s *Session) IsActive() bool {
	return s.State == StateActive && !s.IsExpired()
}

// UpdateActivity updates the last active time
func (s *Session) UpdateActivity() {
	s.LastActiveAt = time.Now()
}

// SessionFilter represents filtering criteria for sessions
type SessionFilter struct {
	UserID string
	State  *State
	Limit  int
	Offset int
}
