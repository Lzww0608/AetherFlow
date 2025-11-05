// Package fec implements Forward Error Correction using Reed-Solomon encoding
package fec

import (
	"fmt"
	"sync"

	"github.com/klauspost/reedsolomon"
)

const (
	// DefaultDataShards is the default number of data shards
	DefaultDataShards = 10

	// DefaultParityShards is the default number of parity shards
	DefaultParityShards = 3

	// MaxShardSize is the maximum size of a single shard
	MaxShardSize = 1400
)

// Encoder handles FEC encoding for outgoing packets
type Encoder struct {
	mu sync.Mutex

	dataShards   int
	parityShards int
	encoder      reedsolomon.Encoder

	// Current encoding group
	currentGroup *EncodingGroup
	groupID      uint64
}

// Decoder handles FEC decoding for incoming packets
type Decoder struct {
	mu sync.RWMutex

	dataShards   int
	parityShards int
	encoder      reedsolomon.Encoder // Used for reconstruction

	// Active decoding groups
	groups map[uint64]*DecodingGroup

	// Statistics
	totalRecovered uint64
	failedRecovery uint64
}

// EncodingGroup represents a group of packets being encoded
type EncodingGroup struct {
	GroupID      uint64
	DataShards   [][]byte
	ParityShards [][]byte
	Count        int
	Complete     bool
}

// DecodingGroup represents a group of packets being decoded
type DecodingGroup struct {
	GroupID       uint64
	DataShards    [][]byte
	ParityShards  [][]byte
	ReceivedMask  []bool // Track which shards have been received
	ReceivedCount int
	Complete      bool
}

// Config contains configuration for FEC
type Config struct {
	DataShards   int
	ParityShards int
}

// DefaultConfig returns default FEC configuration
func DefaultConfig() *Config {
	return &Config{
		DataShards:   DefaultDataShards,
		ParityShards: DefaultParityShards,
	}
}

// NewEncoder creates a new FEC encoder
func NewEncoder(config *Config) (*Encoder, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.DataShards < 1 || config.DataShards > 256 {
		return nil, fmt.Errorf("invalid data shards: %d (must be 1-256)", config.DataShards)
	}

	if config.ParityShards < 0 || config.ParityShards > 256 {
		return nil, fmt.Errorf("invalid parity shards: %d (must be 0-256)", config.ParityShards)
	}

	enc, err := reedsolomon.New(config.DataShards, config.ParityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	return &Encoder{
		dataShards:   config.DataShards,
		parityShards: config.ParityShards,
		encoder:      enc,
		groupID:      1,
	}, nil
}

// AddData adds a data packet to the current encoding group
// Returns parity shards when the group is complete, or nil if more data is needed
func (e *Encoder) AddData(data []byte) (groupID uint64, parityShards [][]byte, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Initialize new group if needed
	if e.currentGroup == nil || e.currentGroup.Complete {
		e.currentGroup = &EncodingGroup{
			GroupID:    e.groupID,
			DataShards: make([][]byte, e.dataShards),
			Count:      0,
			Complete:   false,
		}
		e.groupID++
	}

	// Add data to current group
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	e.currentGroup.DataShards[e.currentGroup.Count] = dataCopy
	e.currentGroup.Count++

	// If group is complete, generate parity shards
	if e.currentGroup.Count == e.dataShards {
		if err := e.encodeGroup(); err != nil {
			return 0, nil, fmt.Errorf("failed to encode group: %w", err)
		}

		e.currentGroup.Complete = true
		return e.currentGroup.GroupID, e.currentGroup.ParityShards, nil
	}

	return 0, nil, nil
}

// encodeGroup generates parity shards for the current group
func (e *Encoder) encodeGroup() error {
	// Pad data shards to equal length
	maxLen := 0
	for _, shard := range e.currentGroup.DataShards {
		if len(shard) > maxLen {
			maxLen = len(shard)
		}
	}

	// Pad all shards to maxLen
	for i := range e.currentGroup.DataShards {
		if len(e.currentGroup.DataShards[i]) < maxLen {
			padded := make([]byte, maxLen)
			copy(padded, e.currentGroup.DataShards[i])
			e.currentGroup.DataShards[i] = padded
		}
	}

	// Create parity shards
	e.currentGroup.ParityShards = make([][]byte, e.parityShards)
	for i := range e.currentGroup.ParityShards {
		e.currentGroup.ParityShards[i] = make([]byte, maxLen)
	}

	// Combine data and parity shards for encoding
	allShards := append(e.currentGroup.DataShards, e.currentGroup.ParityShards...)

	// Encode
	if err := e.encoder.Encode(allShards); err != nil {
		return fmt.Errorf("Reed-Solomon encoding failed: %w", err)
	}

	// Extract parity shards (they were modified in place)
	e.currentGroup.ParityShards = allShards[e.dataShards:]

	return nil
}

// Reset resets the encoder state
func (e *Encoder) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentGroup = nil
}

// GetConfig returns the encoder configuration
func (e *Encoder) GetConfig() (dataShards, parityShards int) {
	return e.dataShards, e.parityShards
}

