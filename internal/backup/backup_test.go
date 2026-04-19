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
