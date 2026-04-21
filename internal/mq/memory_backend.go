package mq

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// MemoryBackend implements Backend using in-memory channels
type MemoryBackend struct {
	mu      sync.RWMutex
	topics  map[string][]*subscription
	closed  bool
	counter uint64
}

type subscription struct {
	id      string
	topic   string
	handler Handler
	ch      chan Message
	cancel  context.CancelFunc
}

// NewMemoryBackend creates a new in-memory message queue backend
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		topics: make(map[string][]*subscription),
	}
}

// Name returns the backend name
func (mb *MemoryBackend) Name() string {
	return "memory"
}

// Publish publishes a message to all subscribers of a topic
func (mb *MemoryBackend) Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) (*Message, error) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	if mb.closed {
		return nil, fmt.Errorf("backend is closed")
	}

	msg := &Message{
		ID:        uuid.New().String(),
		Topic:     topic,
		Payload:   payload,
		Headers:   headers,
		Timestamp: time.Now(),
	}

	subs, ok := mb.topics[topic]
	if !ok {
		return msg, nil // no subscribers, message is dropped
	}

	for _, sub := range subs {
		select {
		case sub.ch <- *msg:
		default:
			// Channel full, drop message for this subscriber
		}
	}

	return msg, nil
}

// Subscribe subscribes to a topic
func (mb *MemoryBackend) Subscribe(ctx context.Context, topic string, handler Handler) (*Subscription, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.closed {
		return nil, fmt.Errorf("backend is closed")
	}

	subID := fmt.Sprintf("sub-%d", atomic.AddUint64(&mb.counter, 1))
	ch := make(chan Message, 256)
	subCtx, cancel := context.WithCancel(ctx)

	s := &subscription{
		id:      subID,
		topic:   topic,
		handler: handler,
		ch:      ch,
		cancel:  cancel,
	}

	mb.topics[topic] = append(mb.topics[topic], s)

	// Start delivery goroutine
	go mb.deliver(subCtx, s)

	return &Subscription{
		ID:      subID,
		Topic:   topic,
		Handler: handler,
	}, nil
}

// Unsubscribe removes a subscription
func (mb *MemoryBackend) Unsubscribe(ctx context.Context, subID string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	for topic, subs := range mb.topics {
		for i, sub := range subs {
			if sub.id == subID {
				sub.cancel()
				mb.topics[topic] = append(subs[:i], subs[i+1:]...)
				if len(mb.topics[topic]) == 0 {
					delete(mb.topics, topic)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("subscription %q not found", subID)
}

// Close shuts down the backend
func (mb *MemoryBackend) Close() error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.closed {
		return nil
	}
	mb.closed = true

	for _, subs := range mb.topics {
		for _, sub := range subs {
			sub.cancel()
		}
	}
	mb.topics = make(map[string][]*subscription)

	return nil
}

// deliver forwards messages from the channel to the handler
func (mb *MemoryBackend) deliver(ctx context.Context, sub *subscription) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sub.ch:
			if err := sub.handler(ctx, msg); err != nil {
				// Handler error — log and continue
				_ = err
			}
		}
	}
}