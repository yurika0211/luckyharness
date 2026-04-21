package configcenter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryBackend implements Backend using in-memory storage (for testing)
type MemoryBackend struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	version int64
	watchCh chan ChangeEvent
	closed  bool
}

// NewMemoryBackend creates a new in-memory configuration backend
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		entries: make(map[string]*Entry),
		watchCh: make(chan ChangeEvent, 256),
	}
}

// Name returns the backend name
func (mb *MemoryBackend) Name() string {
	return "memory"
}

// Get retrieves a configuration entry by key
func (mb *MemoryBackend) Get(ctx context.Context, key string) (*Entry, error) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	entry, ok := mb.entries[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	cp := *entry
	return &cp, nil
}

// List retrieves all entries matching prefix
func (mb *MemoryBackend) List(ctx context.Context, prefix string) ([]*Entry, error) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	var result []*Entry
	for _, entry := range mb.entries {
		if prefix == "" || hasPrefix(entry.Key, prefix) {
			cp := *entry
			result = append(result, &cp)
		}
	}
	return result, nil
}

// Set stores a configuration entry
func (mb *MemoryBackend) Set(ctx context.Context, key, value string, valType ValueType) (*Entry, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	ver := atomic.AddInt64(&mb.version, 1)
	entry := &Entry{
		Key:       key,
		Value:     value,
		Type:      valType,
		Version:   ver,
		UpdatedAt: time.Now(),
	}

	oldEntry, existed := mb.entries[key]
	mb.entries[key] = entry

	// Emit change event
	event := ChangeEvent{
		Key:     key,
		NewVal:  value,
		Type:    valType,
		Version: ver,
	}
	if existed {
		event.OldVal = oldEntry.Value
	}

	select {
	case mb.watchCh <- event:
	default:
	}

	return entry, nil
}

// Delete removes a configuration entry
func (mb *MemoryBackend) Delete(ctx context.Context, key string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, ok := mb.entries[key]; !ok {
		return fmt.Errorf("key %q not found", key)
	}

	delete(mb.entries, key)
	return nil
}

// Watch watches for changes on keys matching prefix
func (mb *MemoryBackend) Watch(ctx context.Context, prefix string) (<-chan ChangeEvent, error) {
	return mb.watchCh, nil
}

// Close cleans up backend resources
func (mb *MemoryBackend) Close() error {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if !mb.closed {
		mb.closed = true
		close(mb.watchCh)
	}
	return nil
}