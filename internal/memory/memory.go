package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry 代表一条记忆
type Entry struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
}

// Store 管理持久记忆
type Store struct {
	mu      sync.RWMutex
	entries []Entry
	dir     string
}

// NewStore 创建记忆存储
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create memory dir: %w", err)
	}
	s := &Store{
		dir: dir,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Save 保存一条记忆
func (s *Store) Save(content, category string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := Entry{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Content:   content,
		Category:  category,
		CreatedAt: time.Now(),
	}
	s.entries = append(s.entries, entry)
	return s.persist()
}

// Search 搜索记忆
func (s *Store) Search(query string) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Entry
	for _, e := range s.entries {
		if contains(e.Content, query) || contains(e.Category, query) {
			results = append(results, e)
		}
	}
	return results
}

// Recent 返回最近的 N 条记忆
func (s *Store) Recent(n int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n > len(s.entries) {
		n = len(s.entries)
	}
	results := make([]Entry, n)
	copy(results, s.entries[len(s.entries)-n:])
	return results
}

func (s *Store) load() error {
	// v0.1.0: 简单的行式存储
	path := filepath.Join(s.dir, "memory.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load memory: %w", err)
	}

	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		s.entries = append(s.entries, Entry{
			Content: line,
		})
	}
	return nil
}

func (s *Store) persist() error {
	path := filepath.Join(s.dir, "memory.txt")
	var data string
	for _, e := range s.entries {
		data += e.Content + "\n"
	}
	return os.WriteFile(path, []byte(data), 0600)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
