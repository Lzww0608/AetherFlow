// Package reliability implements reliable delivery mechanisms for Quantum protocol
package reliability

import (
	"fmt"
	"sync"

	"github.com/aetherflow/aetherflow/internal/quantum/protocol"
	"github.com/aetherflow/aetherflow/internal/quantum/transport"
)

// ReceivedPacket represents a received packet waiting to be delivered
type ReceivedPacket struct {
	Packet *transport.Packet
	SeqNum uint32
}

// ReceiveBuffer manages received packets and handles out-of-order delivery
type ReceiveBuffer struct {
	mu sync.RWMutex

	// Receive window parameters
	nextExpected uint32 // Next expected sequence number
	recvWindow   uint32 // Maximum number of out-of-order packets to buffer

	// Buffer for out-of-order packets
	packets map[uint32]*ReceivedPacket

	// Delivered packets for statistics
	totalReceived uint64
	totalOrdered  uint64
	outOfOrder    uint64
	duplicates    uint64
}

// NewReceiveBuffer creates a new receive buffer
func NewReceiveBuffer(windowSize uint32) *ReceiveBuffer {
	return &ReceiveBuffer{
		packets:      make(map[uint32]*ReceivedPacket),
		nextExpected: 1, // Start from 1, 0 is reserved
		recvWindow:   windowSize,
	}
}

// AddPacket adds a received packet to the buffer
// Returns:
// - ordered: list of packets that can now be delivered in order
// - isDuplicate: whether this packet is a duplicate
func (rb *ReceiveBuffer) AddPacket(packet *transport.Packet) (ordered []*transport.Packet, isDuplicate bool, err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	seqNum := packet.Header.SequenceNumber

	// Check if this is a duplicate
	if seqNum < rb.nextExpected {
		rb.duplicates++
		return nil, true, nil
	}

	// Check if already received
	if _, exists := rb.packets[seqNum]; exists {
		rb.duplicates++
		return nil, true, nil
	}

	// Check receive window
	if seqNum >= rb.nextExpected+rb.recvWindow {
		return nil, false, fmt.Errorf("packet outside receive window: seq=%d, expected=%d, window=%d",
			seqNum, rb.nextExpected, rb.recvWindow)
	}

	rb.totalReceived++

	// If this is the next expected packet, it can be delivered immediately
	if seqNum == rb.nextExpected {
		ordered = append(ordered, packet)
		rb.nextExpected++
		rb.totalOrdered++

		// Check if any buffered packets can now be delivered
		for {
			if pkt, exists := rb.packets[rb.nextExpected]; exists {
				ordered = append(ordered, pkt.Packet)
				delete(rb.packets, rb.nextExpected)
				rb.nextExpected++
				rb.totalOrdered++
			} else {
				break
			}
		}
	} else {
		// Out-of-order packet, buffer it
		rb.packets[seqNum] = &ReceivedPacket{
			Packet: packet,
			SeqNum: seqNum,
		}
		rb.outOfOrder++
	}

	return ordered, false, nil
}

// GenerateSACK generates SACK blocks for acknowledgment
// Returns cumulative ACK number and SACK blocks
func (rb *ReceiveBuffer) GenerateSACK() (ackNum uint32, sackBlocks []protocol.SACKBlock) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	// Cumulative ACK is the next expected sequence number
	ackNum = rb.nextExpected

	// Generate SACK blocks for out-of-order packets
	if len(rb.packets) == 0 {
		return ackNum, nil
	}

	// Find contiguous ranges of received packets
	// Sort sequence numbers
	seqNums := make([]uint32, 0, len(rb.packets))
	for seq := range rb.packets {
		seqNums = append(seqNums, seq)
	}

	// Simple bubble sort (sufficient for small number of SACK blocks)
	for i := 0; i < len(seqNums); i++ {
		for j := i + 1; j < len(seqNums); j++ {
			if seqNums[i] > seqNums[j] {
				seqNums[i], seqNums[j] = seqNums[j], seqNums[i]
			}
		}
	}

	// Build SACK blocks from contiguous ranges
	var currentBlock *protocol.SACKBlock
	for _, seq := range seqNums {
		if currentBlock == nil {
			currentBlock = &protocol.SACKBlock{Start: seq, End: seq}
		} else if seq == currentBlock.End+1 {
			// Extend current block
			currentBlock.End = seq
		} else {
			// Start new block
			sackBlocks = append(sackBlocks, *currentBlock)
			if len(sackBlocks) >= protocol.MaxSACKBlocks {
				break
			}
			currentBlock = &protocol.SACKBlock{Start: seq, End: seq}
		}
	}

	// Add last block
	if currentBlock != nil && len(sackBlocks) < protocol.MaxSACKBlocks {
		sackBlocks = append(sackBlocks, *currentBlock)
	}

	return ackNum, sackBlocks
}

// NextExpected returns the next expected sequence number
func (rb *ReceiveBuffer) NextExpected() uint32 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.nextExpected
}

// BufferedCount returns the number of buffered out-of-order packets
func (rb *ReceiveBuffer) BufferedCount() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.packets)
}

// UpdateWindow updates the receive window size
func (rb *ReceiveBuffer) UpdateWindow(size uint32) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.recvWindow = size
}

// GetWindow returns the current receive window size
func (rb *ReceiveBuffer) GetWindow() uint32 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.recvWindow
}

// Statistics returns receive buffer statistics
func (rb *ReceiveBuffer) Statistics() map[string]uint64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return map[string]uint64{
		"total_received": rb.totalReceived,
		"total_ordered":  rb.totalOrdered,
		"out_of_order":   rb.outOfOrder,
		"duplicates":     rb.duplicates,
		"buffered":       uint64(len(rb.packets)),
	}
}

// Reset resets the receive buffer
func (rb *ReceiveBuffer) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.packets = make(map[uint32]*ReceivedPacket)
	rb.nextExpected = 1
	rb.totalReceived = 0
	rb.totalOrdered = 0
	rb.outOfOrder = 0
	rb.duplicates = 0
}

// Clear removes all buffered packets
func (rb *ReceiveBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.packets = make(map[uint32]*ReceivedPacket)
}

