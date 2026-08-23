package server

import (
	"net/http"
)

type configReloadResponse struct {
	Reloaded        bool     `json:"reloaded"`
	HotReloaded     []string `json:"hot_reloaded,omitempty"`
	RestartRequired []string `json:"restart_required,omitempty"`
}

func (s *Server) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}
	if s.agent == nil {
		s.sendError(w, "agent runtime unavailable", http.StatusServiceUnavailable, "")
		return
	}
	result, err := s.agent.ReloadConfig()
	if err != nil {
		s.sendError(w, "configuration reload rejected; previous configuration remains active", http.StatusBadRequest, "configuration validation or parsing failed")
		return
	}
	s.sendJSON(w, http.StatusOK, configReloadResponse{
		Reloaded:        result.Changed,
		HotReloaded:     result.HotReloaded,
		RestartRequired: result.RestartRequired,
	})
}
