package configcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// FileBackend implements Backend using local filesystem
type FileBackend struct {
	mu      sync.RWMutex
	dir     string
	entries map[string]*Entry
	version int64
	watchCh chan ChangeEvent
	cancel  context.CancelFunc
}

// NewFileBackend creates a new file-based configuration backend
func NewFileBackend(dir string) (*FileBackend, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fb := &FileBackend{
		dir:     dir,
		entries: make(map[string]*Entry),
		watchCh: make(chan ChangeEvent, 256),
		cancel:  cancel,
	}

	// Load existing entries
	if err := fb.loadAll(); err != nil {
		cancel()
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Start file watcher
	go fb.watchFiles(ctx)

	return fb, nil
}

// Name returns the backend name
func (fb *FileBackend) Name() string {
	return "file"
}

// Get retrieves a configuration entry by key
func (fb *FileBackend) Get(ctx context.Context, key string) (*Entry, error) {
	fb.mu.RLock()
	defer fb.mu.RUnlock()

	entry, ok := fb.entries[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	cp := *entry
	return &cp, nil
}

// List retrieves all entries matching prefix
func (fb *FileBackend) List(ctx context.Context, prefix string) ([]*Entry, error) {
	fb.mu.RLock()
	defer fb.mu.RUnlock()

	var result []*Entry
	for _, entry := range fb.entries {
		if prefix == "" || hasPrefix(entry.Key, prefix) {
			cp := *entry
			result = append(result, &cp)
		}
	}
	return result, nil
}

// Set stores a configuration entry
func (fb *FileBackend) Set(ctx context.Context, key, value string, valType ValueType) (*Entry, error) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	ver := atomic.AddInt64(&fb.version, 1)
	entry := &Entry{
		Key:       key,
		Value:     value,
		Type:      valType,
		Version:   ver,
		UpdatedAt: time.Now(),
	}

	fb.entries[key] = entry

	// Persist to disk
	if err := fb.persistEntry(entry); err != nil {
		return nil, fmt.Errorf("persist %q: %w", key, err)
	}

	return entry, nil
}

// Delete removes a configuration entry
func (fb *FileBackend) Delete(ctx context.Context, key string) error {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if _, ok := fb.entries[key]; !ok {
		return fmt.Errorf("key %q not found", key)
	}

	delete(fb.entries, key)

	// Remove file
	filePath := fb.keyToPath(key)
	os.Remove(filePath)

	return nil
}

// Watch watches for changes on keys matching prefix
func (fb *FileBackend) Watch(ctx context.Context, prefix string) (<-chan ChangeEvent, error) {
	// Return the shared watch channel; filtering is done by the Center
	return fb.watchCh, nil
}

// Close cleans up backend resources
func (fb *FileBackend) Close() error {
	fb.cancel()
	close(fb.watchCh)
	return nil
}

// keyToPath converts a config key to a file path
func (fb *FileBackend) keyToPath(key string) string {
	return filepath.Join(fb.dir, key+".json")
}

// persistEntry writes an entry to disk
func (fb *FileBackend) persistEntry(entry *Entry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fb.keyToPath(entry.Key), data, 0644)
}

// loadAll loads all entries from disk
func (fb *FileBackend) loadAll() error {
	entries, err := os.ReadDir(fb.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(fb.dir, e.Name()))
		if err != nil {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		fb.entries[entry.Key] = &entry
		if entry.Version > fb.version {
			fb.version = entry.Version
		}
	}

	return nil
}

// watchFiles polls the filesystem for changes
func (fb *FileBackend) watchFiles(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastMod := make(map[string]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fb.checkFileChanges(lastMod)
		}
	}
}

// checkFileChanges detects file modifications and emits events
func (fb *FileBackend) checkFileChanges(lastMod map[string]time.Time) {
	entries, err := os.ReadDir(fb.dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		path := e.Name()
		modTime := info.ModTime()

		if prev, ok := lastMod[path]; ok && modTime.After(prev) {
			// File was modified externally
			data, err := os.ReadFile(filepath.Join(fb.dir, path))
			if err != nil {
				continue
			}

			var entry Entry
			if err := json.Unmarshal(data, &entry); err != nil {
				continue
			}

			fb.mu.Lock()
			oldEntry, existed := fb.entries[entry.Key]
			fb.entries[entry.Key] = &entry
			fb.mu.Unlock()

			event := ChangeEvent{
				Key:     entry.Key,
				NewVal:  entry.Value,
				Type:    entry.Type,
				Version: entry.Version,
			}
			if existed {
				event.OldVal = oldEntry.Value
			}

			select {
			case fb.watchCh <- event:
			default:
				// Channel full, drop event
			}
		}

		lastMod[path] = modTime
	}
}