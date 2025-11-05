// Package quantum implements the complete Quantum protocol stack
package quantum

import (
	"context"
	"fmt"
	"sync"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/aetherflow/aetherflow/internal/quantum/bbr"
	"github.com/aetherflow/aetherflow/internal/quantum/fec"
	"github.com/aetherflow/aetherflow/internal/quantum/protocol"
	"github.com/aetherflow/aetherflow/internal/quantum/reliability"
	"github.com/aetherflow/aetherflow/internal/quantum/transport"
)

const (
	// DefaultSendWindow is the default send window size (packets)
	DefaultSendWindow = 256

	// DefaultRecvWindow is the default receive window size (packets)
	DefaultRecvWindow = 256

	// DefaultKeepaliveInterval is the default keepalive interval
	DefaultKeepaliveInterval = 10 * time.Second

	// DefaultIdleTimeout is the default idle timeout
	DefaultIdleTimeout = 60 * time.Second

	// MaxRetries is the maximum number of connection retries
	MaxRetries = 5
)

// State represents connection state
type State int

const (
	// StateInit is the initial state
	StateInit State = iota

	// StateConnecting is when connection is being established
	StateConnecting

	// StateEstablished is when connection is active
	StateEstablished

	// StateClosing is when connection is being closed
	StateClosing

	// StateClosed is when connection is closed
	StateClosed
)

func (s State) String() string {
	switch s {
	case StateInit:
		return "INIT"
	case StateConnecting:
		return "CONNECTING"
	case StateEstablished:
		return "ESTABLISHED"
	case StateClosing:
		return "CLOSING"
	case StateClosed:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

// Connection represents a Quantum protocol connection
type Connection struct {
	mu sync.RWMutex

	// Connection identity
	guid       guuid.UUID
	localAddr  string
	remoteAddr string

	// State
	state State

	// Transport layer
	conn *transport.Conn

	// Reliability layer
	sendBuf *reliability.SendBuffer
	recvBuf *reliability.ReceiveBuffer

	// Congestion control
	bbr *bbr.BBR

	// Forward error correction
	fecEnabled bool
	fecEncoder *fec.Encoder
	fecDecoder *fec.Decoder

	// Channels for data flow
	sendQueue   chan *transport.Packet
	recvQueue   chan []byte
	closeSignal chan struct{}

	// Goroutine management
	wg sync.WaitGroup

	// Configuration
	config *Config

	// Statistics
	stats Statistics
}

// Config contains configuration for Quantum connection
type Config struct {
	// Window sizes
	SendWindow uint32
	RecvWindow uint32

	// Keepalive and timeout
	KeepaliveInterval time.Duration
	IdleTimeout       time.Duration

	// FEC configuration
	FECEnabled      bool
	FECDataShards   int
	FECParityShards int

	// BBR configuration
	BBRConfig *bbr.Config

	// Transport configuration
	TransportConfig *transport.Config
}

// Statistics holds connection statistics
type Statistics struct {
	PacketsSent      uint64
	PacketsReceived  uint64
	BytesSent        uint64
	BytesReceived    uint64
	PacketsLost      uint64
	PacketsRecovered uint64
	Retransmissions  uint64
}

// DefaultConfig returns default connection configuration
func DefaultConfig() *Config {
	return &Config{
		SendWindow:        DefaultSendWindow,
		RecvWindow:        DefaultRecvWindow,
		KeepaliveInterval: DefaultKeepaliveInterval,
		IdleTimeout:       DefaultIdleTimeout,
		FECEnabled:        true,
		FECDataShards:     fec.DefaultDataShards,
		FECParityShards:   fec.DefaultParityShards,
		BBRConfig:         bbr.DefaultConfig(),
		TransportConfig:   transport.DefaultConfig(),
	}
}

// Dial creates a new connection to a remote address
func Dial(network, address string, config *Config) (*Connection, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Generate GUUID for this connection using UUIDv7 (time-ordered)
	guid, err := guuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate GUUID: %w", err)
	}

	// Create transport connection
	conn, err := transport.Dial(network, address, config.TransportConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	// Create connection
	qconn := &Connection{
		guid:        guid,
		localAddr:   conn.LocalAddr().String(),
		remoteAddr:  conn.RemoteAddr().String(),
		state:       StateInit,
		conn:        conn,
		sendBuf:     reliability.NewSendBuffer(config.SendWindow),
		recvBuf:     reliability.NewReceiveBuffer(config.RecvWindow),
		bbr:         bbr.NewBBR(config.BBRConfig),
		fecEnabled:  config.FECEnabled,
		sendQueue:   make(chan *transport.Packet, 1024),
		recvQueue:   make(chan []byte, 1024),
		closeSignal: make(chan struct{}),
		config:      config,
	}

	// Initialize FEC if enabled
	if config.FECEnabled {
		fecConfig := &fec.Config{
			DataShards:   config.FECDataShards,
			ParityShards: config.FECParityShards,
		}
		qconn.fecEncoder, err = fec.NewEncoder(fecConfig)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create FEC encoder: %w", err)
		}
		qconn.fecDecoder, err = fec.NewDecoder(fecConfig)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create FEC decoder: %w", err)
		}
	}

	// Start connection handshake
	if err := qconn.connect(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Start goroutines
	qconn.start()

	return qconn, nil
}

// Listen creates a server connection listening on an address
func Listen(network, address string, config *Config) (*Connection, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Generate GUUID for this connection using UUIDv7 (time-ordered)
	guid, err := guuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate GUUID: %w", err)
	}

	// Create transport connection
	conn, err := transport.Listen(network, address, config.TransportConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	// Create connection
	qconn := &Connection{
		guid:        guid,
		localAddr:   conn.LocalAddr().String(),
		state:       StateInit,
		conn:        conn,
		sendBuf:     reliability.NewSendBuffer(config.SendWindow),
		recvBuf:     reliability.NewReceiveBuffer(config.RecvWindow),
		bbr:         bbr.NewBBR(config.BBRConfig),
		fecEnabled:  config.FECEnabled,
		sendQueue:   make(chan *transport.Packet, 1024),
		recvQueue:   make(chan []byte, 1024),
		closeSignal: make(chan struct{}),
		config:      config,
	}

	// Initialize FEC if enabled
	if config.FECEnabled {
		fecConfig := &fec.Config{
			DataShards:   config.FECDataShards,
			ParityShards: config.FECParityShards,
		}
		qconn.fecEncoder, err = fec.NewEncoder(fecConfig)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create FEC encoder: %w", err)
		}
		qconn.fecDecoder, err = fec.NewDecoder(fecConfig)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create FEC decoder: %w", err)
		}
	}

	return qconn, nil
}

