package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yurika0211/luckyharness/internal/utils"
)

// LoadManifest 从文件加载插件清单
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	// 简单 YAML 解析（避免引入 yaml 依赖）
	return parseManifest(string(data))
}

// SaveManifest 保存插件清单到文件
func SaveManifest(manifest *Manifest, path string) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	content := formatManifest(manifest)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// parseManifest 简单 YAML 解析器
func parseManifest(content string) (*Manifest, error) {
	m := &Manifest{
		Permissions:  []Permission{},
		Dependencies: []Dependency{},
		Tags:         []string{},
		ConfigSchema: map[string]ConfigField{},
	}

	lines := utils.SplitLines(content)
	var currentSection string

	for _, line := range lines {
		trimmed := trimSpace(line)

		// 跳过空行和注释
		if trimmed == "" || hasPrefix(trimmed, "#") {
			continue
		}

		// Section headers
		if hasPrefix(trimmed, "dependencies:") {
			currentSection = "dependencies"
			continue
		}
		if hasPrefix(trimmed, "permissions:") {
			currentSection = "permissions"
			continue
		}
		if hasPrefix(trimmed, "tags:") {
			currentSection = "tags"
			continue
		}

		// List items
		if hasPrefix(trimmed, "- ") {
			item := trimPrefix(trimmed, "- ")
			switch currentSection {
			case "dependencies":
				dep := parseDependency(item)
				m.Dependencies = append(m.Dependencies, dep)
			case "permissions":
				m.Permissions = append(m.Permissions, Permission(trimSpace(item)))
			case "tags":
				m.Tags = append(m.Tags, trimSpace(item))
			}
			continue
		}

		// Key-value pairs
		key, value, ok := parseKV(trimmed)
		if !ok {
			continue
		}

		currentSection = "" // reset section on key-value

		switch key {
		case "name":
			m.Name = value
		case "version":
			m.Version = value
		case "author":
			m.Author = value
		case "description":
			m.Description = value
		case "license":
			m.License = value
		case "homepage":
			m.Homepage = value
		case "repository":
			m.Repository = value
		case "entry":
			m.Entry = value
		case "type":
			m.Type = value
		case "min_version":
			m.MinVersion = value
		}
	}

	return m, nil
}

// parseDependency 解析依赖条目 "name@version" 或 "name"
func parseDependency(s string) Dependency {
	parts := splitString(s, "@")
	dep := Dependency{Name: trimSpace(parts[0])}
	if len(parts) > 1 {
		dep.Version = trimSpace(parts[1])
	}
	return dep
}

// parseKV 解析 "key: value" 格式
func parseKV(s string) (string, string, bool) {
	idx := indexOfColon(s)
	if idx < 0 {
		return "", "", false
	}
	key := trimSpace(s[:idx])
	value := trimSpace(s[idx+1:])
	// 去除引号
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

// formatManifest 格式化清单为 YAML
func formatManifest(m *Manifest) string {
	var s string
	s += fmt.Sprintf("name: %s\n", m.Name)
	s += fmt.Sprintf("version: %s\n", m.Version)
	s += fmt.Sprintf("author: %s\n", m.Author)
	s += fmt.Sprintf("description: %s\n", m.Description)
	if m.License != "" {
		s += fmt.Sprintf("license: %s\n", m.License)
	}
	if m.Homepage != "" {
		s += fmt.Sprintf("homepage: %s\n", m.Homepage)
	}
	if m.Repository != "" {
		s += fmt.Sprintf("repository: %s\n", m.Repository)
	}
	s += fmt.Sprintf("entry: %s\n", m.Entry)
	s += fmt.Sprintf("type: %s\n", m.Type)
	if m.MinVersion != "" {
		s += fmt.Sprintf("min_version: %s\n", m.MinVersion)
	}

	if len(m.Tags) > 0 {
		s += "tags:\n"
		for _, tag := range m.Tags {
			s += fmt.Sprintf("  - %s\n", tag)
		}
	}

	if len(m.Dependencies) > 0 {
		s += "dependencies:\n"
		for _, dep := range m.Dependencies {
			if dep.Version != "" {
				s += fmt.Sprintf("  - %s@%s\n", dep.Name, dep.Version)
			} else {
				s += fmt.Sprintf("  - %s\n", dep.Name)
			}
		}
	}

	if len(m.Permissions) > 0 {
		s += "permissions:\n"
		for _, perm := range m.Permissions {
			s += fmt.Sprintf("  - %s\n", perm)
		}
	}

	return s
}

// --- string helpers (avoid importing strings for simple ops) ---

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimPrefix(s, prefix string) string {
	if hasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func indexOfColon(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

func splitString(s, sep string) []string {
	if len(sep) == 0 {
		return []string{s}
	}
	var parts []string
	for {
		idx := findString(s, sep)
		if idx < 0 {
			parts = append(parts, s)
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	return parts
}

func findString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
