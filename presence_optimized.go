// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// PresenceOptimizedConfig contains configuration for presence-optimized websocket connections
type PresenceOptimizedConfig struct {
	// Enable presence-optimized mode
	Enabled bool

	// Reduce keepalive frequency for presence-only usage
	KeepAliveInterval time.Duration

	// Faster reconnection for presence
	ReconnectDelay time.Duration

	// Ignore 1006 errors in logs
	IgnoreAbnormalClosureErrors bool

	// Custom websocket dialer settings
	HandshakeTimeout time.Duration
	ReadBufferSize   int
	WriteBufferSize  int
}

// DefaultPresenceOptimizedConfig returns default configuration for presence-optimized connections
func DefaultPresenceOptimizedConfig() *PresenceOptimizedConfig {
	return &PresenceOptimizedConfig{
		Enabled:                     true,
		KeepAliveInterval:           45 * time.Second, // Longer interval for presence-only
		ReconnectDelay:              5 * time.Second,  // Faster reconnect
		IgnoreAbnormalClosureErrors: true,
		HandshakeTimeout:            30 * time.Second,
		ReadBufferSize:              4096,
		WriteBufferSize:             4096,
	}
}

// SetPresenceOptimizedMode configures the client for presence-optimized usage
func (cli *Client) SetPresenceOptimizedMode(config *PresenceOptimizedConfig) {
	if config == nil {
		config = DefaultPresenceOptimizedConfig()
	}

	if config.Enabled {
		// Adjust keepalive settings for presence usage
		if config.KeepAliveInterval > 0 {
			KeepAliveIntervalMin = config.KeepAliveInterval
			KeepAliveIntervalMax = config.KeepAliveInterval + 10*time.Second
		}

		// Configure websocket dialer if not already set
		if cli.wsDialer == nil {
			cli.wsDialer = &websocket.Dialer{
				HandshakeTimeout: config.HandshakeTimeout,
				ReadBufferSize:   config.ReadBufferSize,
				WriteBufferSize:  config.WriteBufferSize,
			}
		} else {
			// Update existing dialer
			if cli.wsDialer.HandshakeTimeout == 0 {
				cli.wsDialer.HandshakeTimeout = config.HandshakeTimeout
			}
			if cli.wsDialer.ReadBufferSize == 0 {
				cli.wsDialer.ReadBufferSize = config.ReadBufferSize
			}
			if cli.wsDialer.WriteBufferSize == 0 {
				cli.wsDialer.WriteBufferSize = config.WriteBufferSize
			}
		}

		// Enable auto-reconnect with faster timing
		cli.EnableAutoReconnect = true
		cli.InitialAutoReconnect = true

		cli.Log.Infof("Presence-optimized mode enabled with %v keepalive interval", config.KeepAliveInterval)
	}
}

// ConnectForPresence connects the client optimized for presence usage
func (cli *Client) ConnectForPresence(config *PresenceOptimizedConfig) error {
	cli.SetPresenceOptimizedMode(config)
	return cli.Connect()
}

// SubscribePresenceOptimized subscribes to presence with optimized error handling
func (cli *Client) SubscribePresenceOptimized(jid types.JID) error {
	// First ensure we're online for presence
	if err := cli.SendPresence(types.PresenceAvailable); err != nil {
		cli.Log.Warnf("Failed to set presence available before subscribing: %v", err)
	}

	// Subscribe to presence
	err := cli.SubscribePresence(jid)
	if err != nil {
		cli.Log.Warnf("Failed to subscribe to presence for %s: %v", jid, err)
		return err
	}

	cli.Log.Debugf("Successfully subscribed to presence for %s", jid)
	return nil
}

// HandlePresenceEvents sets up optimized presence event handling
func (cli *Client) HandlePresenceEvents(handler func(*events.Presence)) uint32 {
	return cli.AddEventHandler(func(evt any) {
		if presence, ok := evt.(*events.Presence); ok {
			handler(presence)
		}
	})
}

// MonitorPresenceConnection monitors connection health for presence usage
func (cli *Client) MonitorPresenceConnection(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// First check basic connection
			if !cli.IsConnected() {
				cli.Log.Debugf("Presence connection lost, attempting reconnect...")
				if cli.EnableAutoReconnect {
					go cli.autoReconnect()
				}
				continue
			}

			// Then check websocket health
			if !cli.IsWebSocketHealthy() {
				cli.Log.Debugf("WebSocket appears unhealthy, testing with ping...")

				// Try to ping the websocket
				if err := cli.PingWebSocket(); err != nil {
					cli.Log.Warnf("WebSocket ping failed: %v, forcing reconnect", err)
					cli.Disconnect()
					cli.resetExpectedDisconnect()
					if cli.EnableAutoReconnect {
						go cli.autoReconnect()
					}
				}
			}
		}
	}
}

// CheckWebSocketHealth performs an immediate health check on the websocket
func (cli *Client) CheckWebSocketHealth() (bool, error) {
	if !cli.IsConnected() {
		return false, ErrNotConnected
	}

	if !cli.IsWebSocketHealthy() {
		// Try ping to confirm
		if err := cli.PingWebSocket(); err != nil {
			return false, err
		}
		// Wait a moment for pong
		time.Sleep(2 * time.Second)
		return cli.IsWebSocketHealthy(), nil
	}

	return true, nil
}
