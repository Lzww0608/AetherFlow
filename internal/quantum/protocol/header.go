// Package protocol implements the core Quantum protocol packet format and processing
package protocol

import (
	"encoding/binary"
	"fmt"
	
	"github.com/aetherflow/aetherflow/pkg/guuid"
)

const (
	// MagicNumber identifies Quantum protocol packets
	MagicNumber uint32 = 0x51554E54 // "QUNT" in ASCII
	
	// CurrentVersion is the current protocol version
	CurrentVersion uint8 = 1
	
	// HeaderMinSize is the minimum header size without SACK blocks
	HeaderMinSize = 32
	
	// MaxSACKBlocks is the maximum number of SACK blocks allowed
	MaxSACKBlocks = 8
	
	// MaxPayloadSize is the maximum payload size per packet
	MaxPayloadSize = 1400 // Leave room for IP/UDP headers
)

// Flags represent various control flags in the packet header
type Flags uint8

const (
	FlagSYN Flags = 1 << iota // Connection establishment
	FlagACK                   // Acknowledgment
	FlagFIN                   // Connection termination
	FlagRST                   // Connection reset
	FlagFEC                   // Forward Error Correction enabled
	FlagPSH                   // Push data immediately
	FlagURG                   // Urgent data
	FlagECE                   // ECN Echo
)

// SACKBlock represents a selective acknowledgment block
type SACKBlock struct {
	Start uint32 // Starting sequence number (inclusive)
	End   uint32 // Ending sequence number (inclusive)
}

// Header represents the Quantum protocol packet header
type Header struct {
	MagicNumber    uint32      // 4 bytes - Protocol identifier
	Version        uint8       // 1 byte  - Protocol version
	Flags          Flags       // 1 byte  - Control flags
	GUUID          guuid.GUUID // 16 bytes - Global unique identifier
	SequenceNumber uint32      // 4 bytes - Packet sequence number
	AckNumber      uint32      // 4 bytes - Acknowledgment number
	SACKBlocks     []SACKBlock // Variable - Selective ACK blocks
	PayloadLength  uint16      // 2 bytes - Payload data length
}

// NewHeader creates a new Quantum protocol header
func NewHeader(guid guuid.GUUID, seqNum, ackNum uint32, flags Flags) *Header {
	return &Header{
		MagicNumber:    MagicNumber,
		Version:        CurrentVersion,
		Flags:          flags,
		GUUID:          guid,
		SequenceNumber: seqNum,
		AckNumber:      ackNum,
		SACKBlocks:     make([]SACKBlock, 0),
		PayloadLength:  0,
	}
}

// AddSACKBlock adds a SACK block to the header
func (h *Header) AddSACKBlock(start, end uint32) error {
	if len(h.SACKBlocks) >= MaxSACKBlocks {
		return fmt.Errorf("maximum SACK blocks (%d) exceeded", MaxSACKBlocks)
	}
	
	h.SACKBlocks = append(h.SACKBlocks, SACKBlock{
		Start: start,
		End:   end,
	})
	
	return nil
}

// HasFlag checks if a specific flag is set
func (h *Header) HasFlag(flag Flags) bool {
	return h.Flags&flag != 0
}

// SetFlag sets a specific flag
func (h *Header) SetFlag(flag Flags) {
	h.Flags |= flag
}

// ClearFlag clears a specific flag
func (h *Header) ClearFlag(flag Flags) {
	h.Flags &^= flag
}

// Size returns the total size of the header in bytes
func (h *Header) Size() int {
	return HeaderMinSize + len(h.SACKBlocks)*8 // Each SACK block is 8 bytes
}

