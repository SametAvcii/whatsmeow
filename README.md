# whatsmeow
[![Go Reference](https://pkg.go.dev/badge/go.mau.fi/whatsmeow.svg)](https://pkg.go.dev/go.mau.fi/whatsmeow)

whatsmeow is a Go library for the WhatsApp web multidevice API.

## Discussion
Matrix room: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)

For questions about the WhatsApp protocol (like how to send a specific type of
message), you can also use the [WhatsApp protocol Q&A] section on GitHub
discussions.

[WhatsApp protocol Q&A]: https://github.com/tulir/whatsmeow/discussions/categories/whatsapp-protocol-q-a

## Usage
The [godoc](https://pkg.go.dev/go.mau.fi/whatsmeow) includes docs for all methods and event types.
There's also a [simple example](https://pkg.go.dev/go.mau.fi/whatsmeow#example-package) at the top.

## Features
Most core features are already present:

* Sending messages to private chats and groups (both text and media)
* Receiving all messages
* Managing groups and receiving group change events
* Joining via invite messages, using and creating invite links
* Sending and receiving typing notifications
* Sending and receiving delivery and read receipts
* Reading and writing app state (contact list, chat pin/mute status, etc)
* Sending and handling retry receipts if message decryption fails
* Sending status messages (experimental, may not work for large contact lists)
* **Presence monitoring with optimized websocket connection handling**

## Presence Optimization

For applications that primarily monitor presence status (online/offline), whatsmeow includes optimized connection handling to reduce websocket errors and improve reliability:

### Features
* Reduced logging of abnormal closure errors (1006) that are common with presence-only usage
* Optimized keepalive intervals for presence monitoring
* Enhanced websocket connection stability with ping/pong handlers
* **Real websocket health checking** - goes beyond basic connection status
* Faster reconnection for presence-focused applications
* Dedicated presence event handling

### Usage
```go
// Configure for presence-optimized usage
config := &whatsmeow.PresenceOptimizedConfig{
    Enabled:                     true,
    KeepAliveInterval:          60 * time.Second,
    IgnoreAbnormalClosureErrors: true,
}

// Connect with optimization
client.ConnectForPresence(config)

// Subscribe to presence with error handling
client.SubscribePresenceOptimized(jid)

// Handle presence events
client.HandlePresenceEvents(func(presence *events.Presence) {
    fmt.Printf("%s is %s\n", presence.From,
        map[bool]string{true: "offline", false: "online"}[presence.Unavailable])
})

// Check websocket health (beyond basic connection status)
if client.IsConnected() {
    if healthy, err := client.CheckWebSocketHealth(); err != nil {
        fmt.Printf("Health check failed: %v\n", err)
    } else if !healthy {
        fmt.Println("WebSocket is connected but unhealthy")
        // Will trigger automatic reconnection
    }
}
```

See `examples/presence_optimized_example.go` for a complete example.

Things that are not yet implemented:

* Sending broadcast list messages (this is not supported on WhatsApp web either)
* Calls
