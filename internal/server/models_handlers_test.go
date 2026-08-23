package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleConfigRedactsSecrets(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	response := httptest.NewRecorder()
	s.handleConfig(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sk-test") {
		t.Fatalf("config endpoint exposed API key: %s", response.Body.String())
	}
}

func TestHandleModelSwitchPersistsTypedSelection(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/models/switch", bytes.NewBufferString(`{"kind":"embedding","model":"embed-test","provider":"jina"}`))
	response := httptest.NewRecorder()
	s.handleModelSwitch(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if got := a.Config().Get().Models.Active["embedding"]; got != "embed-test" {
		t.Fatalf("active embedding model = %q", got)
	}
}
