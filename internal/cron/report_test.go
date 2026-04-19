package cron

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDailyReport(t *testing.T) {
	e := NewEngine()
	tmpDir := t.TempDir()
	rg := NewReportGenerator(tmpDir, e)

	// Add a job to report on
	e.AddJob("test-job", "Test Job", "test description", IntervalSchedule{Interval: 1 * time.Hour}, func() error { return nil })

	fn := rg.DailyReport()
	if err := fn(); err != nil {
		t.Fatalf("DailyReport failed: %v", err)
	}

	// Check file exists
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	// Check content
	content, err := os.ReadFile(filepath.Join(tmpDir, files[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(content) == 0 {
		t.Error("report is empty")
	}
}

func TestHealthCheck(t *testing.T) {
	e := NewEngine()
	tmpDir := t.TempDir()
	rg := NewReportGenerator(tmpDir, e)

	fn := rg.HealthCheck()
	if err := fn(); err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}