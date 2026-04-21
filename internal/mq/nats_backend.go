//go:build nats

package mq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NATSBackend implements Backend using NATS as the message broker
type NATSBackend struct {
	mu      sync.RWMutex
	conn    *nats.Conn
	subs    map[string]*nats.Subscription
	handlers map[string]Handler
	closed  bool
	counter uint64
}

// NATSConfig holds NATS connection configuration
type NATSConfig struct {
	URL         string        `yaml:"url"`
	Name        string        `yaml:"name,omitempty"`
	ReconnectWait time.Duration `yaml:"reconnect_wait,omitempty"`
	MaxReconnect int           `yaml:"max_reconnect,omitempty"`
}

// NewNATSBackend creates a new NATS-based message queue backend
func NewNATSBackend(cfg NATSConfig) (*NATSBackend, error) {
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.MaxReconnects(cfg.MaxReconnect),
	}

	if cfg.ReconnectWait == 0 {
		opts = append(opts, nats.ReconnectWait(2*time.Second))
	}
	if cfg.MaxReconnect == 0 {
		opts = append(opts, nats.MaxReconnects(60))
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	return &NATSBackend{
		conn:     conn,
		subs:     make(map[string]*nats.Subscription),
		handlers: make(map[string]Handler),
	}, nil
}

// Name returns the backend name
func (nb *NATSBackend) Name() string {
	return "nats"
}

// Publish publishes a message to a NATS subject
func (nb *NATSBackend) Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) (*Message, error) {
	nb.mu.RLock()
	defer nb.mu.RUnlock()

	if nb.closed {
		return nil, fmt.Errorf("backend is closed")
	}

	msg := &Message{
		ID:        uuid.New().String(),
		Topic:     topic,
		Payload:   payload,
		Headers:   headers,
		Timestamp: time.Now(),
	}

	// Use message ID as NATS reply subject for potential request-reply
	data, _ := encodeMessage(msg)
	if err := nb.conn.Publish(topic, data); err != nil {
		return nil, fmt.Errorf("nats publish: %w", err)
	}

	return msg, nil
}

// Subscribe subscribes to a NATS subject
func (nb *NATSBackend) Subscribe(ctx context.Context, topic string, handler Handler) (*Subscription, error) {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	if nb.closed {
		return nil, fmt.Errorf("backend is closed")
	}

	subID := fmt.Sprintf("nats-sub-%d", nb.counter)
	nb.counter++

	natsSub, err := nb.conn.Subscribe(topic, func(natsMsg *nats.Msg) {
		msg, err := decodeMessage(natsMsg.Data)
		if err != nil {
			return
		}
		handler(ctx, *msg)
	})

	if err != nil {
		return nil, fmt.Errorf("nats subscribe: %w", err)
	}

	nb.subs[subID] = natsSub
	nb.handlers[subID] = handler

	return &Subscription{
		ID:      subID,
		Topic:   topic,
		Handler: handler,
	}, nil
}

// Unsubscribe removes a NATS subscription
func (nb *NATSBackend) Unsubscribe(ctx context.Context, subID string) error {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	sub, ok := nb.subs[subID]
	if !ok {
		return fmt.Errorf("subscription %q not found", subID)
	}

	if err := sub.Unsubscribe(); err != nil {
		return fmt.Errorf("nats unsubscribe: %w", err)
	}

	delete(nb.subs, subID)
	delete(nb.handlers, subID)

	return nil
}

// Close shuts down the NATS connection
func (nb *NATSBackend) Close() error {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	if nb.closed {
		return nil
	}
	nb.closed = true

	for _, sub := range nb.subs {
		sub.Unsubscribe()
	}
	nb.conn.Close()

	return nil
}

// encodeMessage serializes a Message for NATS transport
func encodeMessage(msg *Message) ([]byte, error) {
	// Simple encoding: ID\nTopic\nTimestamp\nHeaders\nPayload
	result := fmt.Sprintf("%s\n%s\n%s\n", msg.ID, msg.Topic, msg.Timestamp.Format(time.RFC3339Nano))
	for k, v := range msg.Headers {
		result += fmt.Sprintf("H:%s=%s\n", k, v)
	}
	result += "\n" // empty line separates headers from payload
	result += string(msg.Payload)
	return []byte(result), nil
}

// decodeMessage deserializes a Message from NATS transport
func decodeMessage(data []byte) (*Message, error) {
	msg := &Message{
		Headers: make(map[string]string),
	}

	// Simple parsing
	lines := splitLines(data)
	if len(lines) < 3 {
		return nil, fmt.Errorf("invalid message format")
	}

	msg.ID = lines[0]
	msg.Topic = lines[1]
	ts, err := time.Parse(time.RFC3339Nano, lines[2])
	if err != nil {
		msg.Timestamp = time.Now()
	} else {
		msg.Timestamp = ts
	}

	// Parse headers and find payload separator
	payloadStart := 3
	for i := 3; i < len(lines); i++ {
		if lines[i] == "" {
			payloadStart = i + 1
			break
		}
		if len(lines[i]) > 2 && lines[i][:2] == "H:" {
			// Parse header
			hdr := lines[i][2:]
			for j := 0; j < len(hdr); j++ {
				if hdr[j] == '=' {
					msg.Headers[hdr[:j]] = hdr[j+1:]
					break
				}
			}
		}
	}

	// Remaining lines are payload
	if payloadStart < len(lines) {
		payload := ""
		for i := payloadStart; i < len(lines); i++ {
			if i > payloadStart {
				payload += "\n"
			}
			payload += lines[i]
		}
		msg.Payload = []byte(payload)
	}

	return msg, nil
}

// splitLines splits byte data into lines
func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}