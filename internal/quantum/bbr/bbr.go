// Package bbr implements the BBR congestion control algorithm for Quantum protocol
// Based on Google's BBR algorithm: https://queue.acm.org/detail.cfm?id=3022184
package bbr

import (
	"sync"
	"time"
)

// State represents the current state of BBR
type State int

const (
	// StateStartup is the initial state where BBR aggressively probes for bandwidth
	StateStartup State = iota

	// StateDrain reduces the sending rate to drain the queue built up during startup
	StateDrain

	// StateProbeBW is the steady state where BBR probes for more bandwidth
	StateProbeBW

	// StateProbeRTT reduces inflight data to probe for minimum RTT
	StateProbeRTT
)

func (s State) String() string {
	switch s {
	case StateStartup:
		return "STARTUP"
	case StateDrain:
		return "DRAIN"
	case StateProbeBW:
		return "PROBE_BW"
	case StateProbeRTT:
		return "PROBE_RTT"
	default:
		return "UNKNOWN"
	}
}

const (
	// StartupGain is the pacing gain used during STARTUP
	StartupGain = 2.77

	// DrainGain is the pacing gain used during DRAIN
	DrainGain = 1.0 / StartupGain

	// ProbeBWGainCycle is the cycle of pacing gains during PROBE_BW
	ProbeBWCycleLen = 8

	// ProbeRTTDuration is how long to stay in PROBE_RTT
	ProbeRTTDuration = 200 * time.Millisecond

	// ProbeRTTInterval is the interval between PROBE_RTT states
	ProbeRTTInterval = 10 * time.Second

	// MinPipeCwnd is the minimum cwnd value (in packets)
	MinPipeCwnd = 4

	// HighGain is used to probe for bandwidth
	HighGain = 2.0

	// FullBandwidthThreshold is the threshold to consider bandwidth fully utilized
	// (no growth in 3 rounds)
	FullBandwidthThreshold = 1.25
)

// ProbeBW gain cycle: alternate between probing higher and lower to find equilibrium
var probeBWGainCycle = []float64{1.25, 0.75, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}

// BBR implements the BBR congestion control algorithm
type BBR struct {
	mu sync.RWMutex

	// State machine
	state        State
	stateEntryAt time.Time

	// Core BBR variables
	btlBw       uint64        // Bottleneck bandwidth (bytes/sec)
	rtProp      time.Duration // Round-trip propagation delay (minimum RTT)
	rtPropStamp time.Time     // Last time rtProp was updated

	// Pacing and windowing
	pacingRate  uint64 // Current pacing rate (bytes/sec)
	sendWindow  uint32 // Send window size (bytes)
	pacingGain  float64
	cwndGain    float64

	// PROBE_BW cycle tracking
	cycleIndex  int
	cycleStamp  time.Time
	priorCwnd   uint32

	// Bandwidth probing
	bandwidthSamples []bandwidthSample
	lastSampleTime   time.Time
	roundCount       uint64
	roundStart       bool

	// Full bandwidth detection (for STARTUP exit)
	fullBandwidthReached  bool
	fullBandwidthCount    int
	lastBandwidthReached  uint64

	// Statistics
	deliveredBytes uint64
	deliveredTime  time.Time
	
	// Configuration
	minRTT      time.Duration
	maxBandwidth uint64
}

type bandwidthSample struct {
	bandwidth uint64
	rtt       time.Duration
	timestamp time.Time
}

// Config contains configuration for BBR
type Config struct {
	InitialCwnd      uint32        // Initial congestion window (packets)
	MinRTT           time.Duration // Minimum RTT hint
	MaxBandwidth     uint64        // Maximum bandwidth hint (bytes/sec)
}

// DefaultConfig returns default BBR configuration
func DefaultConfig() *Config {
	return &Config{
		InitialCwnd:  10,
		MinRTT:       10 * time.Millisecond,
		MaxBandwidth: 100 * 1024 * 1024, // 100 MB/s
	}
}

// NewBBR creates a new BBR congestion controller
func NewBBR(config *Config) *BBR {
	if config == nil {
		config = DefaultConfig()
	}

	now := time.Now()

	bbr := &BBR{
		state:            StateStartup,
		stateEntryAt:     now,
		btlBw:            0,
		rtProp:           config.MinRTT,
		rtPropStamp:      now,
		pacingGain:       StartupGain,
		cwndGain:         StartupGain,
		cycleIndex:       0,
		cycleStamp:       now,
		bandwidthSamples: make([]bandwidthSample, 0, 10),
		lastSampleTime:   now,
		deliveredTime:    now,
		minRTT:           config.MinRTT,
		maxBandwidth:     config.MaxBandwidth,
	}

	// Initialize send window
	bbr.sendWindow = config.InitialCwnd * 1400 // Assume 1400 bytes per packet
	bbr.pacingRate = uint64(float64(bbr.sendWindow) / bbr.rtProp.Seconds())

	return bbr
}

