package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yurika0211/luckyagent/internal/provider"
)

func TestHandlerDiscoverModelsUsesCurrentProviderConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zen/v1/models" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"available-model"}]}`))
	}))
	defer server.Close()
	h := &Handler{modelDiscovery: provider.NewModelDiscovery()}
	models, source, err := h.discoverModels(context.Background(), agentConfigSnapshot{
		Provider: "openai-compatible",
		APIKey:   "test-key",
		APIBase:  server.URL + "/zen/v1/chat/completions",
	}, false)
	if err != nil {
		t.Fatalf("discoverModels: %v", err)
	}
	if source != "verified by current credentials" {
		t.Fatalf("unexpected source %q", source)
	}
	if len(models) != 1 || models[0].ID != "available-model" {
		t.Fatalf("unexpected models %#v", models)
	}
}

func TestConfiguredModelsForProviderUsesOnlyCurrentProvider(t *testing.T) {
	models := configuredModelsForProvider(agentConfigSnapshot{
		Provider: "manual-provider",
		Model:    "configured-model",
		CustomModels: []provider.ModelInfo{
			{ID: "configured-model", Provider: "manual-provider"},
			{ID: "other-model", Provider: "other-provider"},
		},
	})
	if len(models) != 1 || models[0].ID != "configured-model" {
		t.Fatalf("unexpected manual models %#v", models)
	}
}
