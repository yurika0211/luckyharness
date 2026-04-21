package mq

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBackend_PublishSubscribe(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	var mu sync.Mutex
	var received Message
	var got bool

	_, err := backend.Subscribe(ctx, "test.topic", func(ctx context.Context, msg Message) error {
		mu.Lock()
		received = msg
		got = true
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)

	_, err = backend.Publish(ctx, "test.topic", []byte("hello"), nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got
	}, time.Second, 10*time.Millisecond)
	mu.Lock()
	assert.Equal(t, "hello", string(received.Payload))
	assert.Equal(t, "test.topic", received.Topic)
	mu.Unlock()
}

func TestMemoryBackend_MultipleSubscribers(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	var count atomic.Int32

	for i := 0; i < 3; i++ {
		_, err := backend.Subscribe(ctx, "broadcast", func(ctx context.Context, msg Message) error {
			count.Add(1)
			return nil
		})
		require.NoError(t, err)
	}

	_, err := backend.Publish(ctx, "broadcast", []byte("msg"), nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool { return count.Load() == 3 }, time.Second, 10*time.Millisecond)
}

func TestMemoryBackend_PublishNoSubscribers(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	// Should not error, message is just dropped
	_, err := backend.Publish(ctx, "no.subs", []byte("dropped"), nil)
	require.NoError(t, err)
}

func TestMemoryBackend_Unsubscribe(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	var count atomic.Int32

	sub, err := backend.Subscribe(ctx, "unsub.topic", func(ctx context.Context, msg Message) error {
		count.Add(1)
		return nil
	})
	require.NoError(t, err)

	backend.Publish(ctx, "unsub.topic", []byte("1"), nil)
	assert.Eventually(t, func() bool { return count.Load() == 1 }, time.Second, 10*time.Millisecond)

	err = backend.Unsubscribe(ctx, sub.ID)
	require.NoError(t, err)

	backend.Publish(ctx, "unsub.topic", []byte("2"), nil)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), count.Load()) // Should not increase
}

func TestMemoryBackend_UnsubscribeNotFound(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	err := backend.Unsubscribe(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestMemoryBackend_PublishAfterClose(t *testing.T) {
	backend := NewMemoryBackend()
	backend.Close()

	ctx := context.Background()
	_, err := backend.Publish(ctx, "closed.topic", []byte("msg"), nil)
	assert.Error(t, err)
}

func TestMemoryBackend_SubscribeAfterClose(t *testing.T) {
	backend := NewMemoryBackend()
	backend.Close()

	ctx := context.Background()
	_, err := backend.Subscribe(ctx, "closed.topic", func(ctx context.Context, msg Message) error {
		return nil
	})
	assert.Error(t, err)
}

func TestMemoryBackend_MessageHeaders(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	var mu sync.Mutex
	var received Message
	var got bool

	_, err := backend.Subscribe(ctx, "headers.topic", func(ctx context.Context, msg Message) error {
		mu.Lock()
		received = msg
		got = true
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)

	headers := map[string]string{
		"trace-id": "abc123",
		"source":   "test",
	}
	_, err = backend.Publish(ctx, "headers.topic", []byte("data"), headers)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got
	}, time.Second, 10*time.Millisecond)
	mu.Lock()
	assert.Equal(t, "abc123", received.Headers["trace-id"])
	assert.Equal(t, "test", received.Headers["source"])
	mu.Unlock()
}

func TestQueue_PublishSubscribe(t *testing.T) {
	backend := NewMemoryBackend()
	q, err := New(backend)
	require.NoError(t, err)
	defer q.Close()

	ctx := context.Background()
	var mu sync.Mutex
	var received Message
	var got bool

	_, err = q.Subscribe(ctx, "queue.test", func(ctx context.Context, msg Message) error {
		mu.Lock()
		received = msg
		got = true
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)

	_, err = q.Publish(ctx, "queue.test", []byte("hello"), nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got
	}, time.Second, 10*time.Millisecond)
	mu.Lock()
	assert.Equal(t, "hello", string(received.Payload))
	mu.Unlock()
}

func TestQueue_PublishString(t *testing.T) {
	backend := NewMemoryBackend()
	q, err := New(backend)
	require.NoError(t, err)
	defer q.Close()

	ctx := context.Background()
	var mu sync.Mutex
	var received Message
	var got bool

	q.Subscribe(ctx, "str.test", func(ctx context.Context, msg Message) error {
		mu.Lock()
		received = msg
		got = true
		mu.Unlock()
		return nil
	})

	q.PublishString(ctx, "str.test", "hello string")
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got
	}, time.Second, 10*time.Millisecond)
	mu.Lock()
	assert.Equal(t, "hello string", string(received.Payload))
	mu.Unlock()
}

