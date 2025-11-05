package fec

import (
	"bytes"
	"testing"
)

func TestEncoderDecoder(t *testing.T) {
	config := &Config{
		DataShards:   4,
		ParityShards: 2,
	}

	encoder, err := NewEncoder(config)
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	decoder, err := NewDecoder(config)
	if err != nil {
		t.Fatalf("Failed to create decoder: %v", err)
	}

	// Prepare test data
	testData := [][]byte{
		[]byte("packet1"),
		[]byte("packet2"),
		[]byte("packet3"),
		[]byte("packet4"),
	}

	// Encode data
	var groupID uint64
	var parityShards [][]byte
	for _, data := range testData {
		gid, parity, err := encoder.AddData(data)
		if err != nil {
			t.Fatalf("Failed to add data: %v", err)
		}
		if parity != nil {
			groupID = gid
			parityShards = parity
		}
	}

	if parityShards == nil {
		t.Fatal("Should have generated parity shards")
	}

	if len(parityShards) != config.ParityShards {
		t.Errorf("Expected %d parity shards, got %d", config.ParityShards, len(parityShards))
	}

	t.Logf("Generated group %d with %d parity shards", groupID, len(parityShards))

	// Simulate packet loss: lose packet 1 and packet 3
	// Send packets 0, 2 and all parity shards to decoder
	decoder.AddShard(groupID, 0, testData[0], false)
	decoder.AddShard(groupID, 2, testData[2], false)

	// Add parity shards
	var recovered [][]byte
	for i, parity := range parityShards {
		rec, err := decoder.AddShard(groupID, i, parity, true)
		if err != nil {
			t.Fatalf("Failed to add parity shard: %v", err)
		}
		if rec != nil {
			recovered = rec
		}
	}

	if recovered == nil {
		t.Fatal("Should have recovered data")
	}

	if len(recovered) != config.DataShards {
		t.Errorf("Expected %d recovered shards, got %d", config.DataShards, len(recovered))
	}

	// Verify recovered data (accounting for padding)
	for i, original := range testData {
		if !bytes.HasPrefix(recovered[i], original) {
			t.Errorf("Recovered data %d does not match original", i)
		}
	}
}

func TestEncoderSingleGroup(t *testing.T) {
	encoder, err := NewEncoder(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	dataCount := DefaultDataShards
	for i := 0; i < dataCount-1; i++ {
		gid, parity, err := encoder.AddData([]byte("test data"))
		if err != nil {
			t.Fatalf("Failed to add data %d: %v", i, err)
		}
		if parity != nil {
			t.Errorf("Should not generate parity until group is complete (at %d)", i)
		}
		if gid != 0 {
			t.Errorf("Should return 0 group ID until complete (at %d)", i)
		}
	}

	// Add last data packet
	gid, parity, err := encoder.AddData([]byte("test data"))
	if err != nil {
		t.Fatalf("Failed to add last data: %v", err)
	}

	if parity == nil {
		t.Error("Should generate parity when group is complete")
	}

	if gid == 0 {
		t.Error("Should return non-zero group ID when complete")
	}

	if len(parity) != DefaultParityShards {
		t.Errorf("Expected %d parity shards, got %d", DefaultParityShards, len(parity))
	}
}

func TestDecoderCleanup(t *testing.T) {
	decoder, err := NewDecoder(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create decoder: %v", err)
	}

	// Add shards for multiple groups
	for groupID := uint64(1); groupID <= 10; groupID++ {
		decoder.AddShard(groupID, 0, []byte("test"), false)
	}

	stats := decoder.Statistics()
	if stats["active_groups"] != 10 {
		t.Errorf("Expected 10 active groups, got %d", stats["active_groups"])
	}

	// Cleanup old groups, keep only 5 latest
	decoder.CleanupOldGroups(5)

	stats = decoder.Statistics()
	if stats["active_groups"] != 5 {
		t.Errorf("After cleanup, expected 5 active groups, got %d", stats["active_groups"])
	}
}

func TestCalculateOverhead(t *testing.T) {
	tests := []struct {
		data   int
		parity int
		want   float64
	}{
		{10, 3, 0.3},
		{4, 2, 0.5},
		{10, 0, 0.0},
	}

	for _, tt := range tests {
		got := CalculateOverhead(tt.data, tt.parity)
		if got != tt.want {
			t.Errorf("CalculateOverhead(%d, %d) = %f, want %f", tt.data, tt.parity, got, tt.want)
		}
	}
}

func TestCalculateRequiredShards(t *testing.T) {
	tests := []struct {
		data   int
		parity int
		want   int
	}{
		{10, 3, 10},
		{4, 2, 4},
		{5, 5, 5},
	}

	for _, tt := range tests {
		got := CalculateRequiredShards(tt.data, tt.parity)
		if got != tt.want {
			t.Errorf("CalculateRequiredShards(%d, %d) = %d, want %d", tt.data, tt.parity, got, tt.want)
		}
	}
}

func TestInvalidConfig(t *testing.T) {
	// Invalid data shards
	_, err := NewEncoder(&Config{DataShards: 0, ParityShards: 2})
	if err == nil {
		t.Error("Should reject 0 data shards")
	}

	_, err = NewEncoder(&Config{DataShards: 300, ParityShards: 2})
	if err == nil {
		t.Error("Should reject too many data shards")
	}

	// Invalid parity shards
	_, err = NewEncoder(&Config{DataShards: 10, ParityShards: -1})
	if err == nil {
		t.Error("Should reject negative parity shards")
	}
}
