package configcenter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBackend_GetSet(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()

	// Set a value
	entry, err := backend.Set(ctx, "test.key", "hello", TypeString)
	require.NoError(t, err)
	assert.Equal(t, "test.key", entry.Key)
	assert.Equal(t, "hello", entry.Value)
	assert.Equal(t, TypeString, entry.Type)
	assert.Equal(t, int64(1), entry.Version)

	// Get the value
	got, err := backend.Get(ctx, "test.key")
	require.NoError(t, err)
	assert.Equal(t, "hello", got.Value)
}

func TestMemoryBackend_GetNotFound(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	_, err := backend.Get(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestMemoryBackend_Delete(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()

	backend.Set(ctx, "del.key", "value", TypeString)
	err := backend.Delete(ctx, "del.key")
	require.NoError(t, err)

	_, err = backend.Get(ctx, "del.key")
	assert.Error(t, err)
}

func TestMemoryBackend_DeleteNotFound(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()
	err := backend.Delete(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestMemoryBackend_List(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()

	backend.Set(ctx, "app.name", "luckyharness", TypeString)
	backend.Set(ctx, "app.version", "0.28.0", TypeString)
	backend.Set(ctx, "db.host", "localhost", TypeString)

	// List all
	entries, err := backend.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// List with prefix
	entries, err = backend.List(ctx, "app")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestMemoryBackend_Watch(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()

	ch, err := backend.Watch(ctx, "")
	require.NoError(t, err)

	// Set a value to trigger watch
	backend.Set(ctx, "watch.key", "triggered", TypeString)

	select {
	case event := <-ch:
		assert.Equal(t, "watch.key", event.Key)
		assert.Equal(t, "triggered", event.NewVal)
	case <-time.After(time.Second):
		t.Fatal("watch event not received within timeout")
	}
}

func TestMemoryBackend_VersionIncrement(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	ctx := context.Background()

	e1, _ := backend.Set(ctx, "key1", "v1", TypeString)
	e2, _ := backend.Set(ctx, "key2", "v2", TypeString)
	e3, _ := backend.Set(ctx, "key1", "v1-updated", TypeString)

	assert.Equal(t, int64(1), e1.Version)
	assert.Equal(t, int64(2), e2.Version)
	assert.Equal(t, int64(3), e3.Version)
}

func TestCenter_GetSet(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	entry, err := center.Set(ctx, "center.key", "value1", TypeString)
	require.NoError(t, err)
	assert.Equal(t, "center.key", entry.Key)

	got, err := center.Get(ctx, "center.key")
	require.NoError(t, err)
	assert.Equal(t, "value1", got.Value)
}

func TestCenter_GetDefault(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	// Key doesn't exist, should return default
	val := center.GetDefault(ctx, "missing.key", "default_val")
	assert.Equal(t, "default_val", val)

	// Set and get
	center.Set(ctx, "existing.key", "real_val", TypeString)
	val = center.GetDefault(ctx, "existing.key", "default_val")
	assert.Equal(t, "real_val", val)
}

func TestCenter_GetString(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	center.Set(ctx, "str.key", "hello", TypeString)
	val, err := center.GetString(ctx, "str.key")
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

func TestCenter_Delete(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	center.Set(ctx, "del.key", "value", TypeString)
	err = center.Delete(ctx, "del.key")
	require.NoError(t, err)

	_, err = center.Get(ctx, "del.key")
	assert.Error(t, err)
}

func TestCenter_List(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	center.Set(ctx, "svc.name", "lh", TypeString)
	center.Set(ctx, "svc.port", "8080", TypeString)
	center.Set(ctx, "db.host", "localhost", TypeString)

	entries, err := center.List(ctx, "svc")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestCenter_Watch(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	var mu sync.Mutex
	var receivedEvent ChangeEvent
	var eventReceived bool

	center.Watch("app.", func(event ChangeEvent) {
		mu.Lock()
		receivedEvent = event
		eventReceived = true
		mu.Unlock()
	})

	center.Set(ctx, "app.name", "luckyharness", TypeString)

	// Wait for async callback
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return eventReceived
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, "app.name", receivedEvent.Key)
	assert.Equal(t, "luckyharness", receivedEvent.NewVal)
	mu.Unlock()
}

func TestCenter_WatchPrefixFilter(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	var mu sync.Mutex
	var appEvents []ChangeEvent
	var dbEvents []ChangeEvent

	center.Watch("app.", func(event ChangeEvent) {
		mu.Lock()
		appEvents = append(appEvents, event)
		mu.Unlock()
	})
	center.Watch("db.", func(event ChangeEvent) {
		mu.Lock()
		dbEvents = append(dbEvents, event)
		mu.Unlock()
	})

	center.Set(ctx, "app.name", "lh", TypeString)
	center.Set(ctx, "db.host", "localhost", TypeString)
	center.Set(ctx, "other.key", "value", TypeString)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(appEvents) >= 1 && len(dbEvents) >= 1
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Len(t, appEvents, 1)
	assert.Len(t, dbEvents, 1)
	assert.Equal(t, "app.name", appEvents[0].Key)
	assert.Equal(t, "db.host", dbEvents[0].Key)
	mu.Unlock()
}

func TestCenter_Unwatch(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	var mu sync.Mutex
	var eventCount int

	center.Watch("test.", func(event ChangeEvent) {
		mu.Lock()
		eventCount++
		mu.Unlock()
	})

	center.Set(ctx, "test.key1", "v1", TypeString)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return eventCount >= 1
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, 1, eventCount)
	mu.Unlock()

	center.Unwatch("test.")

	center.Set(ctx, "test.key2", "v2", TypeString)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 1, eventCount) // Should not increase
	mu.Unlock()
}

func TestCenter_FallbackBackend(t *testing.T) {
	primary := NewMemoryBackend()
	secondary := NewMemoryBackend()
	center, err := New(primary, secondary)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	// Set in secondary only
	secondary.Set(ctx, "secondary.key", "from-secondary", TypeString)

	// Should be accessible via center (fallback)
	got, err := center.Get(ctx, "secondary.key")
	require.NoError(t, err)
	assert.Equal(t, "from-secondary", got.Value)
}

func TestCenter_PrimaryBackend(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	assert.Equal(t, backend, center.Primary())
}

func TestCenter_Backends(t *testing.T) {
	b1 := NewMemoryBackend()
	b2 := NewMemoryBackend()
	center, err := New(b1, b2)
	require.NoError(t, err)
	defer center.Close()

	backends := center.Backends()
	assert.Len(t, backends, 2)
}

func TestCenter_NoBackends(t *testing.T) {
	_, err := New()
	assert.Error(t, err)
}

func TestCenter_SetAfterClose(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)

	center.Close()

	ctx := context.Background()
	_, err = center.Set(ctx, "closed.key", "value", TypeString)
	assert.Error(t, err)
}

func TestCenter_ChangeEventOldValue(t *testing.T) {
	backend := NewMemoryBackend()
	center, err := New(backend)
	require.NoError(t, err)
	defer center.Close()

	ctx := context.Background()

	var mu sync.Mutex
	var receivedEvent ChangeEvent

	center.Watch("key.", func(event ChangeEvent) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
	})

	// First set (no old value)
	center.Set(ctx, "key.1", "initial", TypeString)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return receivedEvent.Key == "key.1"
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, "", receivedEvent.OldVal)
	assert.Equal(t, "initial", receivedEvent.NewVal)
	mu.Unlock()

	// Update (should have old value)
	center.Set(ctx, "key.1", "updated", TypeString)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return receivedEvent.NewVal == "updated"
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, "key.1", receivedEvent.Key)
	assert.Equal(t, "initial", receivedEvent.OldVal)
	assert.Equal(t, "updated", receivedEvent.NewVal)
	mu.Unlock()
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		key    string
		prefix string
		want   bool
	}{
		{"app.name", "app", true},
		{"app.name", "app.", true},
		{"app.name", "db", false},
		{"app.name", "", true},
		{"a", "app", false},
	}

	for _, tt := range tests {
		got := hasPrefix(tt.key, tt.prefix)
		assert.Equal(t, tt.want, got, "hasPrefix(%q, %q)", tt.key, tt.prefix)
	}
}

