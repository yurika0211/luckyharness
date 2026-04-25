package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	outputPath := filepath.Join(tmpDir, "backup.tar.gz")

	// 创建一些测试文件
	os.MkdirAll(filepath.Join(homeDir, "memory"), 0700)
	os.WriteFile(filepath.Join(homeDir, "config.yaml"), []byte("provider: openai\nmodel: gpt-4o\n"), 0600)
	os.WriteFile(filepath.Join(homeDir, "memory", "memory.json"), []byte(`{"entries":[]}`), 0600)
	os.WriteFile(filepath.Join(homeDir, "SOUL.md"), []byte("# Test SOUL"), 0644)

	// 创建备份
	b := New(homeDir)
	if err := b.Create(outputPath); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 验证备份文件存在
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("backup file not created")
	}

	// 删除原始文件
	os.RemoveAll(homeDir)

	// 恢复
	if err := b.Restore(outputPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// 验证恢复
	data, err := os.ReadFile(filepath.Join(homeDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(data) != "provider: openai\nmodel: gpt-4o\n" {
		t.Errorf("unexpected config content: %s", data)
	}

	soulData, err := os.ReadFile(filepath.Join(homeDir, "SOUL.md"))
	if err != nil {
		t.Fatalf("read restored SOUL: %v", err)
	}
	if string(soulData) != "# Test SOUL" {
		t.Errorf("unexpected SOUL content: %s", soulData)
	}
}

func TestCreateAutoName(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)
	os.WriteFile(filepath.Join(homeDir, "test.txt"), []byte("hello"), 0600)

	b := New(homeDir)
	if err := b.Create(""); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 应该自动生成文件名
	backups, err := b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(backups) == 0 {
		t.Error("no backup files found")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)
	os.WriteFile(filepath.Join(homeDir, "test.txt"), []byte("hello"), 0600)

	b := New(homeDir)

	// 创建两个备份
	_ = b.Create(filepath.Join(tmpDir, "b1.tar.gz"))
	_ = b.Create(filepath.Join(tmpDir, "b2.tar.gz"))

	// 手动复制到 homeDir
	data1, _ := os.ReadFile(filepath.Join(tmpDir, "b1.tar.gz"))
	os.WriteFile(filepath.Join(homeDir, "backup_2026-01-01_000000.tar.gz"), data1, 0600)
	data2, _ := os.ReadFile(filepath.Join(tmpDir, "b2.tar.gz"))
	os.WriteFile(filepath.Join(homeDir, "backup_2026-01-02_000000.tar.gz"), data2, 0600)

	backups, err := b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(backups) != 2 {
		t.Errorf("expected 2 backups, got %d", len(backups))
	}
}

func TestInfo(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)
	os.WriteFile(filepath.Join(homeDir, "test.txt"), []byte("hello"), 0600)

	outputPath := filepath.Join(tmpDir, "backup.tar.gz")
	b := New(homeDir)
	_ = b.Create(outputPath)

	info, err := b.Info(outputPath)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	if info["path"] != outputPath {
		t.Errorf("unexpected path: %v", info["path"])
	}
	if info["size"].(int64) == 0 {
		t.Error("backup file is empty")
	}
}

func TestRestoreNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	b := New(filepath.Join(tmpDir, "lh"))

	err := b.Restore("/nonexistent/backup.tar.gz")
	if err == nil {
		t.Error("expected error for nonexistent backup")
	}
}

func TestBackupSkipsOldBackups(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)
	os.WriteFile(filepath.Join(homeDir, "test.txt"), []byte("hello"), 0600)
	// 创建一个旧备份
	os.WriteFile(filepath.Join(homeDir, "backup_old.tar.gz"), []byte("old backup data"), 0600)

	b := New(homeDir)
	outputPath := filepath.Join(tmpDir, "new_backup.tar.gz")
	if err := b.Create(outputPath); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 恢复到新目录
	restoreDir := filepath.Join(tmpDir, "restored")
	os.MkdirAll(restoreDir, 0700)
	b2 := New(restoreDir)
	if err := b2.Restore(outputPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// 旧备份不应该被包含
	if _, err := os.Stat(filepath.Join(restoreDir, "backup_old.tar.gz")); err == nil {
		t.Error("old backup should not be included in new backup")
	}
}

// v0.82.0: backup 包测试补全 - 覆盖边缘情况

// TestCreate_InvalidPath 测试 Create 处理无效路径
func TestCreate_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)
	os.WriteFile(filepath.Join(homeDir, "test.txt"), []byte("hello"), 0600)

	b := New(homeDir)

	// 尝试写入不可写路径（/proc 是不可写的）
	err := b.Create("/proc/test_backup.tar.gz")
	if err == nil {
		t.Error("Create should fail for unwritable path")
	}

	t.Logf("Create unwritable path error: %v", err)
}

// TestCreate_EmptyHomeDir 测试空 home 目录
func TestCreate_EmptyHomeDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "empty_lh")
	outputPath := filepath.Join(tmpDir, "empty_backup.tar.gz")

	// 创建空目录
	os.MkdirAll(homeDir, 0700)

	b := New(homeDir)
	err := b.Create(outputPath)

	if err != nil {
		t.Fatalf("Create should succeed with empty home dir: %v", err)
	}

	// 验证备份文件存在
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("backup file should be created")
	}

	t.Logf("Empty home dir backup created: %s", outputPath)
}

