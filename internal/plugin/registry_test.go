package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name    string
		manifest *Manifest
		wantErr bool
	}{
		{
			name: "valid manifest",
			manifest: &Manifest{
				Name:    "my-plugin",
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "skill",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			manifest: &Manifest{
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "skill",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			manifest: &Manifest{
				Name:  "my-plugin",
				Entry: "main.go",
				Type:  "skill",
			},
			wantErr: true,
		},
		{
			name: "missing entry",
			manifest: &Manifest{
				Name:    "my-plugin",
				Version: "1.0.0",
				Type:    "skill",
			},
			wantErr: true,
		},
		{
			name: "missing type",
			manifest: &Manifest{
				Name:    "my-plugin",
				Version: "1.0.0",
				Entry:   "main.go",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			manifest: &Manifest{
				Name:    "my-plugin",
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid name with uppercase",
			manifest: &Manifest{
				Name:    "MyPlugin",
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "skill",
			},
			wantErr: true,
		},
		{
			name: "invalid name with spaces",
			manifest: &Manifest{
				Name:    "my plugin",
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "skill",
			},
			wantErr: true,
		},
		{
			name: "valid name with hyphens",
			manifest: &Manifest{
				Name:    "my-cool-plugin",
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "tool",
			},
			wantErr: false,
		},
		{
			name: "invalid version format",
			manifest: &Manifest{
				Name:    "my-plugin",
				Version: "1.0",
				Entry:   "main.go",
				Type:    "skill",
			},
			wantErr: true,
		},
		{
			name: "provider type",
			manifest: &Manifest{
				Name:    "my-provider",
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "provider",
			},
			wantErr: false,
		},
		{
			name: "hook type",
			manifest: &Manifest{
				Name:    "my-hook",
				Version: "1.0.0",
				Entry:   "main.go",
				Type:    "hook",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManifestFullName(t *testing.T) {
	m := &Manifest{Name: "my-plugin", Version: "1.2.3"}
	if got := m.FullName(); got != "my-plugin@1.2.3" {
		t.Errorf("FullName() = %v, want my-plugin@1.2.3", got)
	}
}

func TestManifestHasPermission(t *testing.T) {
	m := &Manifest{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Entry:       "main.go",
		Type:        "skill",
		Permissions: []Permission{PermFileSystem, PermNetwork},
	}

	if !m.HasPermission(PermFileSystem) {
		t.Error("HasPermission(FileSystem) = false, want true")
	}
	if m.HasPermission(PermAdmin) {
		t.Error("HasPermission(Admin) = true, want false")
	}
}

func TestManifestHasDependency(t *testing.T) {
	m := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
		Dependencies: []Dependency{
			{Name: "base-plugin", Version: "1.0.0"},
		},
	}

	if !m.HasDependency("base-plugin") {
		t.Error("HasDependency(base-plugin) = false, want true")
	}
	if m.HasDependency("nonexistent") {
		t.Error("HasDependency(nonexistent) = true, want false")
	}
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"0.1.0", true},
		{"10.20.30", true},
		{"1.0", false},
		{"1", false},
		{"", false},
		{"v1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := isValidVersion(tt.version); got != tt.want {
				t.Errorf("isValidVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestRegistryRegister(t *testing.T) {
	reg := NewRegistry("")

	m := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
	}

	if err := reg.Register(m); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if reg.Count() != 1 {
		t.Errorf("Count() = %d, want 1", reg.Count())
	}

	entry, ok := reg.Get("test-plugin")
	if !ok {
		t.Error("Get() not found")
	}
	if entry.Manifest.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", entry.Manifest.Version)
	}
	if entry.Status != StatusInstalled {
		t.Errorf("Status = %v, want installed", entry.Status)
	}
}

func TestRegistryUnregister(t *testing.T) {
	reg := NewRegistry("")

	m := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
	}

	reg.Register(m)
	if err := reg.Unregister("test-plugin"); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	if reg.Count() != 0 {
		t.Errorf("Count() = %d, want 0", reg.Count())
	}

	if err := reg.Unregister("nonexistent"); err == nil {
		t.Error("Unregister(nonexistent) should return error")
	}
}

func TestRegistryListByType(t *testing.T) {
	reg := NewRegistry("")

	reg.Register(&Manifest{Name: "p1", Version: "1.0.0", Entry: "main.go", Type: "skill"})
	reg.Register(&Manifest{Name: "p2", Version: "1.0.0", Entry: "main.go", Type: "tool"})
	reg.Register(&Manifest{Name: "p3", Version: "1.0.0", Entry: "main.go", Type: "skill"})

	skills := reg.ListByType("skill")
	if len(skills) != 2 {
		t.Errorf("ListByType(skill) = %d, want 2", len(skills))
	}

	tools := reg.ListByType("tool")
	if len(tools) != 1 {
		t.Errorf("ListByType(tool) = %d, want 1", len(tools))
	}
}

func TestRegistryUpdateStatus(t *testing.T) {
	reg := NewRegistry("")

	reg.Register(&Manifest{Name: "test-plugin", Version: "1.0.0", Entry: "main.go", Type: "skill"})

	if err := reg.UpdateStatus("test-plugin", StatusDisabled); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	entry, _ := reg.Get("test-plugin")
	if entry.Status != StatusDisabled {
		t.Errorf("Status = %v, want disabled", entry.Status)
	}

	if err := reg.UpdateStatus("nonexistent", StatusInstalled); err == nil {
		t.Error("UpdateStatus(nonexistent) should return error")
	}
}

func TestRegistrySetConfig(t *testing.T) {
	reg := NewRegistry("")

	reg.Register(&Manifest{Name: "test-plugin", Version: "1.0.0", Entry: "main.go", Type: "skill"})

	if err := reg.SetConfig("test-plugin", "key1", "value1"); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	val, ok := reg.GetConfig("test-plugin", "key1")
	if !ok || val != "value1" {
		t.Errorf("GetConfig() = %v, want value1", val)
	}

	_, ok = reg.GetConfig("test-plugin", "nonexistent")
	if ok {
		t.Error("GetConfig(nonexistent) should return false")
	}
}

func TestRegistryCheckDependencies(t *testing.T) {
	reg := NewRegistry("")

	reg.Register(&Manifest{Name: "base-plugin", Version: "1.0.0", Entry: "main.go", Type: "skill"})

	m := &Manifest{
		Name:    "dependent-plugin",
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
		Dependencies: []Dependency{
			{Name: "base-plugin"},
			{Name: "missing-plugin"},
		},
	}

	missing := reg.CheckDependencies(m)
	if len(missing) != 1 || missing[0] != "missing-plugin" {
		t.Errorf("CheckDependencies() = %v, want [missing-plugin]", missing)
	}
}

func TestRegistryLoadFromDisk(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	os.MkdirAll(pluginsDir, 0755)

	// 创建测试插件
	pluginDir := filepath.Join(pluginsDir, "test-plugin")
	os.MkdirAll(pluginDir, 0755)

	manifestContent := `name: test-plugin
version: 1.0.0
author: test-author
description: A test plugin
entry: main.go
type: skill
permissions:
  - filesystem
  - network
`
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(manifestContent), 0644)

	reg := NewRegistry(pluginsDir)
	if err := reg.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk() error = %v", err)
	}

	if reg.Count() != 1 {
		t.Errorf("Count() = %d, want 1", reg.Count())
	}

	entry, ok := reg.Get("test-plugin")
	if !ok {
		t.Error("Get(test-plugin) not found")
	}
	if entry.Manifest.Author != "test-author" {
		t.Errorf("Author = %v, want test-author", entry.Manifest.Author)
	}
	if entry.Manifest.Type != "skill" {
		t.Errorf("Type = %v, want skill", entry.Manifest.Type)
	}
}

func TestRegistryLoadFromDiskEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")

	reg := NewRegistry(pluginsDir)
	// 目录不存在时不应报错
	if err := reg.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk() on nonexistent dir error = %v", err)
	}
	if reg.Count() != 0 {
		t.Errorf("Count() = %d, want 0", reg.Count())
	}
}

