package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type commandRunResult struct {
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Output  string `json:"output"`
}

func runWebCommand(t *testing.T, server *Server, body string) commandRunResult {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.handleCommands(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var result commandRunResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return result
}

func TestCommandCatalogIsServed(t *testing.T) {
	server := New(createTestAgent(t), DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/commands", nil)
	recorder := httptest.NewRecorder()
	server.handleCommands(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}

	var payload struct {
		Commands []commandSpec `json:"commands"`
		Count    int           `json:"count"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Count == 0 || len(payload.Commands) != payload.Count {
		t.Fatalf("count = %d, commands = %d", payload.Count, len(payload.Commands))
	}
	for _, spec := range payload.Commands {
		if spec.Name == "" || spec.Usage == "" || spec.Description == "" {
			t.Fatalf("incomplete spec: %+v", spec)
		}
		if !strings.HasPrefix(spec.Usage, "/"+spec.Name) {
			t.Fatalf("usage %q does not start with /%s", spec.Usage, spec.Name)
		}
	}
}

// Every advertised command must actually run; a catalog entry with no
// implementation would be a dead menu item in the UI.
func TestEveryAdvertisedCommandRuns(t *testing.T) {
	server := New(createTestAgent(t), DefaultServerConfig())

	// Commands that legitimately refuse without an argument or active session.
	needsInput := map[string]bool{
		"tool": true, "skill": true, "remember": true, "remember_long": true,
		"recall": true, "promote": true, "rename": true, "session": true,
	}

	for _, spec := range webCommandSpecs() {
		result := runWebCommand(t, server, `{"command":"`+spec.Name+`"}`)
		if result.Command != spec.Name {
			t.Fatalf("%s: echoed command = %q", spec.Name, result.Command)
		}
		if result.Output == "" {
			t.Fatalf("%s: empty output", spec.Name)
		}
		if !result.OK && !needsInput[spec.Name] {
			t.Fatalf("%s: ok = false with output %q", spec.Name, result.Output)
		}
		if strings.Contains(result.Output, "is not available in the web UI") {
			t.Fatalf("%s is advertised but has no implementation", spec.Name)
		}
	}
}

func TestUnknownCommandIsReportedNotCrashed(t *testing.T) {
	server := New(createTestAgent(t), DefaultServerConfig())

	result := runWebCommand(t, server, `{"command":"lucky"}`)
	if result.OK {
		t.Fatal("ok = true for a command the web UI does not implement")
	}
	if !strings.Contains(result.Output, "not available") {
		t.Fatalf("output = %q, want an explanation", result.Output)
	}
}

func TestCommandNameIsNormalized(t *testing.T) {
	server := New(createTestAgent(t), DefaultServerConfig())

	// Leading slash, upper case, and the dash spelling all reach the same command.
	for _, body := range []string{
		`{"command":"/MEMSTATS"}`,
		`{"command":"memstats"}`,
	} {
		result := runWebCommand(t, server, body)
		if !result.OK || result.Command != "memstats" {
			t.Fatalf("%s -> command=%q ok=%v", body, result.Command, result.OK)
		}
	}

	dashed := runWebCommand(t, server, `{"command":"remember-long","args":"a durable note"}`)
	if dashed.Command != "remember_long" {
		t.Fatalf("remember-long normalized to %q, want remember_long", dashed.Command)
	}
}

func TestConfigCommandWithholdsValues(t *testing.T) {
	server := New(createTestAgent(t), DefaultServerConfig())

	result := runWebCommand(t, server, `{"command":"config"}`)
	if !result.OK {
		t.Fatalf("ok = false: %s", result.Output)
	}

	// The output goes straight into a chat transcript, so it must name sections
	// and nothing else. Rather than grep for secret-looking words, assert that
	// no configured value appears verbatim in the rendering.
	raw, err := json.Marshal(server.agent.Config().Get())
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var tree map[string]interface{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	// A value that happens to equal a section name (e.g. a provider literally
	// named "opencli") is not a leak — the section list is what we print.
	sectionNames := make(map[string]bool, len(tree))
	for key := range tree {
		sectionNames[key] = true
	}
	for _, value := range collectStringValues(tree) {
		if len(value) < 6 || sectionNames[value] {
			continue
		}
		if strings.Contains(result.Output, value) {
			t.Fatalf("config output leaks the value %q:\n%s", value, result.Output)
		}
	}
	if !strings.Contains(result.Output, "`agent`") {
		t.Fatalf("config output does not list section names:\n%s", result.Output)
	}
}

// collectStringValues walks a decoded JSON tree and returns every string leaf,
// which is where a credential would live.
func collectStringValues(node interface{}) []string {
	switch typed := node.(type) {
	case string:
		return []string{typed}
	case []interface{}:
		var out []string
		for _, item := range typed {
			out = append(out, collectStringValues(item)...)
		}
		return out
	case map[string]interface{}:
		var out []string
		for _, item := range typed {
			out = append(out, collectStringValues(item)...)
		}
		return out
	default:
		return nil
	}
}

func TestCommandRequiresAName(t *testing.T) {
	server := New(createTestAgent(t), DefaultServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands", bytes.NewReader([]byte(`{"command":"  "}`)))
	recorder := httptest.NewRecorder()
	server.handleCommands(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}
