package collab

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AgentStatus Agent 状态
type AgentStatus string

const (
	StatusOnline  AgentStatus = "online"
	StatusBusy    AgentStatus = "busy"
	StatusOffline AgentStatus = "offline"
)

// AgentProfile Agent 注册信息
type AgentProfile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Capabilities []string         `json:"capabilities"`  // 能力标签列表
	Status      AgentStatus       `json:"status"`
	LastSeen    time.Time         `json:"last_seen"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Registry Agent 注册中心
type Registry struct {
	mu      sync.RWMutex
	agents  map[string]*AgentProfile
	hooks   []RegistryHook // 注册/注销钩子
}

// RegistryHook 注册中心钩子
type RegistryHook struct {
	OnRegister   func(profile *AgentProfile)
	OnDeregister func(agentID string)
	OnStatusChange func(agentID string, oldStatus, newStatus AgentStatus)
}

// NewRegistry 创建注册中心
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*AgentProfile),
	}
}

// Register 注册 Agent
func (r *Registry) Register(profile *AgentProfile) error {
	if profile.ID == "" {
		return fmt.Errorf("agent ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	profile.LastSeen = time.Now()
	if profile.Status == "" {
		profile.Status = StatusOnline
	}
	if profile.Metadata == nil {
		profile.Metadata = make(map[string]string)
	}

	r.agents[profile.ID] = profile

	// 触发钩子
	for _, hook := range r.hooks {
		if hook.OnRegister != nil {
			hook.OnRegister(profile)
		}
	}

	return nil
}

// Deregister 注销 Agent
func (r *Registry) Deregister(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.agents[agentID]; !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	delete(r.agents, agentID)

	// 触发钩子
	for _, hook := range r.hooks {
		if hook.OnDeregister != nil {
			hook.OnDeregister(agentID)
		}
	}

	return nil
}

// Get 获取 Agent 信息
func (r *Registry) Get(agentID string) (*AgentProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.agents[agentID]
	if !ok {
		return nil, false
	}
	// 返回副本
	cp := *p
	return &cp, true
}

// List 列出所有 Agent
func (r *Registry) List() []*AgentProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*AgentProfile, 0, len(r.agents))
	for _, p := range r.agents {
		cp := *p
		result = append(result, &cp)
	}
	return result
}

// ListByCapability 按能力筛选 Agent
func (r *Registry) ListByCapability(capability string) []*AgentProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AgentProfile
	for _, p := range r.agents {
		for _, cap := range p.Capabilities {
			if cap == capability {
				cp := *p
				result = append(result, &cp)
				break
			}
		}
	}
	return result
}

// ListOnline 列出在线 Agent
func (r *Registry) ListOnline() []*AgentProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AgentProfile
	for _, p := range r.agents {
		if p.Status == StatusOnline {
			cp := *p
			result = append(result, &cp)
		}
	}
	return result
}

// UpdateStatus 更新 Agent 状态
func (r *Registry) UpdateStatus(agentID string, status AgentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	oldStatus := p.Status
	p.Status = status
	p.LastSeen = time.Now()

	// 触发钩子
	for _, hook := range r.hooks {
		if hook.OnStatusChange != nil {
			hook.OnStatusChange(agentID, oldStatus, status)
		}
	}

	return nil
}

// Heartbeat 心跳更新
func (r *Registry) Heartbeat(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	p.LastSeen = time.Now()
	if p.Status == StatusOffline {
		p.Status = StatusOnline
	}
	return nil
}

// HealthCheck 健康检查 — 标记超时 Agent 为离线
func (r *Registry) HealthCheck(timeout time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	stale := 0
	now := time.Now()
	for _, p := range r.agents {
		if now.Sub(p.LastSeen) > timeout {
			if p.Status != StatusOffline {
				p.Status = StatusOffline
				stale++
			}
		}
	}
	return stale
}

// AddHook 添加钩子
func (r *Registry) AddHook(hook RegistryHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, hook)
}

// Count 统计 Agent 数量
func (r *Registry) Count() (total, online, busy, offline int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.agents {
		total++
		switch p.Status {
		case StatusOnline:
			online++
		case StatusBusy:
			busy++
		case StatusOffline:
			offline++
		}
	}
	return
}

// StartHealthChecker 启动定期健康检查
func (r *Registry) StartHealthChecker(ctx context.Context, interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.HealthCheck(timeout)
			}
		}
	}()
}