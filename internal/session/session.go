package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yurika0211/luckyharness/internal/provider"
)

// Session 代表一次对话会话
type Session struct {
	mu        sync.RWMutex
	ID        string
	Messages  []provider.Message
	CreatedAt time.Time
	UpdatedAt time.Time
	dir       string
}

// NewSession 创建新会话
func NewSession(id, dir string) *Session {
	return &Session{
		ID:        id,
		Messages:  make([]provider.Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		dir:       dir,
	}
}

// AddMessage 添加消息
func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, provider.Message{Role: role, Content: content})
	s.UpdatedAt = time.Now()
}

// GetMessages 获取所有消息
func (s *Session) GetMessages() []provider.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]provider.Message, len(s.Messages))
	copy(cp, s.Messages)
	return cp
}

// LastMessage 获取最后一条消息
func (s *Session) LastMessage() *provider.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Messages) == 0 {
		return nil
	}
	m := s.Messages[len(s.Messages)-1]
	return &m
}

// Save 保存会话到磁盘
func (s *Session) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	path := filepath.Join(s.dir, s.ID+".txt")
	var data string
	for _, m := range s.Messages {
		data += fmt.Sprintf("[%s] %s\n", m.Role, m.Content)
	}
	return os.WriteFile(path, []byte(data), 0600)
}

// Manager 管理多个会话
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	dir      string
}

// NewManager 创建会话管理器
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}
	return &Manager{
		sessions: make(map[string]*Session),
		dir:      dir,
	}, nil
}

// New 创建新会话
func (m *Manager) New() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	s := NewSession(id, m.dir)
	m.sessions[id] = s
	return s
}

// Get 获取会话
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// List 列出所有会话
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}