// connect performs the connection handshake
func (c *Connection) connect() error {
	c.mu.Lock()
	c.state = StateConnecting
	c.mu.Unlock()

	// Send SYN packet
	synPacket := transport.NewPacket(c.guid, 0, 0, protocol.FlagSYN, nil)
	if err := c.conn.Send(synPacket); err != nil {
		return fmt.Errorf("failed to send SYN: %w", err)
	}

	// Wait for SYN-ACK with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		packet, err := c.conn.ReceivePacket(ctx)
		if err != nil {
			return fmt.Errorf("failed to receive SYN-ACK: %w", err)
		}

		if packet.Header.HasFlag(protocol.FlagSYN) && packet.Header.HasFlag(protocol.FlagACK) {
			// Send ACK
			ackPacket := transport.NewPacket(c.guid, 1, 1, protocol.FlagACK, nil)
			if err := c.conn.Send(ackPacket); err != nil {
				return fmt.Errorf("failed to send ACK: %w", err)
			}

			c.mu.Lock()
			c.state = StateEstablished
			c.remoteAddr = packet.Addr.String()
			c.mu.Unlock()

			return nil
		}
	}
}

// start starts the connection goroutines
func (c *Connection) start() {
	// Send loop
	c.wg.Add(1)
	go c.sendLoop()

	// Receive loop
	c.wg.Add(1)
	go c.recvLoop()

	// Reliability loop (retransmission detection)
	c.wg.Add(1)
	go c.reliabilityLoop()

	// Keepalive loop
	c.wg.Add(1)
	go c.keepaliveLoop()
}

// sendLoop handles sending packets with pacing
func (c *Connection) sendLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeSignal:
			return

		case <-ticker.C:
			// Check if we can send
			if !c.sendBuf.CanSend() {
				continue
			}

			select {
			case packet := <-c.sendQueue:
				// Send packet
				if err := c.conn.Send(packet); err != nil {
					// Handle send error
					continue
				}

				// Update statistics
				c.mu.Lock()
				c.stats.PacketsSent++
				c.stats.BytesSent += uint64(len(packet.Payload))
				c.mu.Unlock()

				// Add to send buffer
				c.sendBuf.AddPacket(packet)

				// Notify BBR
				c.bbr.OnPacketSent(uint32(len(packet.Payload)), time.Now())

				// Calculate pacing delay
				delay := c.bbr.CalculatePacingDelay(uint32(len(packet.Payload)))
				if delay > 0 {
					time.Sleep(delay)
				}

			default:
				// No packet to send
			}
		}
	}
}

// recvLoop handles receiving packets
func (c *Connection) recvLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.closeSignal:
			return

		default:
			// Receive packet
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			packet, err := c.conn.ReceivePacket(ctx)
			cancel()

			if err != nil {
				continue
			}

			// Process received packet
			c.handleReceivedPacket(packet)
		}
	}
}