// --- v0.62.0 ConfigCenter Package Coverage Improvements ---

func TestMemoryBackend_Name(t *testing.T) {
	backend := NewMemoryBackend()
	defer backend.Close()

	name := backend.Name()
	assert.Equal(t, "memory", name)
}

func TestFileBackend_Name(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	require.NoError(t, err)
	defer backend.Close()

	name := backend.Name()
	assert.Equal(t, "file", name)
}

func TestFileBackend_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set a value
	entry, err := backend.Set(ctx, "test.key", "hello", TypeString)
	require.NoError(t, err)
	assert.Equal(t, "test.key", entry.Key)
	assert.Equal(t, "hello", entry.Value)

	// Get the value
	got, err := backend.Get(ctx, "test.key")
	require.NoError(t, err)
	assert.Equal(t, "hello", got.Value)

	// List
	entries, err := backend.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestFileBackend_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set and delete
	backend.Set(ctx, "del.key", "value", TypeString)
	err = backend.Delete(ctx, "del.key")
	require.NoError(t, err)

	_, err = backend.Get(ctx, "del.key")
	assert.Error(t, err)
}

func TestFileBackend_WatchAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Watch
	ch, err := backend.Watch(ctx, "")
	require.NoError(t, err)
	assert.NotNil(t, ch)

	// Close should not error
	err = backend.Close()
	assert.NoError(t, err)
}