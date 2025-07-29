// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// Context keys for client information
type contextKey string

const (
	ClientJIDKey contextKey = "client_jid"
)

type FrameSocket struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel func()
	log    waLog.Logger
	lock   sync.Mutex

	URL         string
	HTTPHeaders http.Header

	Frames       chan []byte
	OnDisconnect func(remote bool)
	WriteTimeout time.Duration

	Header []byte
	Dialer websocket.Dialer

	incomingLength int
	receivedLength int
	incoming       []byte
	partialHeader  []byte

	// Error logging callback
	OnWebSocketError func(clientJID string, err error)

	// Health check fields
	lastPingTime     time.Time
	lastPongTime     time.Time
	healthCheckMutex sync.RWMutex
}

func NewFrameSocket(log waLog.Logger, dialer websocket.Dialer) *FrameSocket {
	return &FrameSocket{
		conn:   nil,
		log:    log,
		Header: WAConnHeader,
		Frames: make(chan []byte),

		URL:         URL,
		HTTPHeaders: http.Header{"Origin": {Origin}},

		Dialer: dialer,
	}
}

func (fs *FrameSocket) IsConnected() bool {
	return fs.conn != nil
}

// IsWebSocketHealthy checks if the websocket connection is actually healthy
func (fs *FrameSocket) IsWebSocketHealthy() bool {
	if fs.conn == nil {
		return false
	}

	fs.healthCheckMutex.RLock()
	defer fs.healthCheckMutex.RUnlock()

	// If we never sent a ping, consider it healthy (newly connected)
	if fs.lastPingTime.IsZero() {
		return true
	}

	// If we sent a ping but never got a pong, check timeout
	if fs.lastPongTime.Before(fs.lastPingTime) {
		// If ping was sent more than 15 seconds ago without pong, consider unhealthy
		if time.Since(fs.lastPingTime) > 15*time.Second {
			return false
		}
	}

	return true
}

// PingWebSocket sends a ping to test websocket health
func (fs *FrameSocket) PingWebSocket() error {
	if fs.conn == nil {
		return ErrSocketClosed
	}

	fs.healthCheckMutex.Lock()
	fs.lastPingTime = time.Now()
	fs.healthCheckMutex.Unlock()

	fs.log.Debugf("Sending websocket ping for health check")
	return fs.conn.WriteControl(websocket.PingMessage, []byte("health-check"), time.Now().Add(5*time.Second))
}

func (fs *FrameSocket) Context() context.Context {
	return fs.ctx
}

func (fs *FrameSocket) Close(code int) {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	if fs.conn == nil {
		return
	}

	if code > 0 {
		message := websocket.FormatCloseMessage(code, "")
		err := fs.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		if err != nil {
			fs.log.Warnf("Error sending close message: %v", err)
		}
	}

	fs.cancel()
	err := fs.conn.Close()
	if err != nil {
		fs.log.Errorf("Error closing websocket: %v", err)
	}
	fs.conn = nil
	fs.ctx = nil
	fs.cancel = nil
	if fs.OnDisconnect != nil {
		go fs.OnDisconnect(code == 0)
	}
}

