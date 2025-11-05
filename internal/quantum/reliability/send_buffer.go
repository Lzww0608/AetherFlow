// Package reliability implements reliable delivery mechanisms for Quantum protocol
package reliability

import (
	"sync"
	"time"

	"github.com/aetherflow/aetherflow/internal/quantum/protocol"
	"github.com/aetherflow/aetherflow/internal/quantum/transport"
)

const (
	// FastRetransmitThreshold is the number of duplicate ACKs to trigger fast retransmit
	FastRetransmitThreshold = 3

	// DefaultRTO is the default retransmission timeout
	DefaultRTO = 1 * time.Second

	// MinRTO is the minimum retransmission timeout
	MinRTO = 200 * time.Millisecond

	// MaxRTO is the maximum retransmission timeout
	MaxRTO = 60 * time.Second
)

// SentPacket represents a packet that has been sent but not yet acknowledged
type SentPacket struct {
	Packet       *transport.Packet
	SeqNum       uint32
	SendTime     time.Time
	RetransCount int
	Timeout      time.Time
	Acked        bool
	DupAckCount  int // For fast retransmit
}

// SendBuffer manages sent but unacknowledged packets
type SendBuffer struct {
	mu sync.RWMutex

	// Circular buffer of sent packets
	packets map[uint32]*SentPacket

	// Send window parameters
	nextSeqNum  uint32 // Next sequence number to use
	sendBase    uint32 // Oldest unacknowledged sequence number
	sendWindow  uint32 // Maximum number of unacknowledged packets

	// RTT estimation for RTO calculation
	srtt    time.Duration // Smoothed RTT
	rttvar  time.Duration // RTT variation
	rto     time.Duration // Retransmission timeout

	// Statistics
	totalSent       uint64
	totalRetrans    uint64
	fastRetrans     uint64
	timeoutRetrans  uint64
}

// NewSendBuffer creates a new send buffer
func NewSendBuffer(windowSize uint32) *SendBuffer {
	return &SendBuffer{
		packets:    make(map[uint32]*SentPacket),
		nextSeqNum: 1, // Start from 1, 0 is reserved
		sendBase:   1,
		sendWindow: windowSize,
		rto:        DefaultRTO,
	}
}

// NextSeqNum returns the next sequence number to use
func (sb *SendBuffer) NextSeqNum() uint32 {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.nextSeqNum
}

// WindowAvailable returns the number of packets that can be sent
func (sb *SendBuffer) WindowAvailable() uint32 {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	inFlight := sb.nextSeqNum - sb.sendBase
	if inFlight >= sb.sendWindow {
		return 0
	}
	return sb.sendWindow - inFlight
}

// CanSend checks if a packet can be sent (window not full)
func (sb *SendBuffer) CanSend() bool {
	return sb.WindowAvailable() > 0
}

// AddPacket adds a packet to the send buffer
func (sb *SendBuffer) AddPacket(packet *transport.Packet) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	seqNum := sb.nextSeqNum
	
	sentPkt := &SentPacket{
		Packet:       packet,
		SeqNum:       seqNum,
		SendTime:     time.Now(),
		RetransCount: 0,
		Timeout:      time.Now().Add(sb.rto),
		Acked:        false,
		DupAckCount:  0,
	}

	sb.packets[seqNum] = sentPkt
	sb.nextSeqNum++
	sb.totalSent++

	return nil
}

// HandleACK processes an acknowledgment
func (sb *SendBuffer) HandleACK(ackNum uint32, sackBlocks []protocol.SACKBlock) []uint32 {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	var ackedSeqNums []uint32

	// Process cumulative ACK
	for seq := sb.sendBase; seq < ackNum && seq < sb.nextSeqNum; seq++ {
		if pkt, exists := sb.packets[seq]; exists && !pkt.Acked {
			pkt.Acked = true
			ackedSeqNums = append(ackedSeqNums, seq)
			
			// Update RTT estimation
			rtt := time.Since(pkt.SendTime)
			sb.updateRTO(rtt)
		}
	}

	// Process SACK blocks
	for _, block := range sackBlocks {
		for seq := block.Start; seq <= block.End && seq < sb.nextSeqNum; seq++ {
			if pkt, exists := sb.packets[seq]; exists && !pkt.Acked {
				pkt.Acked = true
				ackedSeqNums = append(ackedSeqNums, seq)
				
				// Update RTT estimation
				rtt := time.Since(pkt.SendTime)
				sb.updateRTO(rtt)
			}
		}
	}

	// Update send base to the smallest unacknowledged sequence number
	for seq := sb.sendBase; seq < sb.nextSeqNum; seq++ {
		if pkt, exists := sb.packets[seq]; exists && !pkt.Acked {
			sb.sendBase = seq
			break
		}
		// Clean up acknowledged packets
		delete(sb.packets, seq)
		sb.sendBase = seq + 1
	}

	return ackedSeqNums
}

