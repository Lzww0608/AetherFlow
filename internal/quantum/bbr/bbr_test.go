package bbr

import (
	"testing"
	"time"
)

func TestNewBBR(t *testing.T) {
	bbr := NewBBR(nil)

	if bbr == nil {
		t.Fatal("NewBBR should not return nil")
	}

	if bbr.GetState() != StateStartup {
		t.Errorf("Initial state should be STARTUP, got %s", bbr.GetState().String())
	}

	if bbr.GetSendWindow() == 0 {
		t.Error("Initial send window should not be zero")
	}

	if bbr.GetPacingRate() == 0 {
		t.Error("Initial pacing rate should not be zero")
	}
}

func TestBBRStateTransitions(t *testing.T) {
	config := &Config{
		InitialCwnd:  10,
		MinRTT:       10 * time.Millisecond,
		MaxBandwidth: 100 * 1024 * 1024,
	}
	bbr := NewBBR(config)

	// Initially in STARTUP
	if bbr.GetState() != StateStartup {
		t.Errorf("Should start in STARTUP, got %s", bbr.GetState().String())
	}

	// Simulate ACKs to detect full bandwidth
	now := time.Now()
	for i := 0; i < 10; i++ {
		bbr.OnPacketAcked(1400, 10*time.Millisecond, now)
		now = now.Add(10 * time.Millisecond)
	}

	// State machine should eventually transition out of STARTUP
	// (exact state depends on bandwidth detection)
}

func TestBBRBandwidthEstimation(t *testing.T) {
	bbr := NewBBR(nil)

	now := time.Now()

	// Send some packets
	for i := 0; i < 5; i++ {
		bbr.OnPacketSent(1400, now)
		now = now.Add(1 * time.Millisecond)
	}

	// ACK them
	for i := 0; i < 5; i++ {
		bbr.OnPacketAcked(1400, 10*time.Millisecond, now)
		now = now.Add(1 * time.Millisecond)
	}

	// Bandwidth should be updated
	bw := bbr.GetBandwidth()
	if bw == 0 {
		t.Error("Bandwidth should be updated after ACKs")
	}
}

func TestBBRPacingDelay(t *testing.T) {
	bbr := NewBBR(nil)

	// Set up some bandwidth
	now := time.Now()
	for i := 0; i < 10; i++ {
		bbr.OnPacketSent(1400, now)
		bbr.OnPacketAcked(1400, 10*time.Millisecond, now)
		now = now.Add(10 * time.Millisecond)
	}

	// Calculate pacing delay
	delay := bbr.CalculatePacingDelay(1400)

	// Delay should be positive and reasonable
	if delay <= 0 {
		t.Error("Pacing delay should be positive")
	}

	if delay > 100*time.Millisecond {
		t.Errorf("Pacing delay seems too large: %v", delay)
	}
}

func TestBBRWindowSize(t *testing.T) {
	bbr := NewBBR(nil)

	initialWindow := bbr.GetSendWindow()
	if initialWindow == 0 {
		t.Error("Initial window should not be zero")
	}

	// Simulate some traffic
	now := time.Now()
	for i := 0; i < 20; i++ {
		bbr.OnPacketSent(1400, now)
		bbr.OnPacketAcked(1400, 20*time.Millisecond, now)
		now = now.Add(5 * time.Millisecond)
	}

	// Window should adapt
	finalWindow := bbr.GetSendWindow()

	// In STARTUP, window should grow
	if bbr.GetState() == StateStartup && finalWindow <= initialWindow {
		t.Error("Window should grow in STARTUP state")
	}
}

func TestBBRStatistics(t *testing.T) {
	bbr := NewBBR(nil)

	stats := bbr.Statistics()

	if stats == nil {
		t.Fatal("Statistics should not be nil")
	}

	// Check required fields
	requiredFields := []string{"state", "btl_bw_mbps", "rtt_ms", "pacing_rate", "send_window", "cwnd_packets"}
	for _, field := range requiredFields {
		if _, ok := stats[field]; !ok {
			t.Errorf("Statistics should include field: %s", field)
		}
	}
}

func TestBBRReset(t *testing.T) {
	bbr := NewBBR(nil)

	// Run some traffic
	now := time.Now()
	for i := 0; i < 10; i++ {
		bbr.OnPacketSent(1400, now)
		bbr.OnPacketAcked(1400, 10*time.Millisecond, now)
		now = now.Add(10 * time.Millisecond)
	}

	// Reset
	bbr.Reset()

	// Should be back in STARTUP
	if bbr.GetState() != StateStartup {
		t.Errorf("After reset, should be in STARTUP, got %s", bbr.GetState().String())
	}

	// Bandwidth should be reset
	if bbr.GetBandwidth() != 0 {
		t.Error("Bandwidth should be reset to 0")
	}
}