func (fs *FrameSocket) Connect() error {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	if fs.conn != nil {
		return ErrSocketAlreadyOpen
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Configure dialer for better reliability
	if fs.Dialer.HandshakeTimeout == 0 {
		fs.Dialer.HandshakeTimeout = 30 * time.Second
	}
	if fs.Dialer.ReadBufferSize == 0 {
		fs.Dialer.ReadBufferSize = 4096
	}
	if fs.Dialer.WriteBufferSize == 0 {
		fs.Dialer.WriteBufferSize = 4096
	}

	fs.log.Debugf("Dialing %s", fs.URL)
	conn, _, err := fs.Dialer.Dial(fs.URL, fs.HTTPHeaders)
	if err != nil {
		cancel()
		return fmt.Errorf("couldn't dial whatsapp web websocket: %w", err)
	}

	fs.ctx, fs.cancel = ctx, cancel
	fs.conn = conn
	conn.SetCloseHandler(func(code int, text string) error {
		fs.log.Debugf("Server closed websocket with status %d/%s", code, text)
		cancel()
		// from default CloseHandler
		message := websocket.FormatCloseMessage(code, "")
		_ = conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		return nil
	})

	go fs.readPump(conn, ctx)
	return nil
}

// ConnectWithClientJID connects the websocket with client JID information in context
func (fs *FrameSocket) ConnectWithClientJID(clientJID string) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	if fs.conn != nil {
		return ErrSocketAlreadyOpen
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Add client JID to context
	ctx = context.WithValue(ctx, ClientJIDKey, clientJID)

	// Configure dialer for better reliability
	if fs.Dialer.HandshakeTimeout == 0 {
		fs.Dialer.HandshakeTimeout = 30 * time.Second
	}
	if fs.Dialer.ReadBufferSize == 0 {
		fs.Dialer.ReadBufferSize = 4096
	}
	if fs.Dialer.WriteBufferSize == 0 {
		fs.Dialer.WriteBufferSize = 4096
	}

	fs.log.Debugf("Dialing %s", fs.URL)
	conn, _, err := fs.Dialer.Dial(fs.URL, fs.HTTPHeaders)
	if err != nil {
		cancel()
		return fmt.Errorf("couldn't dial whatsapp web websocket: %w", err)
	}

	fs.ctx, fs.cancel = ctx, cancel
	fs.conn = conn
	conn.SetCloseHandler(func(code int, text string) error {
		fs.log.Debugf("Server closed websocket with status %d/%s", code, text)
		cancel()
		// from default CloseHandler
		message := websocket.FormatCloseMessage(code, "")
		_ = conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		return nil
	})

	go fs.readPump(conn, ctx)
	return nil
}

func (fs *FrameSocket) SendFrame(data []byte) error {
	conn := fs.conn
	if conn == nil {
		return ErrSocketClosed
	}
	dataLength := len(data)
	if dataLength >= FrameMaxSize {
		return fmt.Errorf("%w (got %d bytes, max %d bytes)", ErrFrameTooLarge, len(data), FrameMaxSize)
	}

	headerLength := len(fs.Header)
	// Whole frame is header + 3 bytes for length + data
	wholeFrame := make([]byte, headerLength+FrameLengthSize+dataLength)

	// Copy the header if it's there
	if fs.Header != nil {
		copy(wholeFrame[:headerLength], fs.Header)
		// We only want to send the header once
		fs.Header = nil
	}

	// Encode length of frame
	wholeFrame[headerLength] = byte(dataLength >> 16)
	wholeFrame[headerLength+1] = byte(dataLength >> 8)
	wholeFrame[headerLength+2] = byte(dataLength)

	// Copy actual frame data
	copy(wholeFrame[headerLength+FrameLengthSize:], data)

	if fs.WriteTimeout > 0 {
		err := conn.SetWriteDeadline(time.Now().Add(fs.WriteTimeout))
		if err != nil {
			fs.log.Warnf("Failed to set write deadline: %v", err)
		}
	}
	return conn.WriteMessage(websocket.BinaryMessage, wholeFrame)
}

func (fs *FrameSocket) frameComplete() {
	data := fs.incoming
	fs.incoming = nil
	fs.partialHeader = nil
	fs.incomingLength = 0
	fs.receivedLength = 0
	fs.Frames <- data
}

func (fs *FrameSocket) processData(msg []byte) {
	for len(msg) > 0 {
		// This probably doesn't happen a lot (if at all), so the code is unoptimized
		if fs.partialHeader != nil {
			msg = append(fs.partialHeader, msg...)
			fs.partialHeader = nil
		}
		if fs.incoming == nil {
			if len(msg) >= FrameLengthSize {
				length := (int(msg[0]) << 16) + (int(msg[1]) << 8) + int(msg[2])
				fs.incomingLength = length
				fs.receivedLength = len(msg)
				msg = msg[FrameLengthSize:]
				if len(msg) >= length {
					fs.incoming = msg[:length]
					msg = msg[length:]
					fs.frameComplete()
				} else {
					fs.incoming = make([]byte, length)
					copy(fs.incoming, msg)
					msg = nil
				}
			} else {
				fs.log.Warnf("Received partial header (report if this happens often)")
				fs.partialHeader = msg
				msg = nil
			}
		} else {
			if fs.receivedLength+len(msg) >= fs.incomingLength {
				copy(fs.incoming[fs.receivedLength:], msg[:fs.incomingLength-fs.receivedLength])
				msg = msg[fs.incomingLength-fs.receivedLength:]
				fs.frameComplete()
			} else {
				copy(fs.incoming[fs.receivedLength:], msg)
				fs.receivedLength += len(msg)
				msg = nil
			}
		}
	}
}

func (fs *FrameSocket) readPump(conn *websocket.Conn, ctx context.Context) {
	fs.log.Debugf("Frame websocket read pump starting %p", fs)
	defer func() {
		fs.log.Debugf("Frame websocket read pump exiting %p", fs)
		go fs.Close(0)
	}()

	// Set up ping/pong handlers for better connection health
	conn.SetPingHandler(func(appData string) error {
		fs.log.Debugf("Received ping, sending pong")
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	conn.SetPongHandler(func(appData string) error {
		fs.log.Debugf("Received pong")
		// Update last pong time for health tracking
		fs.healthCheckMutex.Lock()
		fs.lastPongTime = time.Now()
		fs.healthCheckMutex.Unlock()
		return nil
	})

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			// Check if it's a normal close or expected error
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				fs.log.Debugf("Websocket closed normally: %v", err)
				return
			}

			// Check if context was cancelled (expected disconnect)
			if errors.Is(ctx.Err(), context.Canceled) {
				fs.log.Debugf("Websocket read cancelled due to context cancellation")
				return
			}

			// For 1006 errors (abnormal closure), log at debug level instead of error
			// since these are often network-related and will trigger auto-reconnect
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				if strings.Contains(err.Error(), "1006") {
					fs.log.Debugf("Websocket abnormal closure (will auto-reconnect): %v", err)
				} else {
					fs.log.Warnf("Unexpected websocket close: %v", err)
				}
			} else {
				fs.log.Errorf("Error reading from websocket: %v", err)
			}

			// Log error to database if callback is set and client JID is available
			// Log all errors including 1006 to database for tracking
			if fs.OnWebSocketError != nil {
				if clientJID, ok := ctx.Value(ClientJIDKey).(string); ok && clientJID != "" {
					fs.OnWebSocketError(clientJID, err)
				}
			}
			return
		} else if msgType != websocket.BinaryMessage {
			fs.log.Warnf("Got unexpected websocket message type %d", msgType)
			continue
		}
		fs.processData(data)
	}
}
