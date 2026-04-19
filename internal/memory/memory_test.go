package memory

import (
	"testing"
)

func TestSaveAndSearch(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := s.Save("user prefers Chinese", "preference"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save("project uses Go", "context"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	results := s.Search("Chinese")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	results = s.Search("Go")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRecent(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	for i := 0; i < 10; i++ {
		s.Save("memory item", "test")
	}

	recent := s.Recent(3)
	if len(recent) != 3 {
		t.Errorf("expected 3, got %d", len(recent))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	s1, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore1: %v", err)
	}
	s1.Save("persistent memory", "test")

	// Reload
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore2: %v", err)
	}
	results := s2.Search("persistent")
	if len(results) != 1 {
		t.Errorf("expected 1 persistent result, got %d", len(results))
	}
}
