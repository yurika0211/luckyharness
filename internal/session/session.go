package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yurika0211/luckyharness/internal/provider"
)

// Session 代表一次对话会话
type Session struct {
	mu        sync.RWMutex
	ID        string
	Title     string
	Messages  []provider.Message
	CreatedAt time.Time
	UpdatedAt time.Time
	dir       string
}

// NewSession 创建新会话
func NewSession(id, dir string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Messages:  make([]provider.Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
		dir:       dir,
	}
}

// AddMessage 添加消息
func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, provider.Message{Role: role, Content: content})
	s.UpdatedAt = time.Now()

	// 自动生成标题：取第一条用户消息的前 50 字符
	if s.Title == "" && role == "user" {
		title := content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		s.Title = title
	}
}

// AddToolMessage 添加工具结果消息
func (s *Session) AddToolMessage(toolName, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, provider.Message{
		Role:    "tool",
		Content: fmt.Sprintf("[Tool: %s] %s", toolName, result),
	})
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

// MessageCount 返回消息数量
func (s *Session) MessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Messages)
}

// Save 保存会话到磁盘 (JSON 格式)
func (s *Session) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	data := sessionData{
		ID:        s.ID,
		Title:     s.Title,
		Messages:  s.Messages,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(s.dir, s.ID+".json")
	return os.WriteFile(path, jsonData, 0600)
}

// sessionData 是 JSON 序列化格式
type sessionData struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Messages  []provider.Message `json:"messages"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// SessionInfo 是会话的摘要信息（用于列表展示）
type SessionInfo struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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

	m := &Manager{
		sessions: make(map[string]*Session),
		dir:      dir,
	}

	// 加载已有会话
	if err := m.loadFromDisk(); err != nil {
		// 非致命错误：加载失败时继续使用空 map
		fmt.Printf("[session] warning: failed to load sessions from disk: %v\n", err)
	}

	return m, nil
}

// loadFromDisk 从磁盘加载所有会话
func (m *Manager) loadFromDisk() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read sessions dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(m.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // 跳过无法读取的文件
		}

		var sd sessionData
		if err := json.Unmarshal(data, &sd); err != nil {
			continue // 跳过无法解析的文件
		}

		s := &Session{
			ID:        sd.ID,
			Title:     sd.Title,
			Messages:  sd.Messages,
			CreatedAt: sd.CreatedAt,
			UpdatedAt: sd.UpdatedAt,
			dir:       m.dir,
		}
		m.sessions[s.ID] = s
	}

	return nil
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

// NewWithTitle 创建带标题的新会话
func (m *Manager) NewWithTitle(title string) *Session {
	s := m.New()
	s.mu.Lock()
	s.Title = title
	s.mu.Unlock()
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

// ListInfo 列出所有会话的摘要信息（按更新时间排序）
func (m *Manager) ListInfo() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.mu.RLock()
		infos = append(infos, SessionInfo{
			ID:           s.ID,
			Title:        s.Title,
			MessageCount: len(s.Messages),
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
		})
		s.mu.RUnlock()
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt.After(infos[j].UpdatedAt)
	})

	return infos
}

// Search 搜索包含关键词的会话
func (m *Manager) Search(query string) []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []SessionInfo
	lowerQuery := strings.ToLower(query)

	for _, s := range m.sessions {
		s.mu.RLock()
		// 搜索标题
		if strings.Contains(strings.ToLower(s.Title), lowerQuery) {
			results = append(results, SessionInfo{
				ID:           s.ID,
				Title:        s.Title,
				MessageCount: len(s.Messages),
				CreatedAt:    s.CreatedAt,
				UpdatedAt:    s.UpdatedAt,
			})
			s.mu.RUnlock()
			continue
		}

		// 搜索消息内容
		for _, msg := range s.Messages {
			if strings.Contains(strings.ToLower(msg.Content), lowerQuery) {
				results = append(results, SessionInfo{
					ID:           s.ID,
					Title:        s.Title,
					MessageCount: len(s.Messages),
					CreatedAt:    s.CreatedAt,
					UpdatedAt:    s.UpdatedAt,
				})
				break
			}
		}
		s.mu.RUnlock()
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})

	return results
}

// Delete 删除会话
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	// 删除磁盘文件
	path := filepath.Join(s.dir, s.ID+".json")
	os.Remove(path) // 忽略错误，文件可能不存在

	delete(m.sessions, id)
	return nil
}

// SaveAll 保存所有会话到磁盘
func (m *Manager) SaveAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error
	for _, s := range m.sessions {
		if err := s.Save(); err != nil {
			errs = append(errs, fmt.Errorf("save session %s: %w", s.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("save errors: %v", errs)
	}
	return nil
}

// Count 返回会话数量
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}