package cron

import (
	"os"
	"testing"
	"time"
)

func TestWatcherAddPattern(t *testing.T) {
	e := NewEngine()
	w := NewWatcher(e)

	err := w.AddPattern("p1", "Test Pattern", "desc", "/tmp/*.log", 1*time.Minute, nil)
	if err != nil {
		t.Fatalf("AddPattern failed: %v", err)
	}

	patterns := w.ListPatterns()
	if len(patterns) != 1 {
		t.Errorf("ListPatterns count = %d, want 1", len(patterns))
	}

	// Duplicate
	err = w.AddPattern("p1", "Test Pattern", "desc", "/tmp/*.log", 1*time.Minute, nil)
	if err == nil {
		t.Error("AddPattern duplicate should fail")
	}
}

func TestWatcherRemovePattern(t *testing.T) {
	e := NewEngine()
	w := NewWatcher(e)

	w.AddPattern("p1", "Test Pattern", "desc", "/tmp/*.log", 1*time.Minute, nil)

	err := w.RemovePattern("p1")
	if err != nil {
		t.Fatalf("RemovePattern failed: %v", err)
	}

	if len(w.ListPatterns()) != 0 {
		t.Error("pattern should be removed")
	}

	// Not found
	err = w.RemovePattern("nonexistent")
	if err == nil {
		t.Error("RemovePattern nonexistent should fail")
	}
}

func TestWatcherAlertHandler(t *testing.T) {
	e := NewEngine()
	w := NewWatcher(e)

	var alerts []WatchAlert
	w.SetAlertHandler(func(a WatchAlert) {
		alerts = append(alerts, a)
	})

	// Create a temp file to match
	tmpFile, err := os.CreateTemp("", "watcher_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	pattern := tmpFile.Name()
	w.AddPattern("p1", "Test Pattern", "desc", pattern, 1*time.Second, nil)

	// Manually trigger check
	w.checkPattern(w.patterns["p1"], time.Now())

	if len(alerts) == 0 {
		t.Error("expected alert to be triggered")
	}
}

func TestWatcherStartStop(t *testing.T) {
	e := NewEngine()
	w := NewWatcher(e)

	w.Start()
	if !w.running {
		t.Error("watcher should be running")
	}

	w.Stop()
	if w.running {
		t.Error("watcher should not be running")
	}

	// Double stop should be safe
	w.Stop()
}