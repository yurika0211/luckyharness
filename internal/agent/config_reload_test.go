package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/yurika0211/luckyagent/internal/config"
)

func TestAgentConfigWatchUsesNewProviderModelForLaterTurns(t *testing.T) {
	seenModel := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var request struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenModel <- request.Model
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer upstream.Close()

	mgr, err := config.NewManagerWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("NewManagerWithDir: %v", err)
	}
	for key, value := range map[string]string{
		"provider": "openai",
		"api_key":  "test-key",
		"api_base": upstream.URL + "/v1",
		"model":    "old-model",
	} {
		if err := mgr.Set(key, value); err != nil {
			t.Fatalf("Set %s: %v", key, err)
		}
	}
	if err := mgr.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	a, err := New(mgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Close()
	stopWatch, err := a.StartConfigWatch(10 * time.Millisecond)
	if err != nil {
		t.Fatalf("StartConfigWatch: %v", err)
	}
	defer stopWatch()
	configJSON := fmt.Sprintf(`{"llm_provider":{"name":"openai","api_key":"test-key","base_url":%q,"model":"new-model"}}`, upstream.URL+"/v1")
	if err := os.WriteFile(mgr.ConfigFile(), []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write updated config: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if a.Config().Get().Model == "new-model" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := a.Config().Get().Model; got != "new-model" {
		t.Fatalf("expected watched configuration model, got %q", got)
	}
	if _, err := a.Chat(context.Background(), "hello"); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	select {
	case got := <-seenModel:
		if got != "new-model" {
			t.Fatalf("expected new model, got %q", got)
		}
	default:
		t.Fatal("expected a provider request")
	}
}