func TestRegistrySetError(t *testing.T) {
	reg := NewRegistry("")

	reg.Register(&Manifest{Name: "test-plugin", Version: "1.0.0", Entry: "main.go", Type: "skill"})

	reg.SetError("test-plugin", "something went wrong")

	entry, _ := reg.Get("test-plugin")
	if entry.Status != StatusError {
		t.Errorf("Status = %v, want error", entry.Status)
	}
	if entry.Error != "something went wrong" {
		t.Errorf("Error = %v, want 'something went wrong'", entry.Error)
	}
}

func TestRegistryCountByType(t *testing.T) {
	reg := NewRegistry("")

	reg.Register(&Manifest{Name: "p1", Version: "1.0.0", Entry: "main.go", Type: "skill"})
	reg.Register(&Manifest{Name: "p2", Version: "1.0.0", Entry: "main.go", Type: "skill"})
	reg.Register(&Manifest{Name: "p3", Version: "1.0.0", Entry: "main.go", Type: "tool"})

	if got := reg.CountByType("skill"); got != 2 {
		t.Errorf("CountByType(skill) = %d, want 2", got)
	}
	if got := reg.CountByType("tool"); got != 1 {
		t.Errorf("CountByType(tool) = %d, want 1", got)
	}
	if got := reg.CountByType("provider"); got != 0 {
		t.Errorf("CountByType(provider) = %d, want 0", got)
	}
}