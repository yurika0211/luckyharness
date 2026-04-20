package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	content := `name: my-plugin
version: 1.2.3
author: test-author
description: A test plugin for testing
license: MIT
homepage: https://example.com
repository: https://github.com/example/my-plugin
entry: main.go
type: skill
min_version: 0.14.0
tags:
  - testing
  - demo
dependencies:
  - base-plugin@1.0.0
permissions:
  - filesystem
  - network
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plugin.yaml")
	os.WriteFile(path, []byte(content), 0644)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if m.Name != "my-plugin" {
		t.Errorf("Name = %v, want my-plugin", m.Name)
	}
	if m.Version != "1.2.3" {
		t.Errorf("Version = %v, want 1.2.3", m.Version)
	}
	if m.Author != "test-author" {
		t.Errorf("Author = %v, want test-author", m.Author)
	}
	if m.Type != "skill" {
		t.Errorf("Type = %v, want skill", m.Type)
	}
	if m.Entry != "main.go" {
		t.Errorf("Entry = %v, want main.go", m.Entry)
	}
	if m.License != "MIT" {
		t.Errorf("License = %v, want MIT", m.License)
	}
	if m.MinVersion != "0.14.0" {
		t.Errorf("MinVersion = %v, want 0.14.0", m.MinVersion)
	}
	if len(m.Tags) != 2 || m.Tags[0] != "testing" || m.Tags[1] != "demo" {
		t.Errorf("Tags = %v, want [testing, demo]", m.Tags)
	}
	if len(m.Dependencies) != 1 || m.Dependencies[0].Name != "base-plugin" {
		t.Errorf("Dependencies = %v, want [base-plugin@1.0.0]", m.Dependencies)
	}
	if len(m.Permissions) != 2 {
		t.Errorf("Permissions = %v, want 2 items", m.Permissions)
	}
}

func TestSaveManifest(t *testing.T) {
	m := &Manifest{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Author:      "test-author",
		Description: "A test plugin",
		Entry:       "main.go",
		Type:        "skill",
		Tags:        []string{"test"},
		Permissions: []Permission{PermFileSystem},
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plugin.yaml")

	if err := SaveManifest(m, path); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("SaveManifest() did not create file")
	}

	// 重新加载验证
	m2, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() after save error = %v", err)
	}
	if m2.Name != m.Name {
		t.Errorf("Name = %v, want %v", m2.Name, m.Name)
	}
	if m2.Version != m.Version {
		t.Errorf("Version = %v, want %v", m2.Version, m.Version)
	}
}

func TestSaveManifestInvalid(t *testing.T) {
	m := &Manifest{
		// Name is missing
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plugin.yaml")

	if err := SaveManifest(m, path); err == nil {
		t.Error("SaveManifest() should fail for invalid manifest")
	}
}

func TestLoadManifestNonexistent(t *testing.T) {
	_, err := LoadManifest("/nonexistent/plugin.yaml")
	if err == nil {
		t.Error("LoadManifest() should fail for nonexistent file")
	}
}

func TestFormatManifest(t *testing.T) {
	m := &Manifest{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Author:      "test-author",
		Description: "A test plugin",
		Entry:       "main.go",
		Type:        "skill",
		Tags:        []string{"test", "demo"},
		Dependencies: []Dependency{
			{Name: "base", Version: "1.0.0"},
		},
		Permissions: []Permission{PermFileSystem, PermNetwork},
	}

	output := formatManifest(m)

	// 验证关键行
	if !containsString(output, "name: test-plugin") {
		t.Error("formatManifest() missing name")
	}
	if !containsString(output, "version: 1.0.0") {
		t.Error("formatManifest() missing version")
	}
	if !containsString(output, "type: skill") {
		t.Error("formatManifest() missing type")
	}
	if !containsString(output, "- filesystem") {
		t.Error("formatManifest() missing filesystem permission")
	}
	if !containsString(output, "- base@1.0.0") {
		t.Error("formatManifest() missing dependency")
	}
}

func TestParseKV(t *testing.T) {
	tests := []struct {
		input    string
		wantKey  string
		wantVal  string
		wantOk   bool
	}{
		{"name: my-plugin", "name", "my-plugin", true},
		{"version: 1.0.0", "version", "1.0.0", true},
		{`description: "A test plugin"`, "description", "A test plugin", true},
		{"no-colon", "", "", false},
		{"  spaced  :  value  ", "spaced", "value", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			key, val, ok := parseKV(tt.input)
			if ok != tt.wantOk {
				t.Errorf("parseKV() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && (key != tt.wantKey || val != tt.wantVal) {
				t.Errorf("parseKV() = (%q, %q), want (%q, %q)", key, val, tt.wantKey, tt.wantVal)
			}
		})
	}
}

func TestParseDependency(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantVer  string
	}{
		{"base-plugin@1.0.0", "base-plugin", "1.0.0"},
		{"base-plugin", "base-plugin", ""},
		{"  spaced  @  2.0.0  ", "spaced", "2.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dep := parseDependency(tt.input)
			if dep.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", dep.Name, tt.wantName)
			}
			if dep.Version != tt.wantVer {
				t.Errorf("Version = %q, want %q", dep.Version, tt.wantVer)
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}