package reliability

import (
	"testing"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/aetherflow/aetherflow/internal/quantum/protocol"
	"github.com/aetherflow/aetherflow/internal/quantum/transport"
)

func TestReceiveBufferInOrder(t *testing.T) {
	rb := NewReceiveBuffer(256)
	guid, _ := guuid.NewV7()

	// Receive packets in order
	for i := uint32(1); i <= 5; i++ {
		packet := &transport.Packet{
			Header:  protocol.NewHeader(guid, i, 0, 0),
			Payload: []byte{byte(i)},
		}

		ordered, isDup, err := rb.AddPacket(packet)
		if err != nil {
			t.Fatalf("Failed to add packet %d: %v", i, err)
		}

		if isDup {
			t.Errorf("Packet %d should not be duplicate", i)
		}

		if len(ordered) != 1 {
			t.Errorf("Expected 1 ordered packet, got %d", len(ordered))
		}
	}

	if rb.NextExpected() != 6 {
		t.Errorf("NextExpected should be 6, got %d", rb.NextExpected())
	}
}

func TestReceiveBufferOutOfOrder(t *testing.T) {
	rb := NewReceiveBuffer(256)
	guid, _ := guuid.NewV7()

	// Receive packets out of order: 1, 3, 2, 4
	sequences := []uint32{1, 3, 2, 4}

	for _, seq := range sequences {
		packet := &transport.Packet{
			Header:  protocol.NewHeader(guid, seq, 0, 0),
			Payload: []byte{byte(seq)},
		}

		ordered, isDup, err := rb.AddPacket(packet)
		if err != nil {
			t.Fatalf("Failed to add packet %d: %v", seq, err)
		}

		if isDup {
			t.Errorf("Packet %d should not be duplicate", seq)
		}

		// After receiving packet 1, should get 1 ordered packet
		// After receiving packet 3, should get 0 ordered packets (buffered)
		// After receiving packet 2, should get 2 ordered packets (2 and 3)
		// After receiving packet 4, should get 1 ordered packet
		var expectedOrdered int
		switch seq {
		case 1:
			expectedOrdered = 1
		case 3:
			expectedOrdered = 0
		case 2:
			expectedOrdered = 2 // Delivers both 2 and 3
		case 4:
			expectedOrdered = 1
		}

		if len(ordered) != expectedOrdered {
			t.Errorf("For seq %d: expected %d ordered packets, got %d", seq, expectedOrdered, len(ordered))
		}
	}

	if rb.NextExpected() != 5 {
		t.Errorf("NextExpected should be 5, got %d", rb.NextExpected())
	}
}

func TestReceiveBufferDuplicates(t *testing.T) {
	rb := NewReceiveBuffer(256)
	guid, _ := guuid.NewV7()

	packet := &transport.Packet{
		Header:  protocol.NewHeader(guid, 1, 0, 0),
		Payload: []byte{1},
	}

	// First time: should not be duplicate
	_, isDup, err := rb.AddPacket(packet)
	if err != nil {
		t.Fatalf("Failed to add packet: %v", err)
	}
	if isDup {
		t.Error("First packet should not be duplicate")
	}

	// Second time: should be duplicate
	_, isDup, err = rb.AddPacket(packet)
	if err != nil {
		t.Fatalf("Failed to add duplicate packet: %v", err)
	}
	if !isDup {
		t.Error("Second packet should be duplicate")
	}
}

func TestGenerateSACK(t *testing.T) {
	rb := NewReceiveBuffer(256)
	guid, _ := guuid.NewV7()

	// Receive packets: 1, 3, 4, 6, 7, 8
	sequences := []uint32{1, 3, 4, 6, 7, 8}
	for _, seq := range sequences {
		packet := &transport.Packet{
			Header:  protocol.NewHeader(guid, seq, 0, 0),
			Payload: []byte{byte(seq)},
		}
		rb.AddPacket(packet)
	}

	// Generate SACK
	ackNum, sackBlocks := rb.GenerateSACK()

	// Cumulative ACK should be 2 (next expected)
	if ackNum != 2 {
		t.Errorf("AckNum should be 2, got %d", ackNum)
	}

	// Should have SACK blocks for: [3-4] and [6-8]
	if len(sackBlocks) != 2 {
		t.Errorf("Expected 2 SACK blocks, got %d", len(sackBlocks))
	}

	// Verify SACK blocks
	if sackBlocks[0].Start != 3 || sackBlocks[0].End != 4 {
		t.Errorf("First SACK block should be [3-4], got [%d-%d]", sackBlocks[0].Start, sackBlocks[0].End)
	}

	if sackBlocks[1].Start != 6 || sackBlocks[1].End != 8 {
		t.Errorf("Second SACK block should be [6-8], got [%d-%d]", sackBlocks[1].Start, sackBlocks[1].End)
	}
}

func TestReceiveBufferStatistics(t *testing.T) {
	rb := NewReceiveBuffer(256)
	guid, _ := guuid.NewV7()

	// Add some packets
	sequences := []uint32{1, 3, 2, 1} // Last one is duplicate
	for _, seq := range sequences {
		packet := &transport.Packet{
			Header:  protocol.NewHeader(guid, seq, 0, 0),
			Payload: []byte{byte(seq)},
		}
		rb.AddPacket(packet)
	}

	stats := rb.Statistics()

	if stats["total_received"] != 3 {
		t.Errorf("total_received should be 3, got %d", stats["total_received"])
	}

	if stats["duplicates"] != 1 {
		t.Errorf("duplicates should be 1, got %d", stats["duplicates"])
	}

	if stats["out_of_order"] != 1 {
		t.Errorf("out_of_order should be 1, got %d", stats["out_of_order"])
	}
}
