package protocol

import (
	"testing"

	guuid "github.com/Lzww0608/GUUID"
)

func TestHeaderMarshalUnmarshal(t *testing.T) {
	guid, _ := guuid.NewV7()

	original := NewHeader(guid, 100, 50, FlagSYN|FlagACK)
	original.PayloadLength = 1234
	original.AddSACKBlock(10, 20)
	original.AddSACKBlock(30, 40)

	// Marshal
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal header: %v", err)
	}

	// Unmarshal
	parsed := &Header{}
	if err := parsed.Unmarshal(data); err != nil {
		t.Fatalf("Failed to unmarshal header: %v", err)
	}

	// Verify fields
	if parsed.MagicNumber != original.MagicNumber {
		t.Errorf("MagicNumber mismatch: got %x, want %x", parsed.MagicNumber, original.MagicNumber)
	}

	if parsed.Version != original.Version {
		t.Errorf("Version mismatch: got %d, want %d", parsed.Version, original.Version)
	}

	if parsed.Flags != original.Flags {
		t.Errorf("Flags mismatch: got %x, want %x", parsed.Flags, original.Flags)
	}

	if parsed.GUUID != original.GUUID {
		t.Error("GUUID mismatch")
	}

	if parsed.SequenceNumber != original.SequenceNumber {
		t.Errorf("SequenceNumber mismatch: got %d, want %d", parsed.SequenceNumber, original.SequenceNumber)
	}

	if parsed.AckNumber != original.AckNumber {
		t.Errorf("AckNumber mismatch: got %d, want %d", parsed.AckNumber, original.AckNumber)
	}

	if parsed.PayloadLength != original.PayloadLength {
		t.Errorf("PayloadLength mismatch: got %d, want %d", parsed.PayloadLength, original.PayloadLength)
	}

	if len(parsed.SACKBlocks) != len(original.SACKBlocks) {
		t.Errorf("SACKBlocks length mismatch: got %d, want %d", len(parsed.SACKBlocks), len(original.SACKBlocks))
	}
}

func TestHeaderFlags(t *testing.T) {
	guid, _ := guuid.NewV7()
	header := NewHeader(guid, 0, 0, 0)

	// Test SetFlag
	header.SetFlag(FlagSYN)
	if !header.HasFlag(FlagSYN) {
		t.Error("Flag SYN should be set")
	}

	header.SetFlag(FlagACK)
	if !header.HasFlag(FlagACK) {
		t.Error("Flag ACK should be set")
	}

	// Test ClearFlag
	header.ClearFlag(FlagSYN)
	if header.HasFlag(FlagSYN) {
		t.Error("Flag SYN should be cleared")
	}

	if !header.HasFlag(FlagACK) {
		t.Error("Flag ACK should still be set")
	}
}

func TestHeaderValidate(t *testing.T) {
	guid, _ := guuid.NewV7()

	// Valid header
	header := NewHeader(guid, 100, 50, FlagACK)
	header.PayloadLength = 1000
	if err := header.Validate(); err != nil {
		t.Errorf("Valid header should pass validation: %v", err)
	}

	// Invalid: payload too large
	header.PayloadLength = MaxPayloadSize + 1
	if err := header.Validate(); err == nil {
		t.Error("Header with too large payload should fail validation")
	}

	// Invalid: zero GUUID
	header = NewHeader(guuid.Nil, 100, 50, FlagACK)
	header.PayloadLength = 1000
	if err := header.Validate(); err == nil {
		t.Error("Header with zero GUUID should fail validation")
	}
}

func TestSACKBlocks(t *testing.T) {
	guid, _ := guuid.NewV7()
	header := NewHeader(guid, 0, 0, 0)

	// Add SACK blocks
	for i := 0; i < MaxSACKBlocks; i++ {
		err := header.AddSACKBlock(uint32(i*10), uint32(i*10+5))
		if err != nil {
			t.Errorf("Failed to add SACK block %d: %v", i, err)
		}
	}

	// Try to add one more (should fail)
	err := header.AddSACKBlock(100, 105)
	if err == nil {
		t.Error("Should not be able to add more than MaxSACKBlocks")
	}
}

func TestHeaderSize(t *testing.T) {
	guid, _ := guuid.NewV7()
	header := NewHeader(guid, 0, 0, 0)

	// Without SACK blocks
	size := header.Size()
	if size != HeaderMinSize {
		t.Errorf("Header size without SACK blocks should be %d, got %d", HeaderMinSize, size)
	}

	// With SACK blocks
	header.AddSACKBlock(10, 20)
	header.AddSACKBlock(30, 40)
	size = header.Size()
	expected := HeaderMinSize + 2*8 // 2 SACK blocks * 8 bytes each
	if size != expected {
		t.Errorf("Header size with 2 SACK blocks should be %d, got %d", expected, size)
	}
}
