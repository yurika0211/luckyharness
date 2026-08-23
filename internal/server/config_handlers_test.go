package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandleConfigReload(t *testing.T) {
	a := createTestAgent(t)
	if err := a.Config().Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	s := New(a, DefaultServerConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reload", nil)
	w := httptest.NewRecorder()
	s.handleConfigReload(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"reloaded"`) {
		t.Fatalf("unexpected response %s", w.Body.String())
	}
}

func TestHandleConfigReloadDoesNotExposeCredential(t *testing.T) {
	a := createTestAgent(t)
	if err := a.Config().Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := os.WriteFile(a.Config().ConfigFile(), []byte(`{"llm_provider":`), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	s := New(a, DefaultServerConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reload", nil)
	w := httptest.NewRecorder()
	s.handleConfigReload(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "sk-test") {
		t.Fatalf("credential leaked in response %s", w.Body.String())
	}
}
