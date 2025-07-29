package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
	// Set up logging
	log := waLog.Stdout("Main", "DEBUG", true)

	// Set up database
	ctx := context.Background()
	container, err := sqlstore.New(ctx, "sqlite3", "file:presence_example.db?_foreign_keys=on", log)
	if err != nil {
		panic(err)
	}

	// Get device store
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}

	// Create client
	client := whatsmeow.NewClient(deviceStore, log)

	// Configure for presence-optimized usage
	presenceConfig := &whatsmeow.PresenceOptimizedConfig{
		Enabled:                     true,
		KeepAliveInterval:           60 * time.Second, // Longer keepalive for presence-only
		ReconnectDelay:              3 * time.Second,  // Fast reconnect
		IgnoreAbnormalClosureErrors: true,             // Don't log 1006 errors
		HandshakeTimeout:            30 * time.Second,
		ReadBufferSize:              4096,
		WriteBufferSize:             4096,
	}

	// Set up presence event handler
	client.HandlePresenceEvents(func(presence *events.Presence) {
		status := "available"
		if presence.Unavailable {
			status = "unavailable"
		}

		lastSeenStr := ""
		if !presence.LastSeen.IsZero() {
			lastSeenStr = fmt.Sprintf(" (last seen: %v)", presence.LastSeen.Format("2006-01-02 15:04:05"))
		}

		fmt.Printf("Presence update: %s is %s%s\n", presence.From, status, lastSeenStr)
	})

	// Set up connection event handlers
	client.AddEventHandler(func(evt any) {
		switch v := evt.(type) {
		case *events.Connected:
			fmt.Println("✅ Connected to WhatsApp")

			// Set ourselves as available
			if err := client.SendPresence(types.PresenceAvailable); err != nil {
				fmt.Printf("❌ Failed to set presence: %v\n", err)
			} else {
				fmt.Println("✅ Set presence to available")
			}

		case *events.Disconnected:
			fmt.Println("❌ Disconnected from WhatsApp")

		case *events.KeepAliveTimeout:
			fmt.Printf("⚠️  Keepalive timeout (error count: %d)\n", v.ErrorCount)

		case *events.KeepAliveRestored:
			fmt.Println("✅ Keepalive restored")

		case *events.QR:
			fmt.Println("QR Code:")
			if len(v.Codes) > 0 {
				fmt.Println(v.Codes[0])
			}

		case *events.PairSuccess:
			fmt.Println("✅ Paired successfully")
		}
	})

	// Connect with presence optimization
	fmt.Println("🔄 Connecting to WhatsApp with presence optimization...")
	if err := client.ConnectForPresence(presenceConfig); err != nil {
		panic(fmt.Sprintf("Failed to connect: %v", err))
	}

	// Wait for connection
	if !client.WaitForConnection(30 * time.Second) {
		panic("Failed to connect within 30 seconds")
	}

	// Example: Subscribe to presence of specific contacts
	// Replace with actual JIDs you want to monitor
	contactsToMonitor := []string{
		// "1234567890@s.whatsapp.net", // Replace with actual phone numbers
	}

	for _, contact := range contactsToMonitor {
		jid, err := types.ParseJID(contact)
		if err != nil {
			fmt.Printf("❌ Invalid JID %s: %v\n", contact, err)
			continue
		}

		if err := client.SubscribePresenceOptimized(jid); err != nil {
			fmt.Printf("❌ Failed to subscribe to presence for %s: %v\n", contact, err)
		} else {
			fmt.Printf("✅ Subscribed to presence for %s\n", contact)
		}
	}

	// Start presence connection monitoring
	ctx, cancel := context.WithCancel(context.Background())
	go client.MonitorPresenceConnection(ctx)

	// Periodic health check demonstration
	go func() {
		healthTicker := time.NewTicker(2 * time.Minute)
		defer healthTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-healthTicker.C:
				healthy, err := client.CheckWebSocketHealth()
				if err != nil {
					fmt.Printf("🔍 Health check failed: %v\n", err)
				} else if healthy {
					fmt.Println("✅ WebSocket health check: OK")
				} else {
					fmt.Println("⚠️ WebSocket health check: Unhealthy")
				}
			}
		}
	}()

	// Set up graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	fmt.Println("🎯 Presence monitoring active. Press Ctrl+C to stop.")
	fmt.Println("📱 Presence updates will be shown below:")
	fmt.Println("🔍 WebSocket health checks every 2 minutes")
	fmt.Println("---")

	// Wait for shutdown signal
	<-c
	fmt.Println("\n🛑 Shutting down...")

	// Cancel monitoring
	cancel()

	// Set presence to unavailable before disconnecting
	if err := client.SendPresence(types.PresenceUnavailable); err != nil {
		fmt.Printf("⚠️  Failed to set presence unavailable: %v\n", err)
	}

	// Disconnect
	client.Disconnect()
	fmt.Println("👋 Disconnected")
}
