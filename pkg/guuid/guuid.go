// Package guuid provides a Go-native Unique Universal Identifier implementation
// optimized for the Quantum protocol's connection tracking and distributed tracing needs.
package guuid

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

// GUUID represents a 16-byte globally unique identifier
// It serves dual purposes:
// 1. Connection ID for UDP packet demultiplexing
// 2. Trace ID for distributed request tracking
type GUUID [16]byte

// New generates a new GUUID using crypto/rand for high entropy
func New() (GUUID, error) {
	var g GUUID
	_, err := rand.Read(g[:])
	if err != nil {
		return GUUID{}, fmt.Errorf("failed to generate GUUID: %w", err)
	}
	return g, nil
}

// NewWithTimestamp generates a GUUID with embedded timestamp for ordering
// First 8 bytes: Unix timestamp (nanoseconds)
// Last 8 bytes: Random data
func NewWithTimestamp() (GUUID, error) {
	var g GUUID
	
	// Embed current timestamp in first 8 bytes
	timestamp := time.Now().UnixNano()
	binary.BigEndian.PutUint64(g[:8], uint64(timestamp))
	
	// Fill remaining 8 bytes with random data
	_, err := rand.Read(g[8:])
	if err != nil {
		return GUUID{}, fmt.Errorf("failed to generate timestamped GUUID: %w", err)
	}
	
	return g, nil
}

// FromString parses a GUUID from its string representation
func FromString(s string) (GUUID, error) {
	// Remove hyphens if present (UUID format compatibility)
	cleaned := ""
	for _, r := range s {
		if r != '-' {
			cleaned += string(r)
		}
	}
	
	if len(cleaned) != 32 {
		return GUUID{}, fmt.Errorf("invalid GUUID string length: expected 32 hex chars, got %d", len(cleaned))
	}
	
	bytes, err := hex.DecodeString(cleaned)
	if err != nil {
		return GUUID{}, fmt.Errorf("invalid GUUID string format: %w", err)
	}
	
	var g GUUID
	copy(g[:], bytes)
	return g, nil
}

// String returns the string representation of the GUUID
func (g GUUID) String() string {
	return hex.EncodeToString(g[:])
}

// StringWithHyphens returns UUID-compatible string format
func (g GUUID) StringWithHyphens() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		g[0:4], g[4:6], g[6:8], g[8:10], g[10:16])
}

// Bytes returns the raw byte slice
func (g GUUID) Bytes() []byte {
	return g[:]
}

// IsZero checks if the GUUID is zero-valued
func (g GUUID) IsZero() bool {
	for _, b := range g {
		if b != 0 {
			return false
		}
	}
	return true
}

// Timestamp extracts the timestamp from a timestamped GUUID
// Returns zero time if not a timestamped GUUID
func (g GUUID) Timestamp() time.Time {
	timestamp := binary.BigEndian.Uint64(g[:8])
	return time.Unix(0, int64(timestamp))
}

// Equal compares two GUUIDs for equality
func (g GUUID) Equal(other GUUID) bool {
	for i := 0; i < 16; i++ {
		if g[i] != other[i] {
			return false
		}
	}
	return true
}

// MarshalText implements encoding.TextMarshaler
func (g GUUID) MarshalText() ([]byte, error) {
	return []byte(g.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (g *GUUID) UnmarshalText(text []byte) error {
	parsed, err := FromString(string(text))
	if err != nil {
		return err
	}
	*g = parsed
	return nil
}

// Zero returns a zero-valued GUUID
func Zero() GUUID {
	return GUUID{}
}