// DetectLostPackets detects packets that should be retransmitted
// Returns packets for fast retransmit and timeout retransmit
func (sb *SendBuffer) DetectLostPackets() (fastRetrans, timeoutRetrans []*transport.Packet) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	now := time.Now()
	highestAcked := sb.findHighestAcked()

	for seq := sb.sendBase; seq < sb.nextSeqNum; seq++ {
		pkt, exists := sb.packets[seq]
		if !exists || pkt.Acked {
			continue
		}

		// Fast retransmit: packet is likely lost if packets after it have been acked
		if seq < highestAcked && (highestAcked-seq) >= FastRetransmitThreshold {
			fastRetrans = append(fastRetrans, pkt.Packet)
			pkt.RetransCount++
			pkt.SendTime = now
			pkt.Timeout = now.Add(sb.rto)
			sb.totalRetrans++
			sb.fastRetrans++
			continue
		}

		// Timeout retransmit: packet has exceeded RTO
		if now.After(pkt.Timeout) {
			timeoutRetrans = append(timeoutRetrans, pkt.Packet)
			pkt.RetransCount++
			pkt.SendTime = now
			// Exponential backoff for timeout
			pkt.Timeout = now.Add(sb.rto * time.Duration(1<<min(pkt.RetransCount, 5)))
			sb.totalRetrans++
			sb.timeoutRetrans++
		}
	}

	return fastRetrans, timeoutRetrans
}

// findHighestAcked finds the highest acknowledged sequence number
func (sb *SendBuffer) findHighestAcked() uint32 {
	highest := sb.sendBase
	for seq := sb.sendBase; seq < sb.nextSeqNum; seq++ {
		if pkt, exists := sb.packets[seq]; exists && pkt.Acked {
			highest = seq
		}
	}
	return highest
}

// updateRTO updates the retransmission timeout based on measured RTT
// Using algorithm from RFC 6298
func (sb *SendBuffer) updateRTO(rtt time.Duration) {
	if sb.srtt == 0 {
		// First RTT measurement
		sb.srtt = rtt
		sb.rttvar = rtt / 2
	} else {
		// Subsequent measurements
		alpha := 0.125
		beta := 0.25
		
		rttvarDelta := sb.srtt - rtt
		if rttvarDelta < 0 {
			rttvarDelta = -rttvarDelta
		}
		
		sb.rttvar = time.Duration(float64(sb.rttvar)*(1-beta) + float64(rttvarDelta)*beta)
		sb.srtt = time.Duration(float64(sb.srtt)*(1-alpha) + float64(rtt)*alpha)
	}

	// Calculate RTO
	sb.rto = sb.srtt + 4*sb.rttvar
	
	// Clamp RTO to valid range
	if sb.rto < MinRTO {
		sb.rto = MinRTO
	} else if sb.rto > MaxRTO {
		sb.rto = MaxRTO
	}
}

// RTO returns the current retransmission timeout
func (sb *SendBuffer) RTO() time.Duration {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.rto
}

// SRTT returns the current smoothed RTT
func (sb *SendBuffer) SRTT() time.Duration {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.srtt
}

// UpdateWindow updates the send window size
func (sb *SendBuffer) UpdateWindow(size uint32) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.sendWindow = size
}

// GetWindow returns the current send window size
func (sb *SendBuffer) GetWindow() uint32 {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.sendWindow
}

// Statistics returns send buffer statistics
func (sb *SendBuffer) Statistics() map[string]uint64 {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	
	return map[string]uint64{
		"total_sent":        sb.totalSent,
		"total_retrans":     sb.totalRetrans,
		"fast_retrans":      sb.fastRetrans,
		"timeout_retrans":   sb.timeoutRetrans,
		"in_flight":         uint64(len(sb.packets)),
		"window_size":       uint64(sb.sendWindow),
	}
}

// Reset resets the send buffer
func (sb *SendBuffer) Reset() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	
	sb.packets = make(map[uint32]*SentPacket)
	sb.nextSeqNum = 1
	sb.sendBase = 1
	sb.srtt = 0
	sb.rttvar = 0
	sb.rto = DefaultRTO
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