// Marshal serializes the header to bytes
func (h *Header) Marshal() ([]byte, error) {
	size := h.Size()
	buf := make([]byte, size)
	
	// Magic Number (4 bytes)
	binary.BigEndian.PutUint32(buf[0:4], h.MagicNumber)
	
	// Version (1 byte)
	buf[4] = h.Version
	
	// Flags (1 byte)
	buf[5] = uint8(h.Flags)
	
	// GUUID (16 bytes)
	copy(buf[6:22], h.GUUID.Bytes())
	
	// Sequence Number (4 bytes)
	binary.BigEndian.PutUint32(buf[22:26], h.SequenceNumber)
	
	// Ack Number (4 bytes)
	binary.BigEndian.PutUint32(buf[26:30], h.AckNumber)
	
	// Payload Length (2 bytes)
	binary.BigEndian.PutUint16(buf[30:32], h.PayloadLength)
	
	// SACK Blocks (variable length)
	offset := 32
	for _, block := range h.SACKBlocks {
		binary.BigEndian.PutUint32(buf[offset:offset+4], block.Start)
		binary.BigEndian.PutUint32(buf[offset+4:offset+8], block.End)
		offset += 8
	}
	
	return buf, nil
}

// Unmarshal deserializes bytes into the header
func (h *Header) Unmarshal(data []byte) error {
	if len(data) < HeaderMinSize {
		return fmt.Errorf("packet too small: need at least %d bytes, got %d", HeaderMinSize, len(data))
	}
	
	// Magic Number
	h.MagicNumber = binary.BigEndian.Uint32(data[0:4])
	if h.MagicNumber != MagicNumber {
		return fmt.Errorf("invalid magic number: expected 0x%08X, got 0x%08X", MagicNumber, h.MagicNumber)
	}
	
	// Version
	h.Version = data[4]
	if h.Version != CurrentVersion {
		return fmt.Errorf("unsupported version: expected %d, got %d", CurrentVersion, h.Version)
	}
	
	// Flags
	h.Flags = Flags(data[5])
	
	// GUUID
	copy(h.GUUID[:], data[6:22])
	
	// Sequence Number
	h.SequenceNumber = binary.BigEndian.Uint32(data[22:26])
	
	// Ack Number
	h.AckNumber = binary.BigEndian.Uint32(data[26:30])
	
	// Payload Length
	h.PayloadLength = binary.BigEndian.Uint16(data[30:32])
	
	// Calculate number of SACK blocks
	remainingBytes := len(data) - HeaderMinSize
	if remainingBytes%8 != 0 {
		return fmt.Errorf("invalid SACK blocks: remaining bytes %d not divisible by 8", remainingBytes)
	}
	
	numSACKBlocks := remainingBytes / 8
	if numSACKBlocks > MaxSACKBlocks {
		return fmt.Errorf("too many SACK blocks: maximum %d, got %d", MaxSACKBlocks, numSACKBlocks)
	}
	
	// Parse SACK blocks
	h.SACKBlocks = make([]SACKBlock, numSACKBlocks)
	offset := 32
	for i := 0; i < numSACKBlocks; i++ {
		h.SACKBlocks[i].Start = binary.BigEndian.Uint32(data[offset : offset+4])
		h.SACKBlocks[i].End = binary.BigEndian.Uint32(data[offset+4 : offset+8])
		offset += 8
	}
	
	return nil
}

// Validate performs basic validation on the header
func (h *Header) Validate() error {
	if h.MagicNumber != MagicNumber {
		return fmt.Errorf("invalid magic number")
	}
	
	if h.Version != CurrentVersion {
		return fmt.Errorf("unsupported version")
	}
	
	if h.GUUID.IsZero() {
		return fmt.Errorf("GUUID cannot be zero")
	}
	
	if h.PayloadLength > MaxPayloadSize {
		return fmt.Errorf("payload too large: %d > %d", h.PayloadLength, MaxPayloadSize)
	}
	
	// Validate SACK blocks
	for i, block := range h.SACKBlocks {
		if block.Start > block.End {
			return fmt.Errorf("invalid SACK block %d: start %d > end %d", i, block.Start, block.End)
		}
	}
	
	return nil
}

// String returns a string representation of the header
func (h *Header) String() string {
	return fmt.Sprintf("Quantum{GUUID:%s, Seq:%d, Ack:%d, Flags:0x%02X, PayloadLen:%d, SACKBlocks:%d}",
		h.GUUID.String()[:8], h.SequenceNumber, h.AckNumber, uint8(h.Flags), h.PayloadLength, len(h.SACKBlocks))
}
