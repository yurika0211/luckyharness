//go:build etcd

package configcenter

import (
	"context"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdBackend implements Backend using etcd as the storage backend
type EtcdBackend struct {
	mu      sync.RWMutex
	client  *clientv3.Client
	prefix  string
	entries map[string]*Entry
	watchCh chan ChangeEvent
	closed  bool
}

// EtcdConfig holds etcd connection configuration
type EtcdConfig struct {
	Endpoints   []string      `yaml:"endpoints"`
	DialTimeout time.Duration `yaml:"dial_timeout"`
	Username    string        `yaml:"username,omitempty"`
	Password    string        `yaml:"password,omitempty"`
	Prefix      string        `yaml:"prefix,omitempty"`
}

// NewEtcdBackend creates a new etcd-based configuration backend
func NewEtcdBackend(cfg EtcdConfig) (*EtcdBackend, error) {
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "/luckyharness/config/"
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to etcd: %w", err)
	}

	eb := &EtcdBackend{
		client:  client,
		prefix:  cfg.Prefix,
		entries: make(map[string]*Entry),
		watchCh: make(chan ChangeEvent, 256),
	}

	// Load initial entries
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := eb.loadAll(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("load from etcd: %w", err)
	}

	return eb, nil
}

// Name returns the backend name
func (eb *EtcdBackend) Name() string {
	return "etcd"
}

// Get retrieves a configuration entry by key
func (eb *EtcdBackend) Get(ctx context.Context, key string) (*Entry, error) {
	eb.mu.RLock()
	entry, ok := eb.entries[key]
	eb.mu.RUnlock()

	if ok {
		cp := *entry
		return &cp, nil
	}

	// Try etcd directly
	etcdKey := eb.prefix + key
	resp, err := eb.client.Get(ctx, etcdKey)
	if err != nil {
		return nil, fmt.Errorf("etcd get %q: %w", key, err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("key %q not found", key)
	}

	kv := resp.Kvs[0]
	entry = &Entry{
		Key:       key,
		Value:     string(kv.Value),
		Type:      TypeString,
		Version:   int64(kv.ModRevision),
		UpdatedAt: time.Unix(0, kv.CreateRevision),
	}

	eb.mu.Lock()
	eb.entries[key] = entry
	eb.mu.Unlock()

	cp := *entry
	return &cp, nil
}

// List retrieves all entries matching prefix
func (eb *EtcdBackend) List(ctx context.Context, prefix string) ([]*Entry, error) {
	eb.mu.RLock()
	var result []*Entry
	for _, entry := range eb.entries {
		if prefix == "" || hasPrefix(entry.Key, prefix) {
			cp := *entry
			result = append(result, &cp)
		}
	}
	eb.mu.RUnlock()
	return result, nil
}

// Set stores a configuration entry in etcd
func (eb *EtcdBackend) Set(ctx context.Context, key, value string, valType ValueType) (*Entry, error) {
	etcdKey := eb.prefix + key

	resp, err := eb.client.Put(ctx, etcdKey, value)
	if err != nil {
		return nil, fmt.Errorf("etcd put %q: %w", key, err)
	}

	entry := &Entry{
		Key:       key,
		Value:     value,
		Type:      valType,
		Version:   resp.Header.Revision,
		UpdatedAt: time.Now(),
	}

	eb.mu.Lock()
	eb.entries[key] = entry
	eb.mu.Unlock()

	return entry, nil
}

// Delete removes a configuration entry from etcd
func (eb *EtcdBackend) Delete(ctx context.Context, key string) error {
	etcdKey := eb.prefix + key

	_, err := eb.client.Delete(ctx, etcdKey)
	if err != nil {
		return fmt.Errorf("etcd delete %q: %w", key, err)
	}

	eb.mu.Lock()
	delete(eb.entries, key)
	eb.mu.Unlock()

	return nil
}

// Watch watches for changes on keys matching prefix using etcd watch
func (eb *EtcdBackend) Watch(ctx context.Context, prefix string) (<-chan ChangeEvent, error) {
	// Start etcd watch in background
	etcdPrefix := eb.prefix
	if prefix != "" {
		etcdPrefix = eb.prefix + prefix
	}

	watchCh := eb.client.Watch(ctx, etcdPrefix, clientv3.WithPrefix())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case resp, ok := <-watchCh:
				if !ok {
					return
				}
				for _, ev := range resp.Events {
					key := string(ev.Kv.Key)[len(eb.prefix):]
					event := ChangeEvent{
						Key:     key,
						Version: int64(ev.Kv.ModRevision),
						Type:    TypeString,
					}

					switch ev.Type {
					case clientv3.EventTypePut:
						event.NewVal = string(ev.Kv.Value)
					case clientv3.EventTypeDelete:
						// Key deleted, NewVal is empty
					}

					select {
					case eb.watchCh <- event:
					default:
					}
				}
			}
		}
	}()

	return eb.watchCh, nil
}

// Close cleans up etcd resources
func (eb *EtcdBackend) Close() error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if !eb.closed {
		eb.closed = true
		close(eb.watchCh)
		return eb.client.Close()
	}
	return nil
}

// loadAll loads all entries from etcd
func (eb *EtcdBackend) loadAll(ctx context.Context) error {
	resp, err := eb.client.Get(ctx, eb.prefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("etcd get all: %w", err)
	}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)[len(eb.prefix):]
		eb.entries[key] = &Entry{
			Key:       key,
			Value:     string(kv.Value),
			Type:      TypeString,
			Version:   int64(kv.ModRevision),
			UpdatedAt: time.Now(),
		}
	}

	return nil
}