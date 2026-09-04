package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yurika0211/luckyagent/internal/provider"
)

type historyPage struct {
	MessageCount int                `json:"message_count"`
	Messages     []provider.Message `json:"messages"`
	Limit        int                `json:"limit"`
	Offset       int                `json:"offset"`
	Returned     int                `json:"returned"`
	HasMore      bool               `json:"has_more"`
}

func seedHistorySession(t *testing.T, id string, count int) *Server {
	t.Helper()
	agent := createTestAgent(t)
	server := New(agent, DefaultServerConfig())
	sess := agent.Sessions().Ensure(id)
	for i := 0; i < count; i++ {
		sess.AddProviderMessage(provider.Message{Role: "user", Content: fmt.Sprintf("message-%d", i)})
	}
	return server
}

func fetchHistory(t *testing.T, server *Server, id, query string) historyPage {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id+query, nil)
	writer := httptest.NewRecorder()
	server.handleSessionByID(writer, req)
	if writer.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", writer.Code, writer.Body.String())
	}
	var page historyPage
	if err := json.Unmarshal(writer.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return page
}

func TestSessionHistoryPagesFromTheNewestMessage(t *testing.T) {
	const id = "history-paging"
	server := seedHistorySession(t, id, 150)

	first := fetchHistory(t, server, id, "?limit=60")
	if first.Returned != 60 || len(first.Messages) != 60 {
		t.Fatalf("returned = %d, messages = %d, want 60", first.Returned, len(first.Messages))
	}
	if first.MessageCount != 150 {
		t.Fatalf("message_count = %d, want the session total 150", first.MessageCount)
	}
	if !first.HasMore {
		t.Fatal("has_more = false, want true while older messages remain")
	}
	if got := first.Messages[59].Content; got != "message-149" {
		t.Fatalf("last message of page 0 = %q, want the newest message-149", got)
	}
	if got := first.Messages[0].Content; got != "message-90" {
		t.Fatalf("first message of page 0 = %q, want message-90", got)
	}

	second := fetchHistory(t, server, id, "?limit=60&offset=60")
	if got := second.Messages[59].Content; got != "message-89" {
		t.Fatalf("page 1 ends at %q, want message-89 — pages must not overlap or skip", got)
	}
	if !second.HasMore {
		t.Fatal("has_more = false on page 1, want true")
	}

	last := fetchHistory(t, server, id, "?limit=60&offset=120")
	if last.Returned != 30 {
		t.Fatalf("final page returned = %d, want the remaining 30", last.Returned)
	}
	if got := last.Messages[0].Content; got != "message-0" {
		t.Fatalf("final page starts at %q, want the oldest message-0", got)
	}
	if last.HasMore {
		t.Fatal("has_more = true on the final page, want false")
	}
}

func TestSessionHistoryOffsetBeyondEndIsEmpty(t *testing.T) {
	const id = "history-overshoot"
	server := seedHistorySession(t, id, 10)

	page := fetchHistory(t, server, id, "?limit=60&offset=500")
	if page.Returned != 0 || len(page.Messages) != 0 {
		t.Fatalf("returned = %d, messages = %d, want an empty page", page.Returned, len(page.Messages))
	}
	if page.HasMore {
		t.Fatal("has_more = true past the end, want false")
	}
}

func TestSessionHistoryWithoutLimitStaysUnpaged(t *testing.T) {
	const id = "history-unpaged"
	server := seedHistorySession(t, id, 120)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id, nil)
	writer := httptest.NewRecorder()
	server.handleSessionByID(writer, req)
	if writer.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", writer.Code, writer.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(writer.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := raw["has_more"]; ok {
		t.Fatal("has_more present without a limit; existing clients must see the unpaged shape")
	}

	var page historyPage
	if err := json.Unmarshal(writer.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(page.Messages) != 120 {
		t.Fatalf("messages = %d, want the full 120 when no limit is given", len(page.Messages))
	}
}

func TestSessionHistoryLimitIsClamped(t *testing.T) {
	const id = "history-clamp"
	server := seedHistorySession(t, id, 600)

	page := fetchHistory(t, server, id, "?limit=99999")
	if page.Returned != maxHistoryLimit {
		t.Fatalf("returned = %d, want the clamp at %d", page.Returned, maxHistoryLimit)
	}
}
