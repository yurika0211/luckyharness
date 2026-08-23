package lhcmd

import (
	"testing"

	"github.com/yurika0211/luckyagent/internal/config"
	"github.com/yurika0211/luckyagent/internal/gateway"
)

func TestBuildTimeoutRecommendationsIncludesProfilesAndCurrentValues(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MsgGateway.Telegram.ChatTimeoutSeconds = 600
	cfg.Agent.TimeoutSeconds = 60
	cfg.OpenCLI.TimeoutSeconds = 20

	recommendations := buildTimeoutRecommendations(cfg, nil)
	if len(recommendations) != 2 {
		t.Fatalf("recommendations = %d, want 2", len(recommendations))
	}
	quick, ok := recommendationByID(recommendations, "quick_response")
	if !ok {
		t.Fatal("quick_response recommendation missing")
	}
	if quick["priority"] != "info" || quick["evidence_events"] != 0 {
		t.Fatalf("unexpected no-evidence metadata: %#v", quick)
	}
	current, ok := quick["current"].(map[string]int)
	if !ok || current["agent.timeout_seconds"] != 60 {
		t.Fatalf("current timeout values missing or incorrect: %#v", quick["current"])
	}
	suggested, ok := quick["suggested"].(map[string]int)
	if !ok || suggested["msg_gateway.telegram.chat_timeout_seconds"] != 300 || suggested["agent.timeout_seconds"] != 30 {
		t.Fatalf("quick response values incorrect: %#v", quick["suggested"])
	}
}

func TestBuildTimeoutRecommendationsPrioritizesComplexProfileAfterTelegramTimeout(t *testing.T) {
	cfg := config.DefaultConfig()
	events := []gateway.TimeoutEvent{{Layer: "Telegram Gateway Chat"}}
	recommendations := buildTimeoutRecommendations(cfg, events)

	quick, _ := recommendationByID(recommendations, "quick_response")
	complex, _ := recommendationByID(recommendations, "complex_tasks")
	if quick["priority"] != "alternative" {
		t.Fatalf("quick response priority = %v, want alternative", quick["priority"])
	}
	if complex["priority"] != "recommended" || complex["evidence_events"] != 1 {
		t.Fatalf("complex recommendation metadata = %#v", complex)
	}
}

func recommendationByID(items []map[string]interface{}, id string) (map[string]interface{}, bool) {
	for _, item := range items {
		if item["id"] == id {
			return item, true
		}
	}
	return nil, false
}
