// Package main demonstrates a simple Quantum protocol client
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aetherflow/aetherflow/internal/quantum"
)

func main() {
	// Create connection configuration
	config := quantum.DefaultConfig()
	config.FECEnabled = true
	config.FECDataShards = 10
	config.FECParityShards = 3

	// Connect to server
	fmt.Println("Connecting to server...")
	conn, err := quantum.Dial("udp", "localhost:9090", config)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Connected! GUID: %s\n", conn.GUID().String())
	fmt.Printf("Local: %s, Remote: %s\n", conn.LocalAddr(), conn.RemoteAddr())

	// Send data
	messages := []string{
		"Hello, Quantum!",
		"This is a test message.",
		"Quantum protocol is awesome!",
	}

	for i, msg := range messages {
		fmt.Printf("Sending message %d: %s\n", i+1, msg)
		if err := conn.Send([]byte(msg)); err != nil {
			log.Printf("Failed to send message %d: %v", i+1, err)
			continue
		}

		// Wait for response
		data, err := conn.ReceiveWithTimeout(5 * time.Second)
		if err != nil {
			log.Printf("Failed to receive response: %v", err)
			continue
		}

		fmt.Printf("Received response: %s\n", string(data))
	}

	// Print statistics
	fmt.Println("\n=== Connection Statistics ===")
	stats := conn.Statistics()
	fmt.Printf("Packets Sent: %d\n", stats.PacketsSent)
	fmt.Printf("Packets Received: %d\n", stats.PacketsReceived)
	fmt.Printf("Bytes Sent: %d\n", stats.BytesSent)
	fmt.Printf("Bytes Received: %d\n", stats.BytesReceived)
	fmt.Printf("Retransmissions: %d\n", stats.Retransmissions)

	fmt.Println("\n=== BBR Statistics ===")
	bbrStats := conn.BBRStats()
	fmt.Printf("State: %s\n", bbrStats["state"])
	fmt.Printf("Bandwidth: %.2f Mbps\n", bbrStats["btl_bw_mbps"])
	fmt.Printf("RTT: %.2f ms\n", bbrStats["rtt_ms"])
	fmt.Printf("Congestion Window: %d packets\n", bbrStats["cwnd_packets"])

	fmt.Println("\nClosing connection...")
}
