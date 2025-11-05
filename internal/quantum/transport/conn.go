// Package transport provides UDP-based transport layer for Quantum protocol
package transport

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/aetherflow/aetherflow/internal/quantum/protocol"
)

const (
	// DefaultReadBufferSize is the default size for UDP read buffer
	DefaultReadBufferSize = 2 * 1024 * 1024 // 2MB

	// DefaultWriteBufferSize is the default size for UDP write buffer
	DefaultWriteBufferSize = 2 * 1024 * 1024 // 2MB

	// DefaultReadTimeout is the default read timeout
	DefaultReadTimeout = 30 * time.Second
)

// Packet represents a complete Quantum protocol packet
type Packet struct {
	Header  *protocol.Header
	Payload []byte
	Addr    *net.UDPAddr // Remote address for received packets
}

// Conn represents a UDP connection for Quantum protocol
type Conn struct {
	udpConn    *net.UDPConn
	localAddr  *net.UDPAddr
	remoteAddr *net.UDPAddr
	
	// Read buffer for receiving packets
	readBuf []byte
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
	
	// Closed flag
	closed bool
	
	// Statistics
	stats Statistics
}

// Statistics holds connection statistics
type Statistics struct {
	PacketsSent     uint64
	PacketsReceived uint64
	BytesSent       uint64
	BytesReceived   uint64
	Errors          uint64
}

// Config contains configuration for transport connection
type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	ReadTimeout     time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		ReadBufferSize:  DefaultReadBufferSize,
		WriteBufferSize: DefaultWriteBufferSize,
		ReadTimeout:     DefaultReadTimeout,
	}
}

// Listen creates a new UDP connection for listening
func Listen(network, address string, config *Config) (*Conn, error) {
	if config == nil {
		config = DefaultConfig()
	}

	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP(network, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen UDP: %w", err)
	}

	// Set buffer sizes
	if err := udpConn.SetReadBuffer(config.ReadBufferSize); err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to set read buffer: %w", err)
	}

	if err := udpConn.SetWriteBuffer(config.WriteBufferSize); err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to set write buffer: %w", err)
	}

	return &Conn{
		udpConn:   udpConn,
		localAddr: addr,
		readBuf:   make([]byte, protocol.HeaderMinSize+protocol.MaxPayloadSize+protocol.MaxSACKBlocks*8),
		closed:    false,
	}, nil
}

// Dial creates a new UDP connection to a remote address
func Dial(network, address string, config *Config) (*Conn, error) {
	if config == nil {
		config = DefaultConfig()
	}

	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	udpConn, err := net.DialUDP(network, nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP: %w", err)
	}

	// Set buffer sizes
	if err := udpConn.SetReadBuffer(config.ReadBufferSize); err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to set read buffer: %w", err)
	}

	if err := udpConn.SetWriteBuffer(config.WriteBufferSize); err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to set write buffer: %w", err)
	}

	return &Conn{
		udpConn:    udpConn,
		localAddr:  udpConn.LocalAddr().(*net.UDPAddr),
		remoteAddr: addr,
		readBuf:    make([]byte, protocol.HeaderMinSize+protocol.MaxPayloadSize+protocol.MaxSACKBlocks*8),
		closed:     false,
	}, nil
}

// SendPacket sends a Quantum packet to the specified address
func (c *Conn) SendPacket(packet *Packet, addr *net.UDPAddr) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("connection closed")
	}
	c.mu.RUnlock()

	// Set payload length in header
	packet.Header.PayloadLength = uint16(len(packet.Payload))

	// Validate header
	if err := packet.Header.Validate(); err != nil {
		c.stats.Errors++
		return fmt.Errorf("invalid header: %w", err)
	}

	// Marshal header
	headerBytes, err := packet.Header.Marshal()
	if err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to marshal header: %w", err)
	}

	// Combine header and payload
	data := make([]byte, len(headerBytes)+len(packet.Payload))
	copy(data, headerBytes)
	copy(data[len(headerBytes):], packet.Payload)

	// Send to specified address or remote address
	var n int
	if addr != nil {
		n, err = c.udpConn.WriteToUDP(data, addr)
	} else if c.remoteAddr != nil {
		n, err = c.udpConn.WriteToUDP(data, c.remoteAddr)
	} else {
		return fmt.Errorf("no remote address specified")
	}

	if err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to send packet: %w", err)
	}

	// Update statistics
	c.stats.PacketsSent++
	c.stats.BytesSent += uint64(n)

	return nil
}

// Send sends a packet to the default remote address (for connected sockets)
func (c *Conn) Send(packet *Packet) error {
	return c.SendPacket(packet, nil)
}

// ReceivePacket receives a Quantum packet from the connection
func (c *Conn) ReceivePacket(ctx context.Context) (*Packet, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connection closed")
	}
	c.mu.RUnlock()

	// Set read deadline if context has deadline
	if deadline, ok := ctx.Deadline(); ok {
		if err := c.udpConn.SetReadDeadline(deadline); err != nil {
			return nil, fmt.Errorf("failed to set read deadline: %w", err)
		}
	}

	// Read from UDP connection
	n, addr, err := c.udpConn.ReadFromUDP(c.readBuf)
	if err != nil {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			c.stats.Errors++
			return nil, fmt.Errorf("failed to read packet: %w", err)
		}
	}

	// Update statistics
	c.stats.PacketsReceived++
	c.stats.BytesReceived += uint64(n)

	// Parse header
	header := &protocol.Header{}
	if err := header.Unmarshal(c.readBuf[:n]); err != nil {
		c.stats.Errors++
		return nil, fmt.Errorf("failed to unmarshal header: %w", err)
	}

	// Extract payload
	headerSize := header.Size()
	var payload []byte
	if n > headerSize {
		payload = make([]byte, n-headerSize)
		copy(payload, c.readBuf[headerSize:n])
	}

	return &Packet{
		Header:  header,
		Payload: payload,
		Addr:    addr,
	}, nil
}

// Receive receives a packet (shorthand for ReceivePacket with background context)
func (c *Conn) Receive() (*Packet, error) {
	return c.ReceivePacket(context.Background())
}

// LocalAddr returns the local address
func (c *Conn) LocalAddr() *net.UDPAddr {
	return c.localAddr
}

// RemoteAddr returns the remote address
func (c *Conn) RemoteAddr() *net.UDPAddr {
	return c.remoteAddr
}

// SetRemoteAddr sets the remote address for connected-style communication
func (c *Conn) SetRemoteAddr(addr *net.UDPAddr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remoteAddr = addr
}

// Statistics returns a copy of current statistics
func (c *Conn) Statistics() Statistics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Close closes the connection
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.udpConn.Close()
}

// IsClosed returns whether the connection is closed
func (c *Conn) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// NewPacket creates a new Quantum packet
func NewPacket(guid guuid.UUID, seqNum, ackNum uint32, flags protocol.Flags, payload []byte) *Packet {
	return &Packet{
		Header:  protocol.NewHeader(guid, seqNum, ackNum, flags),
		Payload: payload,
	}
}

