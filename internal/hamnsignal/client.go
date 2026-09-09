package hamnsignal

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

var reconnectDelays = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second}

// Client is a receive-only connection to the public Hamnsignal WebSocket.
type Client struct {
	URL          string
	OnEvent      func(Event)
	OnConnection func(bool)
	OnError      func(error)
}

// Run reconnects until ctx is cancelled. It sends no application-level data.
func (c *Client) Run(ctx context.Context) error {
	url := c.URL
	if url == "" {
		url = Endpoint
	}
	delayIndex := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			c.report(fmt.Errorf("connect Hamnsignal WebSocket: %w", err))
			if !waitContext(ctx, reconnectDelays[delayIndex]) {
				return nil
			}
			if delayIndex < len(reconnectDelays)-1 {
				delayIndex++
			}
			continue
		}

		delayIndex = 0 // A successful connection resets the retry sequence.
		c.connected(true)
		err = c.readConnection(ctx, conn)
		c.connected(false)
		if err != nil && ctx.Err() == nil {
			c.report(err)
		}
		if !waitContext(ctx, reconnectDelays[delayIndex]) {
			return nil
		}
		if delayIndex < len(reconnectDelays)-1 {
			delayIndex++
		}
	}
}

func (c *Client) readConnection(ctx context.Context, conn *websocket.Conn) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	defer func() { close(done); _ = conn.Close() }()
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read Hamnsignal WebSocket: %w", err)
		}
		if messageType != websocket.TextMessage {
			continue
		}
		event, err := DecodeEvent(payload)
		if err != nil {
			c.report(err)
			continue
		}
		if c.OnEvent != nil {
			c.OnEvent(event)
		}
	}
}

func (c *Client) connected(value bool) {
	if c.OnConnection != nil {
		c.OnConnection(value)
	}
}
func (c *Client) report(err error) {
	if c.OnError != nil {
		c.OnError(err)
	}
}
func waitContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
