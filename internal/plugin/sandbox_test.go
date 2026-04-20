package plugin

import (
	"testing"
	"time"
)

func TestNewSandbox(t *testing.T) {
	limits := DefaultResourceLimits()
	sandbox := NewSandbox(limits)

	if sandbox == nil {
		t.Error("NewSandbox() returned nil")
	}
	if sandbox.limits.MaxMemoryMB != 256 {
		t.Errorf("MaxMemoryMB = %d, want 256", sandbox.limits.MaxMemoryMB)
	}
	if sandbox.limits.MaxCallsPerMin != 60 {
		t.Errorf("MaxCallsPerMin = %d, want 60", sandbox.limits.MaxCallsPerMin)
	}
}

func TestNewDefaultSandbox(t *testing.T) {
	sandbox := NewDefaultSandbox()
	if sandbox == nil {
		t.Error("NewDefaultSandbox() returned nil")
	}
}

func TestSandboxRegisterPlugin(t *testing.T) {
	sandbox := NewDefaultSandbox()

	m := &Manifest{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Entry:       "main.go",
		Type:        "skill",
		Permissions: []Permission{PermFileSystem, PermNetwork},
	}

	sandbox.RegisterPlugin("test-plugin", m)

	// 验证权限检查
	if err := sandbox.CheckPermission("test-plugin", PermFileSystem); err != nil {
		t.Errorf("CheckPermission(FileSystem) error = %v", err)
	}
	if err := sandbox.CheckPermission("test-plugin", PermNetwork); err != nil {
		t.Errorf("CheckPermission(Network) error = %v", err)
	}
	if err := sandbox.CheckPermission("test-plugin", PermAdmin); err == nil {
		t.Error("CheckPermission(Admin) should fail without admin permission")
	}
	if err := sandbox.CheckPermission("test-plugin", PermRAG); err == nil {
		t.Error("CheckPermission(RAG) should fail without RAG permission")
	}
}

func TestSandboxCheckPermissionNotRegistered(t *testing.T) {
	sandbox := NewDefaultSandbox()

	err := sandbox.CheckPermission("nonexistent", PermFileSystem)
	if err == nil {
		t.Error("CheckPermission for unregistered plugin should fail")
	}
}

func TestSandboxGrantRevokePermission(t *testing.T) {
	sandbox := NewDefaultSandbox()

	m := &Manifest{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Entry:       "main.go",
		Type:        "skill",
		Permissions: []Permission{PermFileSystem},
	}

	sandbox.RegisterPlugin("test-plugin", m)

	// 授予额外权限
	if err := sandbox.GrantPermission("test-plugin", PermNetwork); err != nil {
		t.Fatalf("GrantPermission() error = %v", err)
	}

	perms, err := sandbox.GetPermissions("test-plugin")
	if err != nil {
		t.Fatalf("GetPermissions() error = %v", err)
	}
	if len(perms) != 2 {
		t.Errorf("len(Permissions) = %d, want 2", len(perms))
	}

	// 撤销权限
	if err := sandbox.RevokePermission("test-plugin", PermNetwork); err != nil {
		t.Fatalf("RevokePermission() error = %v", err)
	}

	perms, _ = sandbox.GetPermissions("test-plugin")
	if len(perms) != 1 {
		t.Errorf("len(Permissions) after revoke = %d, want 1", len(perms))
	}

	// 重复授予不应报错
	if err := sandbox.GrantPermission("test-plugin", PermFileSystem); err != nil {
		t.Fatalf("GrantPermission() duplicate error = %v", err)
	}
}

func TestSandboxIsRestricted(t *testing.T) {
	sandbox := NewDefaultSandbox()

	m := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
	}

	sandbox.RegisterPlugin("test-plugin", m)

	if !sandbox.IsRestricted("test-plugin") {
		t.Error("IsRestricted() = false, want true (default)")
	}

	if err := sandbox.SetRestricted("test-plugin", false); err != nil {
		t.Fatalf("SetRestricted() error = %v", err)
	}

	if sandbox.IsRestricted("test-plugin") {
		t.Error("IsRestricted() = true after SetRestricted(false)")
	}
}

