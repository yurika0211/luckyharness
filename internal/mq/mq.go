// Package mq provides a message queue abstraction with support for
// multiple backends (in-memory, NATS) and publish/subscribe patterns.
package mq

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Message represents a message in the queue
type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Payload   []byte            `json:"payload"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// Subscription represents a subscription to a topic
type Subscription struct {
	ID      string
	Topic   string
	Handler Handler
}

// Handler processes incoming messages
type Handler func(ctx context.Context, msg Message) error

// Backend is the interface for message queue backends
type Backend interface {
	// Name returns the backend name
	Name() string

	// Publish publishes a message to a topic
	Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) (*Message, error)

	// Subscribe subscribes to a topic with a handler
	Subscribe(ctx context.Context, topic string, handler Handler) (*Subscription, error)

	// Unsubscribe removes a subscription
	Unsubscribe(ctx context.Context, subID string) error

	// Close shuts down the backend
	Close() error
}

// Queue is the message queue manager
type Queue struct {
	mu       sync.RWMutex
	backend  Backend
	closed   bool
	subs     map[string]*Subscription
	counter  uint64
}

// New creates a new message queue with the given backend
func New(backend Backend) (*Queue, error) {
	if backend == nil {
		return nil, fmt.Errorf("backend is required")
	}

	return &Queue{
		backend: backend,
		subs:    make(map[string]*Subscription),
	}, nil
}

// Publish publishes a message to a topic
func (q *Queue) Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) (*Message, error) {
	if q.isClosed() {
		return nil, fmt.Errorf("queue is closed")
	}
	return q.backend.Publish(ctx, topic, payload, headers)
}

// PublishString is a convenience method to publish a string message
func (q *Queue) PublishString(ctx context.Context, topic, payload string) (*Message, error) {
	return q.Publish(ctx, topic, []byte(payload), nil)
}

// Subscribe subscribes to a topic
func (q *Queue) Subscribe(ctx context.Context, topic string, handler Handler) (*Subscription, error) {
	if q.isClosed() {
		return nil, fmt.Errorf("queue is closed")
	}

	sub, err := q.backend.Subscribe(ctx, topic, handler)
	if err != nil {
		return nil, err
	}

	q.mu.Lock()
	q.subs[sub.ID] = sub
	q.mu.Unlock()

	return sub, nil
}

// Unsubscribe removes a subscription
func (q *Queue) Unsubscribe(ctx context.Context, subID string) error {
	q.mu.Lock()
	delete(q.subs, subID)
	q.mu.Unlock()

	return q.backend.Unsubscribe(ctx, subID)
}

// Close shuts down the queue and backend
func (q *Queue) Close() error {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()

	return q.backend.Close()
}

// Backend returns the current backend
func (q *Queue) Backend() Backend {
	return q.backend
}

// SubscriptionCount returns the number of active subscriptions
func (q *Queue) SubscriptionCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.subs)
}

// isClosed checks if the queue is closed
func (q *Queue) isClosed() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.closed
}