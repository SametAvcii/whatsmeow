package whatsmeow

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestDefaultPresenceOptimizedConfig(t *testing.T) {
	config := DefaultPresenceOptimizedConfig()

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if config.KeepAliveInterval != 45*time.Second {
		t.Errorf("Expected KeepAliveInterval to be 45s, got %v", config.KeepAliveInterval)
	}

	if config.ReconnectDelay != 5*time.Second {
		t.Errorf("Expected ReconnectDelay to be 5s, got %v", config.ReconnectDelay)
	}

	if !config.IgnoreAbnormalClosureErrors {
		t.Error("Expected IgnoreAbnormalClosureErrors to be true")
	}

	if config.HandshakeTimeout != 30*time.Second {
		t.Errorf("Expected HandshakeTimeout to be 30s, got %v", config.HandshakeTimeout)
	}

	if config.ReadBufferSize != 4096 {
		t.Errorf("Expected ReadBufferSize to be 4096, got %d", config.ReadBufferSize)
	}

	if config.WriteBufferSize != 4096 {
		t.Errorf("Expected WriteBufferSize to be 4096, got %d", config.WriteBufferSize)
	}
}

func TestSetPresenceOptimizedMode(t *testing.T) {
	// Create a mock client for testing
	client := &Client{
		EnableAutoReconnect:  false,
		InitialAutoReconnect: false,
	}

	// Test with default config
	client.SetPresenceOptimizedMode(nil)

	if !client.EnableAutoReconnect {
		t.Error("Expected EnableAutoReconnect to be true")
	}

	if !client.InitialAutoReconnect {
		t.Error("Expected InitialAutoReconnect to be true")
	}

	if client.wsDialer == nil {
		t.Error("Expected wsDialer to be set")
	}

	if client.wsDialer.HandshakeTimeout != 30*time.Second {
		t.Errorf("Expected HandshakeTimeout to be 30s, got %v", client.wsDialer.HandshakeTimeout)
	}
}

func TestSetPresenceOptimizedModeWithExistingDialer(t *testing.T) {
	// Create a mock client with existing dialer
	client := &Client{
		wsDialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
			ReadBufferSize:   2048,
			WriteBufferSize:  2048,
		},
	}

	config := &PresenceOptimizedConfig{
		Enabled:          true,
		HandshakeTimeout: 30 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}

	client.SetPresenceOptimizedMode(config)

	// Should not override existing non-zero values
	if client.wsDialer.HandshakeTimeout != 10*time.Second {
		t.Errorf("Expected existing HandshakeTimeout to be preserved, got %v", client.wsDialer.HandshakeTimeout)
	}

	// Should not override existing non-zero values
	if client.wsDialer.ReadBufferSize != 2048 {
		t.Errorf("Expected existing ReadBufferSize to be preserved, got %d", client.wsDialer.ReadBufferSize)
	}
}

func TestSetPresenceOptimizedModeDisabled(t *testing.T) {
	client := &Client{
		EnableAutoReconnect: false,
	}

	config := &PresenceOptimizedConfig{
		Enabled: false,
	}

	client.SetPresenceOptimizedMode(config)

	// Should not change anything when disabled
	if client.EnableAutoReconnect {
		t.Error("Expected EnableAutoReconnect to remain false when disabled")
	}

	if client.wsDialer != nil {
		t.Error("Expected wsDialer to remain nil when disabled")
	}
}

func TestPresenceOptimizedConfigKeepAliveInterval(t *testing.T) {
	// Save original values
	originalMin := KeepAliveIntervalMin
	originalMax := KeepAliveIntervalMax

	// Restore original values after test
	defer func() {
		KeepAliveIntervalMin = originalMin
		KeepAliveIntervalMax = originalMax
	}()

	client := &Client{}
	config := &PresenceOptimizedConfig{
		Enabled:           true,
		KeepAliveInterval: 60 * time.Second,
	}

	client.SetPresenceOptimizedMode(config)

	if KeepAliveIntervalMin != 60*time.Second {
		t.Errorf("Expected KeepAliveIntervalMin to be 60s, got %v", KeepAliveIntervalMin)
	}

	if KeepAliveIntervalMax != 70*time.Second {
		t.Errorf("Expected KeepAliveIntervalMax to be 70s, got %v", KeepAliveIntervalMax)
	}
}

func TestCheckWebSocketHealth(t *testing.T) {
	// Create a mock client
	client := &Client{}

	// Test with nil client
	healthy, err := client.CheckWebSocketHealth()
	if err == nil {
		t.Error("Expected error for nil client")
	}
	if healthy {
		t.Error("Expected healthy to be false for nil client")
	}
}
