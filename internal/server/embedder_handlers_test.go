package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newEmbedderTestServer(t *testing.T) *Server {
	t.Helper()
	a := createTestAgent(t)
	return New(a, DefaultServerConfig())
}

func TestEmbedderList(t *testing.T) {
	s := newEmbedderTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/embedders", nil)
	w := httptest.NewRecorder()
	s.handleEmbedderList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var list []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Error("expected at least one embedder registered")
	}
}

func TestEmbedderGet(t *testing.T) {
	s := newEmbedderTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/embedders/mock-128", nil)
	w := httptest.NewRecorder()
	s.handleEmbedderGet(w, req, "mock-128")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info["id"] != "mock-128" {
		t.Errorf("id = %v, want mock-128", info["id"])
	}
}

func TestEmbedderGetNotFound(t *testing.T) {
	s := newEmbedderTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/embedders/nonexistent", nil)
	w := httptest.NewRecorder()
	s.handleEmbedderGet(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestEmbedderRegister(t *testing.T) {
	s := newEmbedderTestServer(t)

	body := `{"id":"test-mock","provider":"mock","model":"test-model","dimension":64}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embedders/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleEmbedderRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] != "test-mock" {
		t.Errorf("id = %v, want test-mock", resp["id"])
	}
}

func TestEmbedderRegisterDuplicate(t *testing.T) {
	s := newEmbedderTestServer(t)

	body := `{"id":"mock-128","provider":"mock","model":"dup","dimension":64}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embedders/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleEmbedderRegister(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestEmbedderSwitch(t *testing.T) {
	s := newEmbedderTestServer(t)

	// Register a second embedder first
	body := `{"id":"test-64","provider":"mock","model":"test","dimension":64}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embedders/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleEmbedderRegister(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: status = %d, want 201", w.Code)
	}

	// Switch to it
	switchBody := `{"id":"test-64"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/embedders/switch", strings.NewReader(switchBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.handleEmbedderSwitch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("switch: status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["active"] != "test-64" {
		t.Errorf("active = %v, want test-64", resp["active"])
	}
}

func TestEmbedderSwitchNotFound(t *testing.T) {
	s := newEmbedderTestServer(t)

	body := `{"id":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embedders/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleEmbedderSwitch(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestEmbedderTest(t *testing.T) {
	s := newEmbedderTestServer(t)

	body := `{"text":"hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embedders/mock-128/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleEmbedderTest(w, req, "mock-128")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["dimension"] == nil {
		t.Error("expected dimension in response")
	}
}

func TestEmbedderRegisterUnknownProvider(t *testing.T) {
	s := newEmbedderTestServer(t)

	body := `{"id":"bad","provider":"unknown","model":"x","dimension":64}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embedders/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleEmbedderRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestEmbedderRegisterOllama(t *testing.T) {
	s := newEmbedderTestServer(t)

	body := `{"id":"local-ollama","provider":"ollama","model":"nomic-embed-text","dimension":768}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embedders/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleEmbedderRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["name"] != "ollama" {
		t.Errorf("name = %v, want ollama", resp["name"])
	}
}
