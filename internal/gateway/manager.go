package gateway

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// GatewayStats tracks per-gateway statistics.
type GatewayStats struct {
	MessagesSent     int64
	MessagesReceived int64
	Errors           int64
}

// GatewayManager manages multiple platform gateway adapters.
type GatewayManager struct {
	mu       sync.RWMutex
	gateways map[string]Gateway
	stats    map[string]*GatewayStats
	handler  MessageHandler
	running  atomic.Bool
}

// NewGatewayManager creates a new GatewayManager.
func NewGatewayManager() *GatewayManager {
	return &GatewayManager{
		gateways: make(map[string]Gateway),
		stats:    make(map[string]*GatewayStats),
	}
}

// Register adds a gateway adapter to the manager.
func (gm *GatewayManager) Register(gw Gateway) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	name := gw.Name()
	if _, exists := gm.gateways[name]; exists {
		return fmt.Errorf("gateway %q already registered", name)
	}

	gm.gateways[name] = gw
	gm.stats[name] = &GatewayStats{}
	return nil
}

// Unregister removes a gateway adapter from the manager.
// It stops the gateway first if it is running.
func (gm *GatewayManager) Unregister(name string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	gw, exists := gm.gateways[name]
	if !exists {
		return fmt.Errorf("gateway %q not found", name)
	}

	if gw.IsRunning() {
		if err := gw.Stop(); err != nil {
			return fmt.Errorf("stop gateway %q: %w", name, err)
		}
	}

	delete(gm.gateways, name)
	delete(gm.stats, name)
	return nil
}

// OnMessage registers the handler that is called when any gateway receives a message.
func (gm *GatewayManager) OnMessage(handler MessageHandler) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.handler = handler
}

// handleMessage is called by gateways when they receive a message.
func (gm *GatewayManager) handleMessage(ctx context.Context, gwName string, msg *Message) error {
	gm.mu.RLock()
	stats, ok := gm.stats[gwName]
	gm.mu.RUnlock()

	if ok {
		atomic.AddInt64(&stats.MessagesReceived, 1)
	}

	gm.mu.RLock()
	handler := gm.handler
	gm.mu.RUnlock()

	if handler == nil {
		return nil
	}

	return handler(ctx, msg)
}

// RecordSent increments the sent counter for a gateway.
func (gm *GatewayManager) RecordSent(gwName string) {
	gm.mu.RLock()
	stats, ok := gm.stats[gwName]
	gm.mu.RUnlock()

	if ok {
		atomic.AddInt64(&stats.MessagesSent, 1)
	}
}

// RecordError increments the error counter for a gateway.
func (gm *GatewayManager) RecordError(gwName string) {
	gm.mu.RLock()
	stats, ok := gm.stats[gwName]
	gm.mu.RUnlock()

	if ok {
		atomic.AddInt64(&stats.Errors, 1)
	}
}

// StartAll starts all registered gateways.
func (gm *GatewayManager) StartAll(ctx context.Context) error {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	for name, gw := range gm.gateways {
		if err := gw.Start(ctx); err != nil {
			return fmt.Errorf("start gateway %q: %w", name, err)
		}
	}

	gm.running.Store(true)
	return nil
}

// Start starts a specific gateway by name.
func (gm *GatewayManager) Start(ctx context.Context, name string) error {
	gm.mu.RLock()
	gw, exists := gm.gateways[name]
	gm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("gateway %q not found", name)
	}

	if err := gw.Start(ctx); err != nil {
		return fmt.Errorf("start gateway %q: %w", name, err)
	}

	return nil
}

// StopAll stops all running gateways.
func (gm *GatewayManager) StopAll() error {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var firstErr error
	for name, gw := range gm.gateways {
		if gw.IsRunning() {
			if err := gw.Stop(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("stop gateway %q: %w", name, err)
			}
		}
	}

	gm.running.Store(false)
	return firstErr
}

// Stop stops a specific gateway by name.
func (gm *GatewayManager) Stop(name string) error {
	gm.mu.RLock()
	gw, exists := gm.gateways[name]
	gm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("gateway %q not found", name)
	}

	return gw.Stop()
}

// Get returns a gateway by name.
func (gm *GatewayManager) Get(name string) (Gateway, bool) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	gw, ok := gm.gateways[name]
	return gw, ok
}

// List returns the names of all registered gateways.
func (gm *GatewayManager) List() []string {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	names := make([]string, 0, len(gm.gateways))
	for name := range gm.gateways {
		names = append(names, name)
	}
	return names
}

// Stats returns the statistics for a specific gateway.
func (gm *GatewayManager) Stats(name string) (GatewayStats, bool) {
	gm.mu.RLock()
	stats, ok := gm.stats[name]
	gm.mu.RUnlock()

	if !ok {
		return GatewayStats{}, false
	}

	return GatewayStats{
		MessagesSent:     atomic.LoadInt64(&stats.MessagesSent),
		MessagesReceived: atomic.LoadInt64(&stats.MessagesReceived),
		Errors:           atomic.LoadInt64(&stats.Errors),
	}, true
}

// AllStats returns statistics for all gateways.
func (gm *GatewayManager) AllStats() map[string]GatewayStats {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	result := make(map[string]GatewayStats, len(gm.stats))
	for name, stats := range gm.stats {
		result[name] = GatewayStats{
			MessagesSent:     atomic.LoadInt64(&stats.MessagesSent),
			MessagesReceived: atomic.LoadInt64(&stats.MessagesReceived),
			Errors:           atomic.LoadInt64(&stats.Errors),
		}
	}
	return result
}

// IsRunning returns whether the manager has started gateways.
func (gm *GatewayManager) IsRunning() bool {
	return gm.running.Load()
}

// GatewayStatus represents the status of a single gateway.
type GatewayStatus struct {
	Name      string       `json:"name"`
	Running   bool         `json:"running"`
	Stats     GatewayStats `json:"stats"`
}

// Status returns the status of all gateways.
func (gm *GatewayManager) Status() []GatewayStatus {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	statuses := make([]GatewayStatus, 0, len(gm.gateways))
	for name, gw := range gm.gateways {
		stats, _ := gm.Stats(name)
		statuses = append(statuses, GatewayStatus{
			Name:    name,
			Running: gw.IsRunning(),
			Stats:   stats,
		})
	}
	return statuses
}