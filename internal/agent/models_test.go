package agent

import (
	"testing"

	"github.com/yurika0211/luckyagent/internal/config"
)

func TestAgentSwitchModelKindPersistsNonChatSelection(t *testing.T) {
	mgr, err := config.NewManagerWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("NewManagerWithDir: %v", err)
	}
	initial := mgr.Get()
	initial.LlmProvider.APIKey = "test-key"
	if err := mgr.Replace(initial); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	a, err := New(mgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := a.SwitchModelKind(config.ModelKindEmbedding, "embed-next", SwitchModelOptions{Persist: true, Provider: "jina"}); err != nil {
		t.Fatalf("SwitchModelKind: %v", err)
	}
	current, ok := a.CurrentModel(config.ModelKindEmbedding)
	if !ok || current.ID != "embed-next" || current.Provider != "jina" {
		t.Fatalf("unexpected embedding selection: %#v, ok=%t", current, ok)
	}
	saved := mgr.Get()
	if saved.Embedding.Model != "embed-next" {
		t.Fatalf("embedding legacy model = %q", saved.Embedding.Model)
	}
	if got := saved.Models.Active[config.ModelKindEmbedding]; got != "embed-next" {
		t.Fatalf("active embedding model = %q", got)
	}
}