// handleReceivedPacket processes a received packet
func (c *Connection) handleReceivedPacket(packet *transport.Packet) {
	// Update statistics
	c.mu.Lock()
	c.stats.PacketsReceived++
	c.stats.BytesReceived += uint64(len(packet.Payload))
	c.mu.Unlock()

	// Handle ACK
	if packet.Header.HasFlag(protocol.FlagACK) {
		ackedSeqs := c.sendBuf.HandleACK(packet.Header.AckNumber, packet.Header.SACKBlocks)

		// Notify BBR of ACKs
		for range ackedSeqs {
			c.bbr.OnPacketAcked(uint32(len(packet.Payload)), c.sendBuf.SRTT(), time.Now())
		}
	}

	// Handle data
	if len(packet.Payload) > 0 {
		// Add to receive buffer
		ordered, isDuplicate, err := c.recvBuf.AddPacket(packet)
		if err != nil || isDuplicate {
			return
		}

		// Deliver ordered packets
		for _, pkt := range ordered {
			select {
			case c.recvQueue <- pkt.Payload:
			default:
				// Queue full, drop packet
			}
		}

		// Send ACK
		ackNum, sackBlocks := c.recvBuf.GenerateSACK()
		ackPacket := transport.NewPacket(c.guid, 0, ackNum, protocol.FlagACK, nil)
		for _, block := range sackBlocks {
			ackPacket.Header.AddSACKBlock(block.Start, block.End)
		}
		c.sendQueue <- ackPacket
	}
}

// reliabilityLoop handles retransmission detection
func (c *Connection) reliabilityLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeSignal:
			return

		case <-ticker.C:
			// Detect lost packets
			fastRetrans, timeoutRetrans := c.sendBuf.DetectLostPackets()

			// Retransmit lost packets
			for _, packet := range fastRetrans {
				c.sendQueue <- packet
				c.mu.Lock()
				c.stats.Retransmissions++
				c.mu.Unlock()
			}

			for _, packet := range timeoutRetrans {
				c.sendQueue <- packet
				c.mu.Lock()
				c.stats.Retransmissions++
				c.mu.Unlock()
			}
		}
	}
}

// keepaliveLoop sends periodic keepalive packets
func (c *Connection) keepaliveLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.KeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeSignal:
			return

		case <-ticker.C:
			// Send keepalive packet
			packet := transport.NewPacket(c.guid, 0, 0, 0, nil)
			select {
			case c.sendQueue <- packet:
			default:
			}
		}
	}
}

// Send sends data over the connection
func (c *Connection) Send(data []byte) error {
	c.mu.RLock()
	if c.state != StateEstablished {
		c.mu.RUnlock()
		return fmt.Errorf("connection not established")
	}
	c.mu.RUnlock()

	// Create packet
	seqNum := c.sendBuf.NextSeqNum()
	packet := transport.NewPacket(c.guid, seqNum, 0, 0, data)

	// Queue for sending
	select {
	case c.sendQueue <- packet:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send queue full")
	}
}

// Receive receives data from the connection
func (c *Connection) Receive() ([]byte, error) {
	select {
	case data := <-c.recvQueue:
		return data, nil
	case <-c.closeSignal:
		return nil, fmt.Errorf("connection closed")
	}
}

// ReceiveWithTimeout receives data with a timeout
func (c *Connection) ReceiveWithTimeout(timeout time.Duration) ([]byte, error) {
	select {
	case data := <-c.recvQueue:
		return data, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("receive timeout")
	case <-c.closeSignal:
		return nil, fmt.Errorf("connection closed")
	}
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	if c.state == StateClosed || c.state == StateClosing {
		c.mu.Unlock()
		return nil
	}
	c.state = StateClosing
	c.mu.Unlock()

	// Send FIN packet
	finPacket := transport.NewPacket(c.guid, 0, 0, protocol.FlagFIN, nil)
	c.conn.Send(finPacket)

	// Signal goroutines to stop
	close(c.closeSignal)

	// Wait for goroutines
	c.wg.Wait()

	// Close transport connection
	if err := c.conn.Close(); err != nil {
		return err
	}

	c.mu.Lock()
	c.state = StateClosed
	c.mu.Unlock()

	return nil
}

// State returns the current connection state
func (c *Connection) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// GUID returns the connection GUUID
func (c *Connection) GUID() guuid.UUID {
	return c.guid
}

// LocalAddr returns the local address
func (c *Connection) LocalAddr() string {
	return c.localAddr
}

// RemoteAddr returns the remote address
func (c *Connection) RemoteAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.remoteAddr
}

// Statistics returns connection statistics
func (c *Connection) Statistics() Statistics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// BBRStats returns BBR statistics
func (c *Connection) BBRStats() map[string]interface{} {
	return c.bbr.Statistics()
}