// OnPacketSent should be called when a packet is sent
func (bbr *BBR) OnPacketSent(size uint32, now time.Time) {
	bbr.mu.Lock()
	defer bbr.mu.Unlock()

	// Update delivered bytes
	bbr.deliveredBytes += uint64(size)
}

// OnPacketAcked should be called when a packet is acknowledged
func (bbr *BBR) OnPacketAcked(size uint32, rtt time.Duration, now time.Time) {
	bbr.mu.Lock()
	defer bbr.mu.Unlock()

	// Update RTT measurements
	bbr.updateRTT(rtt, now)

	// Update bandwidth measurements
	bbr.updateBandwidth(size, rtt, now)

	// Update BBR state machine
	bbr.updateState(now)

	// Update pacing rate and congestion window
	bbr.updatePacingAndWindow()
}

// updateRTT updates the minimum RTT (rtProp)
func (bbr *BBR) updateRTT(rtt time.Duration, now time.Time) {
	// Update rtProp if this is a new minimum
	if rtt < bbr.rtProp || now.Sub(bbr.rtPropStamp) > ProbeRTTInterval {
		bbr.rtProp = rtt
		bbr.rtPropStamp = now
	}
}

// updateBandwidth updates the bandwidth estimate
func (bbr *BBR) updateBandwidth(size uint32, rtt time.Duration, now time.Time) {
	// Calculate bandwidth sample
	timeDelta := now.Sub(bbr.lastSampleTime)
	if timeDelta <= 0 {
		return
	}

	bandwidth := uint64(float64(size) / timeDelta.Seconds())

	// Add sample to history
	sample := bandwidthSample{
		bandwidth: bandwidth,
		rtt:       rtt,
		timestamp: now,
	}
	bbr.bandwidthSamples = append(bbr.bandwidthSamples, sample)

	// Keep only recent samples (last 10)
	if len(bbr.bandwidthSamples) > 10 {
		bbr.bandwidthSamples = bbr.bandwidthSamples[1:]
	}

	// Update btlBw to the maximum bandwidth seen in recent samples
	maxBw := uint64(0)
	for _, s := range bbr.bandwidthSamples {
		if s.bandwidth > maxBw {
			maxBw = s.bandwidth
		}
	}
	bbr.btlBw = maxBw

	bbr.lastSampleTime = now

	// Check for full bandwidth in STARTUP
	if bbr.state == StateStartup {
		bbr.checkFullBandwidth()
	}
}

// checkFullBandwidth checks if we've reached full bandwidth utilization
func (bbr *BBR) checkFullBandwidth() {
	// If bandwidth hasn't grown by 25% in the last 3 rounds, consider it "full"
	if bbr.btlBw >= bbr.lastBandwidthReached*uint64(FullBandwidthThreshold*100)/100 {
		bbr.lastBandwidthReached = bbr.btlBw
		bbr.fullBandwidthCount = 0
	} else {
		bbr.fullBandwidthCount++
		if bbr.fullBandwidthCount >= 3 {
			bbr.fullBandwidthReached = true
		}
	}
}

// updateState updates the BBR state machine
func (bbr *BBR) updateState(now time.Time) {
	switch bbr.state {
	case StateStartup:
		if bbr.fullBandwidthReached {
			bbr.enterDrain(now)
		}

	case StateDrain:
		// Exit DRAIN when inflight bytes <= BDP
		inflight := bbr.sendWindow
		bdp := bbr.calculateBDP()
		if inflight <= bdp {
			bbr.enterProbeBW(now)
		}

	case StateProbeBW:
		// Check if we should enter PROBE_RTT
		if now.Sub(bbr.rtPropStamp) > ProbeRTTInterval {
			bbr.enterProbeRTT(now)
		} else {
			// Cycle through pacing gains
			bbr.updateProbeBWCycle(now)
		}

	case StateProbeRTT:
		// Exit PROBE_RTT after the duration has elapsed
		if now.Sub(bbr.stateEntryAt) >= ProbeRTTDuration {
			bbr.enterProbeBW(now)
		}
	}
}

// enterDrain transitions to DRAIN state
func (bbr *BBR) enterDrain(now time.Time) {
	bbr.state = StateDrain
	bbr.stateEntryAt = now
	bbr.pacingGain = DrainGain
	bbr.cwndGain = 2.0 // Higher cwnd gain to drain queue quickly
}

// enterProbeBW transitions to PROBE_BW state
func (bbr *BBR) enterProbeBW(now time.Time) {
	bbr.state = StateProbeBW
	bbr.stateEntryAt = now
	bbr.cycleIndex = 0
	bbr.cycleStamp = now
	bbr.pacingGain = probeBWGainCycle[0]
	bbr.cwndGain = 2.0
}

