// Package transport provides packet pooling for memory optimization
package transport

import (
	"sync"
)

// PacketPool manages a pool of reusable packets to reduce GC pressure
type PacketPool struct {
	pool sync.Pool
}

// NewPacketPool creates a new packet pool
func NewPacketPool() *PacketPool {
	return &PacketPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Packet{
					Payload: make([]byte, 0, 1400),
				}
			},
		},
	}
}

// Get retrieves a packet from the pool
func (p *PacketPool) Get() *Packet {
	pkt := p.pool.Get().(*Packet)
	// Reset packet fields
	pkt.Payload = pkt.Payload[:0]
	pkt.Addr = nil
	return pkt
}

// Put returns a packet to the pool
func (p *PacketPool) Put(pkt *Packet) {
	if pkt == nil {
		return
	}
	// Clear sensitive data before returning to pool
	pkt.Header = nil
	if cap(pkt.Payload) <= 2048 { // Only pool reasonably-sized buffers
		pkt.Payload = pkt.Payload[:0]
		p.pool.Put(pkt)
	}
}

// Global packet pool for convenience
var globalPacketPool = NewPacketPool()

// GetPacket gets a packet from the global pool
func GetPacket() *Packet {
	return globalPacketPool.Get()
}

// PutPacket returns a packet to the global pool
func PutPacket(pkt *Packet) {
	globalPacketPool.Put(pkt)
}

