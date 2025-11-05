// Package main demonstrates a simple Quantum protocol server
package main

import (
	"fmt"
	"log"

	"github.com/aetherflow/aetherflow/internal/quantum"
)

func main() {
	// Create server configuration
	config := quantum.DefaultConfig()
	config.FECEnabled = true

	// Listen for connections
	fmt.Println("Starting Quantum server on :9090...")
	conn, err := quantum.Listen("udp", ":9090", config)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Server started! GUID: %s\n", conn.GUID().String())
	fmt.Printf("Listening on: %s\n", conn.LocalAddr())

	// Handle incoming messages
	messageCount := 0
	for {
		// Receive data
		data, err := conn.Receive()
		if err != nil {
			log.Printf("Failed to receive: %v", err)
			break
		}

		messageCount++
		fmt.Printf("Received message %d: %s\n", messageCount, string(data))

		// Echo response
		response := fmt.Sprintf("Echo %d: %s", messageCount, string(data))
		if err := conn.Send([]byte(response)); err != nil {
			log.Printf("Failed to send response: %v", err)
			continue
		}

		// Print statistics every 10 messages
		if messageCount%10 == 0 {
			stats := conn.Statistics()
			fmt.Printf("\n=== Statistics (after %d messages) ===\n", messageCount)
			fmt.Printf("Packets Sent: %d, Received: %d\n", stats.PacketsSent, stats.PacketsReceived)
			fmt.Printf("Retransmissions: %d\n", stats.Retransmissions)

			bbrStats := conn.BBRStats()
			fmt.Printf("BBR State: %s, RTT: %.2f ms\n", bbrStats["state"], bbrStats["rtt_ms"])
			fmt.Println()
		}
	}

	fmt.Println("\nServer shutting down...")
}
