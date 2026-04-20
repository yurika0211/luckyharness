package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallerInstall(t *testing.T) {
	// 创建源插件目录
	srcDir := t.TempDir()
	srcPluginDir := filepath.Join(srcDir, "my-plugin")
	os.MkdirAll(srcPluginDir, 0755)

	manifestContent := `name: my-plugin
version: 1.0.0
author: test-author
description: A test plugin
entry: main.go
type: skill
`
	os.WriteFile(filepath.Join(srcPluginDir, "plugin.yaml"), []byte(manifestContent), 0644)
	os.WriteFile(filepath.Join(srcPluginDir, "main.go"), []byte("package main"), 0644)

	// 创建目标目录
	dstDir := t.TempDir()

	reg := NewRegistry(dstDir)
	inst := NewInstaller(reg, dstDir)

	result, err := inst.Install(srcPluginDir)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if result.Name != "my-plugin" {
		t.Errorf("Name = %v, want my-plugin", result.Name)
	}
	if result.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", result.Version)
	}
	if !result.Installed {
		t.Error("Installed = false, want true")
	}
	if result.Updated {
		t.Error("Updated = true, want false (first install)")
	}

	// 验证注册表
	if reg.Count() != 1 {
		t.Errorf("Count() = %d, want 1", reg.Count())
	}

	// 验证文件复制
	if _, err := os.Stat(filepath.Join(dstDir, "my-plugin", "plugin.yaml")); os.IsNotExist(err) {
		t.Error("plugin.yaml not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "my-plugin", "main.go")); os.IsNotExist(err) {
		t.Error("main.go not copied")
	}
}

func TestInstallerUninstall(t *testing.T) {
	srcDir := t.TempDir()
	srcPluginDir := filepath.Join(srcDir, "my-plugin")
	os.MkdirAll(srcPluginDir, 0755)

	manifestContent := `name: my-plugin
version: 1.0.0
author: test-author
description: A test plugin
entry: main.go
type: skill
`
	os.WriteFile(filepath.Join(srcPluginDir, "plugin.yaml"), []byte(manifestContent), 0644)

	dstDir := t.TempDir()
	reg := NewRegistry(dstDir)
	inst := NewInstaller(reg, dstDir)

	// 先安装
	inst.Install(srcPluginDir)

	// 再卸载
	if err := inst.Uninstall("my-plugin"); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	if reg.Count() != 0 {
		t.Errorf("Count() = %d, want 0", reg.Count())
	}

	// 验证目录已删除
	if _, err := os.Stat(filepath.Join(dstDir, "my-plugin")); !os.IsNotExist(err) {
		t.Error("plugin directory should be deleted")
	}
}

func TestInstallerUninstallNonexistent(t *testing.T) {
	dstDir := t.TempDir()
	reg := NewRegistry(dstDir)
	inst := NewInstaller(reg, dstDir)

	if err := inst.Uninstall("nonexistent"); err == nil {
		t.Error("Uninstall(nonexistent) should return error")
	}
}

func TestInstallerUpdate(t *testing.T) {
	srcDir := t.TempDir()

	// 创建 v1.0.0
	v1Dir := filepath.Join(srcDir, "my-plugin-v1")
	os.MkdirAll(v1Dir, 0755)
	v1Manifest := `name: my-plugin
version: 1.0.0
author: test-author
description: Version 1
entry: main.go
type: skill
`
	os.WriteFile(filepath.Join(v1Dir, "plugin.yaml"), []byte(v1Manifest), 0644)

	// 创建 v2.0.0
	v2Dir := filepath.Join(srcDir, "my-plugin-v2")
	os.MkdirAll(v2Dir, 0755)
	v2Manifest := `name: my-plugin
version: 2.0.0
author: test-author
description: Version 2
entry: main.go
type: skill
`
	os.WriteFile(filepath.Join(v2Dir, "plugin.yaml"), []byte(v2Manifest), 0644)

	dstDir := t.TempDir()
	reg := NewRegistry(dstDir)
	inst := NewInstaller(reg, dstDir)

	// 安装 v1
	result, err := inst.Install(v1Dir)
	if err != nil {
		t.Fatalf("Install v1 error = %v", err)
	}
	if result.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", result.Version)
	}

	// 更新到 v2
	result, err = inst.Update("my-plugin", v2Dir)
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}
	if result.Version != "2.0.0" {
		t.Errorf("Version = %v, want 2.0.0", result.Version)
	}
	if !result.Updated {
		t.Error("Updated = false, want true")
	}

	// 验证注册表中的版本
	entry, ok := reg.Get("my-plugin")
	if !ok {
		t.Error("Get(my-plugin) not found after update")
	}
	if entry.Manifest.Version != "2.0.0" {
		t.Errorf("Version after update = %v, want 2.0.0", entry.Manifest.Version)
	}
}

func TestInstallerInstallInvalidManifest(t *testing.T) {
	srcDir := t.TempDir()
	srcPluginDir := filepath.Join(srcDir, "bad-plugin")
	os.MkdirAll(srcPluginDir, 0755)

	// 缺少必要字段的 manifest
	badManifest := `description: Missing name and version
`
	os.WriteFile(filepath.Join(srcPluginDir, "plugin.yaml"), []byte(badManifest), 0644)

	dstDir := t.TempDir()
	reg := NewRegistry(dstDir)
	inst := NewInstaller(reg, dstDir)

	_, err := inst.Install(srcPluginDir)
	if err == nil {
		t.Error("Install() should fail for invalid manifest")
	}
}

func TestInstallerGetInstalledPath(t *testing.T) {
	srcDir := t.TempDir()
	srcPluginDir := filepath.Join(srcDir, "my-plugin")
	os.MkdirAll(srcPluginDir, 0755)

	manifestContent := `name: my-plugin
version: 1.0.0
author: test-author
description: A test plugin
entry: main.go
type: skill
`
	os.WriteFile(filepath.Join(srcPluginDir, "plugin.yaml"), []byte(manifestContent), 0644)

	dstDir := t.TempDir()
	reg := NewRegistry(dstDir)
	inst := NewInstaller(reg, dstDir)

	inst.Install(srcPluginDir)

	path, err := inst.GetInstalledPath("my-plugin")
	if err != nil {
		t.Fatalf("GetInstalledPath() error = %v", err)
	}
	if path == "" {
		t.Error("GetInstalledPath() returned empty path")
	}

	_, err = inst.GetInstalledPath("nonexistent")
	if err == nil {
		t.Error("GetInstalledPath(nonexistent) should return error")
	}
}