// NewDecoder creates a new FEC decoder
func NewDecoder(config *Config) (*Decoder, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.DataShards < 1 || config.DataShards > 256 {
		return nil, fmt.Errorf("invalid data shards: %d (must be 1-256)", config.DataShards)
	}

	if config.ParityShards < 0 || config.ParityShards > 256 {
		return nil, fmt.Errorf("invalid parity shards: %d (must be 0-256)", config.ParityShards)
	}

	enc, err := reedsolomon.New(config.DataShards, config.ParityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	return &Decoder{
		dataShards:   config.DataShards,
		parityShards: config.ParityShards,
		encoder:      enc,
		groups:       make(map[uint64]*DecodingGroup),
	}, nil
}

// AddShard adds a data or parity shard to a decoding group
// Returns recovered data shards if decoding is successful, or nil if more shards are needed
func (d *Decoder) AddShard(groupID uint64, shardIndex int, data []byte, isParity bool) (recovered [][]byte, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Get or create decoding group
	group, exists := d.groups[groupID]
	if !exists {
		group = &DecodingGroup{
			GroupID:       groupID,
			DataShards:    make([][]byte, d.dataShards),
			ParityShards:  make([][]byte, d.parityShards),
			ReceivedMask:  make([]bool, d.dataShards+d.parityShards),
			ReceivedCount: 0,
			Complete:      false,
		}
		d.groups[groupID] = group
	}

	// Check if already complete
	if group.Complete {
		return nil, nil
	}

	// Add shard
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	var maskIndex int
	if isParity {
		if shardIndex < 0 || shardIndex >= d.parityShards {
			return nil, fmt.Errorf("invalid parity shard index: %d", shardIndex)
		}
		group.ParityShards[shardIndex] = dataCopy
		maskIndex = d.dataShards + shardIndex
	} else {
		if shardIndex < 0 || shardIndex >= d.dataShards {
			return nil, fmt.Errorf("invalid data shard index: %d", shardIndex)
		}
		group.DataShards[shardIndex] = dataCopy
		maskIndex = shardIndex
	}

	// Mark as received
	if !group.ReceivedMask[maskIndex] {
		group.ReceivedMask[maskIndex] = true
		group.ReceivedCount++
	}

	// Try to reconstruct if we have enough shards
	if group.ReceivedCount >= d.dataShards {
		if err := d.reconstructGroup(group); err != nil {
			d.failedRecovery++
			return nil, fmt.Errorf("failed to reconstruct group: %w", err)
		}

		group.Complete = true
		d.totalRecovered += uint64(d.dataShards - group.countReceivedData())

		// Return recovered data shards
		return group.DataShards, nil
	}

	return nil, nil
}

// reconstructGroup attempts to reconstruct missing shards
func (d *Decoder) reconstructGroup(group *DecodingGroup) error {
	// Combine all shards for reconstruction
	allShards := make([][]byte, d.dataShards+d.parityShards)
	for i := 0; i < d.dataShards; i++ {
		allShards[i] = group.DataShards[i]
	}
	for i := 0; i < d.parityShards; i++ {
		allShards[d.dataShards+i] = group.ParityShards[i]
	}

	// Reconstruct missing shards
	if err := d.encoder.Reconstruct(allShards); err != nil {
		return fmt.Errorf("Reed-Solomon reconstruction failed: %w", err)
	}

	// Verify reconstruction
	ok, err := d.encoder.Verify(allShards)
	if err != nil {
		return fmt.Errorf("failed to verify reconstruction: %w", err)
	}
	if !ok {
		return fmt.Errorf("reconstruction verification failed")
	}

	// Update group with reconstructed shards
	for i := 0; i < d.dataShards; i++ {
		if group.DataShards[i] == nil {
			group.DataShards[i] = allShards[i]
		}
	}

	return nil
}

// countReceivedData counts how many data shards were received (not reconstructed)
func (group *DecodingGroup) countReceivedData() int {
	count := 0
	for i := 0; i < len(group.DataShards); i++ {
		if group.ReceivedMask[i] {
			count++
		}
	}
	return count
}

// CleanupOldGroups removes old decoding groups to prevent memory leaks
func (d *Decoder) CleanupOldGroups(keepLatest int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.groups) <= keepLatest {
		return
	}

	// Find oldest groups to remove
	groupIDs := make([]uint64, 0, len(d.groups))
	for id := range d.groups {
		groupIDs = append(groupIDs, id)
	}

	// Sort by ID (ascending)
	for i := 0; i < len(groupIDs)-1; i++ {
		for j := i + 1; j < len(groupIDs); j++ {
			if groupIDs[i] > groupIDs[j] {
				groupIDs[i], groupIDs[j] = groupIDs[j], groupIDs[i]
			}
		}
	}

	// Remove oldest groups
	toRemove := len(groupIDs) - keepLatest
	for i := 0; i < toRemove; i++ {
		delete(d.groups, groupIDs[i])
	}
}

// Statistics returns decoder statistics
func (d *Decoder) Statistics() map[string]uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]uint64{
		"total_recovered": d.totalRecovered,
		"failed_recovery": d.failedRecovery,
		"active_groups":   uint64(len(d.groups)),
	}
}

// GetConfig returns the decoder configuration
func (d *Decoder) GetConfig() (dataShards, parityShards int) {
	return d.dataShards, d.parityShards
}

// Reset resets the decoder state
func (d *Decoder) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups = make(map[uint64]*DecodingGroup)
}

// CalculateOverhead calculates the FEC overhead ratio
func CalculateOverhead(dataShards, parityShards int) float64 {
	if dataShards == 0 {
		return 0
	}
	return float64(parityShards) / float64(dataShards)
}

// CalculateRequiredShards calculates minimum shards needed for reconstruction
func CalculateRequiredShards(dataShards, parityShards int) int {
	return dataShards
}