func TestSandboxGetSetLimits(t *testing.T) {
	sandbox := NewDefaultSandbox()

	m := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
	}

	sandbox.RegisterPlugin("test-plugin", m)

	limits, err := sandbox.GetLimits("test-plugin")
	if err != nil {
		t.Fatalf("GetLimits() error = %v", err)
	}
	if limits.MaxMemoryMB != 256 {
		t.Errorf("MaxMemoryMB = %d, want 256", limits.MaxMemoryMB)
	}

	newLimits := ResourceLimits{
		MaxMemoryMB:    512,
		MaxCPUPercent:  80,
		MaxGoroutines:  20,
		Timeout:        60 * time.Second,
		MaxOutputBytes: 2 * 1024 * 1024,
		MaxCallsPerMin: 120,
	}

	if err := sandbox.SetLimits("test-plugin", newLimits); err != nil {
		t.Fatalf("SetLimits() error = %v", err)
	}

	updatedLimits, _ := sandbox.GetLimits("test-plugin")
	if updatedLimits.MaxMemoryMB != 512 {
		t.Errorf("MaxMemoryMB after update = %d, want 512", updatedLimits.MaxMemoryMB)
	}
}

func TestSandboxUnregisterPlugin(t *testing.T) {
	sandbox := NewDefaultSandbox()

	m := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Entry:   "main.go",
		Type:    "skill",
	}

	sandbox.RegisterPlugin("test-plugin", m)
	sandbox.UnregisterPlugin("test-plugin")

	// 注销后应该找不到
	if !sandbox.IsRestricted("test-plugin") {
		t.Error("IsRestricted() for unregistered plugin should return true")
	}
}

func TestSandboxNotRegisteredErrors(t *testing.T) {
	sandbox := NewDefaultSandbox()

	if err := sandbox.SetRestricted("nonexistent", false); err == nil {
		t.Error("SetRestricted for unregistered plugin should fail")
	}

	if err := sandbox.SetLimits("nonexistent", ResourceLimits{}); err == nil {
		t.Error("SetLimits for unregistered plugin should fail")
	}

	_, err := sandbox.GetLimits("nonexistent")
	if err == nil {
		t.Error("GetLimits for unregistered plugin should fail")
	}

	_, err = sandbox.GetPermissions("nonexistent")
	if err == nil {
		t.Error("GetPermissions for unregistered plugin should fail")
	}
}

func TestFormatPermissions(t *testing.T) {
	tests := []struct {
		perms []Permission
		want  string
	}{
		{nil, "(none)"},
		{[]Permission{}, "(none)"},
		{[]Permission{PermFileSystem}, "filesystem"},
		{[]Permission{PermFileSystem, PermNetwork}, "filesystem, network"},
	}

	for _, tt := range tests {
		got := FormatPermissions(tt.perms)
		if got != tt.want {
			t.Errorf("FormatPermissions(%v) = %q, want %q", tt.perms, got, tt.want)
		}
	}
}

func TestDefaultEnforcerRateLimit(t *testing.T) {
	enforcer := &DefaultEnforcer{
		callCounts: make(map[string]*callCounter),
	}

	// 正常调用不应报错
	for i := 0; i < 5; i++ {
		if err := enforcer.CheckRateLimit("test-plugin"); err != nil {
			t.Errorf("CheckRateLimit() call %d error = %v", i, err)
		}
	}
}

func TestRemoteIndex(t *testing.T) {
	index := NewRemoteIndex("https://example.com/plugins")

	// 添加条目
	index.Add(&RemoteEntry{
		Name:        "web-search",
		Version:     "1.0.0",
		Author:      "luckyharness",
		Description: "Web search plugin",
		Type:        "tool",
		Tags:        []string{"search", "web"},
		DownloadURL: "https://example.com/plugins/web-search-1.0.0.zip",
	})

	index.Add(&RemoteEntry{
		Name:        "code-review",
		Version:     "2.1.0",
		Author:      "luckyharness",
		Description: "Code review assistant",
		Type:        "skill",
		Tags:        []string{"code", "review"},
		DownloadURL: "https://example.com/plugins/code-review-2.1.0.zip",
	})

	if index.Count() != 2 {
		t.Errorf("Count() = %d, want 2", index.Count())
	}

	// 获取
	entry, ok := index.Get("web-search")
	if !ok {
		t.Error("Get(web-search) not found")
	}
	if entry.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", entry.Version)
	}

	// 搜索
	results := index.Search("search")
	if len(results) != 1 {
		t.Errorf("Search(search) = %d results, want 1", len(results))
	}

	results = index.Search("code")
	if len(results) != 1 {
		t.Errorf("Search(code) = %d results, want 1", len(results))
	}

	// 不存在的搜索
	results = index.Search("nonexistent")
	if len(results) != 0 {
		t.Errorf("Search(nonexistent) = %d results, want 0", len(results))
	}
}