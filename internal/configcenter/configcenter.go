// Package configcenter provides a centralized configuration management system
// with support for multiple backends (etcd, local file) and dynamic hot-reload.
package configcenter

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ValueType represents the type of a configuration value
type ValueType string

const (
	TypeString  ValueType = "string"
	TypeInt     ValueType = "int"
	TypeFloat   ValueType = "float"
	TypeBool    ValueType = "bool"
	TypeJSON    ValueType = "json"
)

// Entry represents a single configuration entry
type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Type      ValueType `json:"type"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChangeEvent represents a configuration change event
type ChangeEvent struct {
	Key     string    `json:"key"`
	OldVal  string    `json:"old_value,omitempty"`
	NewVal  string    `json:"new_value"`
	Type    ValueType `json:"type"`
	Version int64     `json:"version"`
}

// WatchCallback is called when a watched key changes
type WatchCallback func(event ChangeEvent)

// Backend is the interface for configuration storage backends
type Backend interface {
	// Name returns the backend name
	Name() string

	// Get retrieves a configuration entry by key
	Get(ctx context.Context, key string) (*Entry, error)

	// List retrieves all configuration entries matching prefix
	List(ctx context.Context, prefix string) ([]*Entry, error)

	// Set stores a configuration entry
	Set(ctx context.Context, key, value string, valType ValueType) (*Entry, error)

	// Delete removes a configuration entry
	Delete(ctx context.Context, key string) error

	// Watch watches for changes on keys matching prefix
	Watch(ctx context.Context, prefix string) (<-chan ChangeEvent, error)

	// Close cleans up backend resources
	Close() error
}

// Center is the configuration center that manages backends and dispatches events
type Center struct {
	mu       sync.RWMutex
	backends []Backend
	primary  Backend
	watchers map[string][]WatchCallback
	entries  map[string]*Entry
	closed   bool
	cancel   context.CancelFunc
}

// New creates a new configuration center with the given backends
// The first backend is the primary (write) backend; others are fallback (read-only)
func New(backends ...Backend) (*Center, error) {
	if len(backends) == 0 {
		return nil, fmt.Errorf("at least one backend is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Center{
		backends: backends,
		primary:  backends[0],
		watchers: make(map[string][]WatchCallback),
		entries:  make(map[string]*Entry),
		cancel:   cancel,
	}

	// Load initial entries from primary backend
	if err := c.loadAll(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("initial load: %w", err)
	}

	// Start watching primary backend for changes
	go c.watchLoop(ctx)

	return c, nil
}

// Get retrieves a configuration value by key
// Falls back to secondary backends if primary returns not found
func (c *Center) Get(ctx context.Context, key string) (*Entry, error) {
	c.mu.RLock()
	if entry, ok := c.entries[key]; ok {
		c.mu.RUnlock()
		return entry, nil
	}
	c.mu.RUnlock()

	// Try backends in order
	for _, b := range c.backends {
		entry, err := b.Get(ctx, key)
		if err == nil && entry != nil {
			c.mu.Lock()
			c.entries[key] = entry
			c.mu.Unlock()
			return entry, nil
		}
	}

	return nil, fmt.Errorf("key %q not found", key)
}

// GetString is a convenience method to get a string value
func (c *Center) GetString(ctx context.Context, key string) (string, error) {
	entry, err := c.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return entry.Value, nil
}

// GetDefault retrieves a value with a default fallback
func (c *Center) GetDefault(ctx context.Context, key, defaultVal string) string {
	entry, err := c.Get(ctx, key)
	if err != nil {
		return defaultVal
	}
	return entry.Value
}

// Set stores a configuration value in the primary backend
func (c *Center) Set(ctx context.Context, key, value string, valType ValueType) (*Entry, error) {
	if c.isClosed() {
		return nil, fmt.Errorf("config center is closed")
	}

	entry, err := c.primary.Set(ctx, key, value, valType)
	if err != nil {
		return nil, fmt.Errorf("set %q: %w", key, err)
	}

	// Update local cache (watchLoop will also update, but we need it here for Get)
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()

	return entry, nil
}

// Delete removes a configuration entry
func (c *Center) Delete(ctx context.Context, key string) error {
	if c.isClosed() {
		return fmt.Errorf("config center is closed")
	}

	if err := c.primary.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}

	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()

	return nil
}

// List retrieves all entries matching a prefix
func (c *Center) List(ctx context.Context, prefix string) ([]*Entry, error) {
	c.mu.RLock()
	var result []*Entry
	for _, entry := range c.entries {
		if prefix == "" || hasPrefix(entry.Key, prefix) {
			result = append(result, entry)
		}
	}
	c.mu.RUnlock()
	return result, nil
}

// Watch registers a callback for changes on keys matching the given prefix
func (c *Center) Watch(prefix string, callback WatchCallback) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("config center is closed")
	}

	c.watchers[prefix] = append(c.watchers[prefix], callback)
	return nil
}

// Unwatch removes all watchers for a given prefix
func (c *Center) Unwatch(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.watchers, prefix)
}

// Primary returns the primary backend
func (c *Center) Primary() Backend {
	return c.primary
}

// Backends returns all backends
func (c *Center) Backends() []Backend {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Backend, len(c.backends))
	copy(result, c.backends)
	return result
}

// Close shuts down the config center and all backends
func (c *Center) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	c.cancel()

	var firstErr error
	for _, b := range c.backends {
		if err := b.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// loadAll loads all entries from the primary backend into the local cache
func (c *Center) loadAll(ctx context.Context) error {
	entries, err := c.primary.List(ctx, "")
	if err != nil {
		return err
	}

	c.mu.Lock()
	for _, entry := range entries {
		c.entries[entry.Key] = entry
	}
	c.mu.Unlock()

	return nil
}

// watchLoop watches the primary backend for changes
func (c *Center) watchLoop(ctx context.Context) {
	ch, err := c.primary.Watch(ctx, "")
	if err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			c.handleWatchEvent(event)
		}
	}
}

// handleWatchEvent processes a change event from the backend
func (c *Center) handleWatchEvent(event ChangeEvent) {
	c.mu.Lock()
	// Update local cache
	if event.NewVal != "" {
		c.entries[event.Key] = &Entry{
			Key:       event.Key,
			Value:     event.NewVal,
			Type:      event.Type,
			Version:   event.Version,
			UpdatedAt: time.Now(),
		}
	} else {
		delete(c.entries, event.Key)
	}
	callbacks := c.matchCallbacks(event.Key)
	c.mu.Unlock()

	c.notifyWatchers(callbacks, event)
}

// matchCallbacks returns all callbacks that match the given key
func (c *Center) matchCallbacks(key string) []WatchCallback {
	var matched []WatchCallback
	for prefix, cbs := range c.watchers {
		if prefix == "" || hasPrefix(key, prefix) {
			matched = append(matched, cbs...)
		}
	}
	return matched
}

// notifyWatchers calls all matched callbacks with the event
func (c *Center) notifyWatchers(callbacks []WatchCallback, event ChangeEvent) {
	for _, cb := range callbacks {
		// Call in goroutine to avoid blocking
		go func(fn WatchCallback) {
			fn(event)
		}(cb)
	}
}

// isClosed checks if the center is closed
func (c *Center) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// hasPrefix checks if a key has the given prefix (with "/" separator)
func hasPrefix(key, prefix string) bool {
	if prefix == "" {
		return true
	}
	if len(key) < len(prefix) {
		return false
	}
	return key[:len(prefix)] == prefix
}