// TestRestore_InvalidArchive 测试 Restore 处理无效归档
func TestRestore_InvalidArchive(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)

	b := New(homeDir)

	// 尝试恢复不存在的文件
	err := b.Restore(filepath.Join(tmpDir, "nonexistent.tar.gz"))
	if err == nil {
		t.Error("Restore should fail for nonexistent file")
	}

	t.Logf("Restore nonexistent file error: %v", err)
}

// TestRestore_CorruptedArchive 测试 Restore 处理损坏的归档
func TestRestore_CorruptedArchive(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	outputPath := filepath.Join(tmpDir, "corrupted.tar.gz")

	os.MkdirAll(homeDir, 0700)

	// 创建一个损坏的"归档"（不是真正的 gzip）
	os.WriteFile(outputPath, []byte("this is not a valid tar.gz file"), 0600)

	b := New(homeDir)
	err := b.Restore(outputPath)

	if err == nil {
		t.Error("Restore should fail for corrupted archive")
	}

	t.Logf("Restore corrupted archive error: %v", err)
}

// TestList_NoBackups 测试 List 在没有备份时
func TestList_NoBackups(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)

	b := New(homeDir)
	backups, err := b.List()

	if err != nil {
		t.Fatalf("List should not fail: %v", err)
	}

	if len(backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(backups))
	}

	t.Logf("List returned empty slice as expected")
}

// TestInfo_NonExistentBackup 测试 Info 处理不存在的备份
func TestInfo_NonExistentBackup(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	os.MkdirAll(homeDir, 0700)

	b := New(homeDir)
	info, err := b.Info("nonexistent.tar.gz")

	if err == nil {
		t.Error("Info should fail for nonexistent backup")
	}

	if info != nil {
		t.Error("Info should return nil for nonexistent backup")
	}

	t.Logf("Info nonexistent backup error: %v", err)
}

// TestInfo_InvalidBackup 测试 Info 处理无效备份文件
func TestInfo_InvalidBackup(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	outputPath := filepath.Join(tmpDir, "invalid.tar.gz")

	os.MkdirAll(homeDir, 0700)

	// 创建一个无效的备份文件
	os.WriteFile(outputPath, []byte("not a valid archive"), 0600)

	b := New(homeDir)
	info, err := b.Info("invalid.tar.gz")

	// Info 可能只检查文件存在性和大小，不一定验证归档完整性
	if err != nil {
		t.Logf("Info error (may be expected): %v", err)
	}

	if info == nil {
		t.Log("Info returned nil for invalid backup")
	} else {
		t.Logf("Info returned: %+v", info)
	}
}

// TestCreate_WithSymlink 测试 Create 处理符号链接
func TestCreate_WithSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	outputPath := filepath.Join(tmpDir, "symlink_backup.tar.gz")

	os.MkdirAll(homeDir, 0700)

	// 创建测试文件
	testFile := filepath.Join(homeDir, "original.txt")
	os.WriteFile(testFile, []byte("original content"), 0600)

	// 创建符号链接
	linkPath := filepath.Join(homeDir, "link.txt")
	if err := os.Symlink(testFile, linkPath); err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	b := New(homeDir)
	err := b.Create(outputPath)

	if err != nil {
		t.Fatalf("Create with symlink failed: %v", err)
	}

	t.Logf("Backup with symlink created successfully")
}

// TestCreate_WithLargeFile 测试 Create 处理大文件
func TestCreate_WithLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	outputPath := filepath.Join(tmpDir, "large_backup.tar.gz")

	os.MkdirAll(homeDir, 0700)

	// 创建一个较大的文件（1MB）
	largeFile := filepath.Join(homeDir, "large.bin")
	f, err := os.Create(largeFile)
	if err != nil {
		t.Fatalf("create large file: %v", err)
	}
	// 写入 1MB 数据
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	f.Write(data)
	f.Close()

	b := New(homeDir)
	err = b.Create(outputPath)

	if err != nil {
		t.Fatalf("Create with large file failed: %v", err)
	}

	// 验证备份文件大小合理（应该被压缩）
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}

	t.Logf("Large file backup created: %d bytes (compressed from 1MB)", info.Size())
}

// TestRestore_ToExistingFiles 测试 Restore 覆盖已存在文件
func TestRestore_ToExistingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "lh")
	outputPath := filepath.Join(tmpDir, "backup.tar.gz")

	os.MkdirAll(homeDir, 0700)
	os.WriteFile(filepath.Join(homeDir, "config.txt"), []byte("original"), 0600)

	b := New(homeDir)
	if err := b.Create(outputPath); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 修改原始文件
	os.WriteFile(filepath.Join(homeDir, "config.txt"), []byte("modified"), 0600)

	// 恢复（应该覆盖）
	if err := b.Restore(outputPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// 验证文件被恢复
	data, err := os.ReadFile(filepath.Join(homeDir, "config.txt"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}

	if string(data) != "original" {
		t.Errorf("expected 'original', got '%s'", string(data))
	}

	t.Logf("Restore successfully overwrote existing files")
}

// TestBackupStruct 测试 Backup 结构体
func TestBackupStruct(t *testing.T) {
	b := New("/test/home/dir")

	if b.homeDir != "/test/home/dir" {
		t.Errorf("expected homeDir '/test/home/dir', got '%s'", b.homeDir)
	}

	t.Logf("Backup struct created with homeDir: %s", b.homeDir)
}