func TestQueue_Unsubscribe(t *testing.T) {
	backend := NewMemoryBackend()
	q, err := New(backend)
	require.NoError(t, err)
	defer q.Close()

	ctx := context.Background()
	var count atomic.Int32

	sub, _ := q.Subscribe(ctx, "q.unsub", func(ctx context.Context, msg Message) error {
		count.Add(1)
		return nil
	})

	q.PublishString(ctx, "q.unsub", "1")
	assert.Eventually(t, func() bool { return count.Load() == 1 }, time.Second, 10*time.Millisecond)

	q.Unsubscribe(ctx, sub.ID)
	assert.Equal(t, 0, q.SubscriptionCount())

	q.PublishString(ctx, "q.unsub", "2")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), count.Load())
}

func TestQueue_SubscriptionCount(t *testing.T) {
	backend := NewMemoryBackend()
	q, err := New(backend)
	require.NoError(t, err)
	defer q.Close()

	ctx := context.Background()

	assert.Equal(t, 0, q.SubscriptionCount())

	q.Subscribe(ctx, "count.1", func(ctx context.Context, msg Message) error { return nil })
	assert.Equal(t, 1, q.SubscriptionCount())

	q.Subscribe(ctx, "count.2", func(ctx context.Context, msg Message) error { return nil })
	assert.Equal(t, 2, q.SubscriptionCount())
}

func TestQueue_Backend(t *testing.T) {
	backend := NewMemoryBackend()
	q, err := New(backend)
	require.NoError(t, err)
	defer q.Close()

	assert.Equal(t, backend, q.Backend())
}

func TestQueue_NoBackend(t *testing.T) {
	_, err := New(nil)
	assert.Error(t, err)
}

func TestQueue_PublishAfterClose(t *testing.T) {
	backend := NewMemoryBackend()
	q, _ := New(backend)
	q.Close()

	ctx := context.Background()
	_, err := q.Publish(ctx, "closed", []byte("msg"), nil)
	assert.Error(t, err)
}

func TestQueue_SubscribeAfterClose(t *testing.T) {
	backend := NewMemoryBackend()
	q, _ := New(backend)
	q.Close()

	ctx := context.Background()
	_, err := q.Subscribe(ctx, "closed", func(ctx context.Context, msg Message) error { return nil })
	assert.Error(t, err)
}

func TestMemoryBackend_TopicIsolation(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	var topicA, topicB atomic.Int32

	backend.Subscribe(ctx, "topic.a", func(ctx context.Context, msg Message) error {
		topicA.Add(1)
		return nil
	})
	backend.Subscribe(ctx, "topic.b", func(ctx context.Context, msg Message) error {
		topicB.Add(1)
		return nil
	})

	backend.Publish(ctx, "topic.a", []byte("a1"), nil)
	backend.Publish(ctx, "topic.a", []byte("a2"), nil)
	backend.Publish(ctx, "topic.b", []byte("b1"), nil)

	assert.Eventually(t, func() bool { return topicA.Load() == 2 }, time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool { return topicB.Load() == 1 }, time.Second, 10*time.Millisecond)
}