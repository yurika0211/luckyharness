package server

import (
	"net/http"
	"strings"

	"github.com/yurika0211/luckyagent/internal/memory"
)

const (
	defaultGraphNodeLimit = 300
	maxGraphNodeLimit     = 1500
)

// handleMemoryGraph returns the wikilink topology of the memory vault so a UI
// can draw it. Isolated notes are excluded unless asked for: they are the
// majority of a typical vault and carry no topology.
func (s *Server) handleMemoryGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	store := s.agent.Memory()
	if store == nil {
		s.sendError(w, "memory store not initialized", http.StatusServiceUnavailable, "")
		return
	}

	topology := store.GraphTopology(memory.GraphTopologyOptions{
		Limit:           boundedQueryInt(r, "limit", defaultGraphNodeLimit, 1, maxGraphNodeLimit),
		IncludeIsolated: isTruthyQuery(r.URL.Query().Get("isolated")),
	})

	s.sendJSON(w, http.StatusOK, topology)
}

func isTruthyQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