// enterProbeRTT transitions to PROBE_RTT state
func (bbr *BBR) enterProbeRTT(now time.Time) {
	bbr.state = StateProbeRTT
	bbr.stateEntryAt = now
	bbr.pacingGain = 1.0
	bbr.cwndGain = 1.0
	bbr.priorCwnd = bbr.sendWindow
}

// updateProbeBWCycle updates the PROBE_BW gain cycle
func (bbr *BBR) updateProbeBWCycle(now time.Time) {
	// Advance cycle every RTT
	if now.Sub(bbr.cycleStamp) > bbr.rtProp {
		bbr.cycleIndex = (bbr.cycleIndex + 1) % ProbeBWCycleLen
		bbr.cycleStamp = now
		bbr.pacingGain = probeBWGainCycle[bbr.cycleIndex]
	}
}

// updatePacingAndWindow updates pacing rate and congestion window
func (bbr *BBR) updatePacingAndWindow() {
	// Calculate pacing rate
	if bbr.btlBw > 0 {
		bbr.pacingRate = uint64(float64(bbr.btlBw) * bbr.pacingGain)
	}

	// Calculate congestion window (in bytes)
	bdp := bbr.calculateBDP()
	cwnd := uint32(float64(bdp) * bbr.cwndGain)

	// Enforce minimum window
	minCwnd := uint32(MinPipeCwnd * 1400) // 4 packets * 1400 bytes
	if cwnd < minCwnd {
		cwnd = minCwnd
	}

	bbr.sendWindow = cwnd
}

// calculateBDP calculates the bandwidth-delay product
func (bbr *BBR) calculateBDP() uint32 {
	if bbr.btlBw == 0 || bbr.rtProp == 0 {
		return MinPipeCwnd * 1400
	}
	bdp := uint64(float64(bbr.btlBw) * bbr.rtProp.Seconds())
	return uint32(bdp)
}

// GetPacingRate returns the current pacing rate (bytes/sec)
func (bbr *BBR) GetPacingRate() uint64 {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()
	return bbr.pacingRate
}

// GetSendWindow returns the current send window (bytes)
func (bbr *BBR) GetSendWindow() uint32 {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()
	return bbr.sendWindow
}

// GetCwnd returns the current congestion window (packets)
func (bbr *BBR) GetCwnd() uint32 {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()
	return bbr.sendWindow / 1400
}

// GetState returns the current BBR state
func (bbr *BBR) GetState() State {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()
	return bbr.state
}

// GetBandwidth returns the estimated bottleneck bandwidth (bytes/sec)
func (bbr *BBR) GetBandwidth() uint64 {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()
	return bbr.btlBw
}

// GetRTT returns the minimum RTT
func (bbr *BBR) GetRTT() time.Duration {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()
	return bbr.rtProp
}

// OnPacketLost should be called when a packet is lost
func (bbr *BBR) OnPacketLost(size uint32, now time.Time) {
	bbr.mu.Lock()
	defer bbr.mu.Unlock()

	// BBR doesn't reduce cwnd on loss, but we track it for statistics
	// The loss is already factored into bandwidth estimation
}

// CalculatePacingDelay calculates the delay between sending packets
func (bbr *BBR) CalculatePacingDelay(packetSize uint32) time.Duration {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()

	if bbr.pacingRate == 0 {
		return 0
	}

	// Time to send this packet = packet_size / pacing_rate
	delay := time.Duration(float64(packetSize) / float64(bbr.pacingRate) * float64(time.Second))
	return delay
}

// Reset resets the BBR controller to initial state
func (bbr *BBR) Reset() {
	bbr.mu.Lock()
	defer bbr.mu.Unlock()

	now := time.Now()
	bbr.state = StateStartup
	bbr.stateEntryAt = now
	bbr.btlBw = 0
	bbr.rtProp = bbr.minRTT
	bbr.rtPropStamp = now
	bbr.pacingGain = StartupGain
	bbr.cwndGain = StartupGain
	bbr.cycleIndex = 0
	bbr.fullBandwidthReached = false
	bbr.fullBandwidthCount = 0
	bbr.lastBandwidthReached = 0
	bbr.bandwidthSamples = bbr.bandwidthSamples[:0]
}

// Statistics returns BBR statistics
func (bbr *BBR) Statistics() map[string]interface{} {
	bbr.mu.RLock()
	defer bbr.mu.RUnlock()

	return map[string]interface{}{
		"state":         bbr.state.String(),
		"btl_bw_mbps":   float64(bbr.btlBw) / 1024 / 1024,
		"rtt_ms":        float64(bbr.rtProp.Microseconds()) / 1000,
		"pacing_rate":   bbr.pacingRate,
		"send_window":   bbr.sendWindow,
		"cwnd_packets":  bbr.sendWindow / 1400,
		"pacing_gain":   bbr.pacingGain,
		"cwnd_gain":     bbr.cwndGain,
	}
}

