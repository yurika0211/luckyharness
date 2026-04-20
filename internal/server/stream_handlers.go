package server

import (
	"encoding/json"
	"net/http"
)

// handleRAGStreamWatch handles watch directory operations
// POST: add watch directory
// DELETE: remove watch directory
func (s *Server) handleRAGStreamWatch(w http.ResponseWriter, r *http.Request) {
	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendError(w, "stream indexer not initialized", http.StatusServiceUnavailable, "")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req struct {
			Dir string `json:"dir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendError(w, "invalid request body", http.StatusBadRequest, err.Error())
			return
		}
		if req.Dir == "" {
			s.sendError(w, "dir is required", http.StatusBadRequest, "")
			return
		}

		streamIndexer.AddWatchDir(req.Dir)
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"action": "add_watch",
			"dir":    req.Dir,
		})

	case http.MethodDelete:
		var req struct {
			Dir string `json:"dir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendError(w, "invalid request body", http.StatusBadRequest, err.Error())
			return
		}
		if req.Dir == "" {
			s.sendError(w, "dir is required", http.StatusBadRequest, "")
			return
		}

		streamIndexer.RemoveWatchDir(req.Dir)
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"action": "remove_watch",
			"dir":    req.Dir,
		})

	default:
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
	}
}

// handleRAGStreamScan triggers a change scan
func (s *Server) handleRAGStreamScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendError(w, "stream indexer not initialized", http.StatusServiceUnavailable, "")
		return
	}

	changes := streamIndexer.Scan()

	// Convert changes to response format
	changeResults := make([]map[string]interface{}, len(changes))
	for i, c := range changes {
		changeResults[i] = map[string]interface{}{
			"path":      c.Path,
			"type":      c.Type.String(),
			"timestamp": c.Timestamp,
		}
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"changes":     len(changes),
		"queue_len":   streamIndexer.Queue().Len(),
		"details":     changeResults,
	})
}

// handleRAGStreamStart starts background workers
func (s *Server) handleRAGStreamStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendError(w, "stream indexer not initialized", http.StatusServiceUnavailable, "")
		return
	}

	if streamIndexer.IsRunning() {
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "already_running",
			"message": "stream indexer is already running",
		})
		return
	}

	streamIndexer.Start()
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "started",
		"message": "stream indexer started",
	})
}

// handleRAGStreamStop stops background workers
func (s *Server) handleRAGStreamStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendError(w, "stream indexer not initialized", http.StatusServiceUnavailable, "")
		return
	}

	if !streamIndexer.IsRunning() {
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "already_stopped",
			"message": "stream indexer is not running",
		})
		return
	}

	streamIndexer.Stop()
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "stopped",
		"message": "stream indexer stopped",
	})
}

// handleRAGStreamStatus returns stream indexer status
func (s *Server) handleRAGStreamStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"initialized": false,
			"message":     "stream indexer not configured",
		})
		return
	}

	stats := streamIndexer.Stats()
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"initialized":   true,
		"running":       stats.Running,
		"queue_len":     stats.QueueLen,
		"watch_dirs":    stats.WatchDirs,
		"tracked_files": stats.TrackedFiles,
	})
}

// handleRAGStreamIndex handles immediate path indexing
// POST: index a path immediately
// DELETE: remove a path from index
func (s *Server) handleRAGStreamIndex(w http.ResponseWriter, r *http.Request) {
	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendError(w, "stream indexer not initialized", http.StatusServiceUnavailable, "")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendError(w, "invalid request body", http.StatusBadRequest, err.Error())
			return
		}
		if req.Path == "" {
			s.sendError(w, "path is required", http.StatusBadRequest, "")
			return
		}

		doc, err := streamIndexer.IndexPath(req.Path)
		if err != nil {
			s.sendError(w, "index path failed", http.StatusInternalServerError, err.Error())
			return
		}

		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"action":    "index_path",
			"doc_id":    doc.ID,
			"title":     doc.Title,
			"chunks":    len(doc.Chunks),
			"indexed_at": doc.IndexedAt,
		})

	case http.MethodDelete:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendError(w, "invalid request body", http.StatusBadRequest, err.Error())
			return
		}
		if req.Path == "" {
			s.sendError(w, "path is required", http.StatusBadRequest, "")
			return
		}

		removed := streamIndexer.RemovePath(req.Path)
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"action":  "remove_path",
			"path":    req.Path,
			"removed": removed,
		})

	default:
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
	}
}

// handleRAGStreamQueue returns pending jobs in the queue
func (s *Server) handleRAGStreamQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendError(w, "stream indexer not initialized", http.StatusServiceUnavailable, "")
		return
	}

	jobs := streamIndexer.Queue().List()
	jobResults := make([]map[string]interface{}, len(jobs))
	for i, job := range jobs {
		jobResults[i] = map[string]interface{}{
			"id":        job.ID,
			"path":      job.Path,
			"type":      job.JobType.String(),
			"priority":  job.Priority,
			"created_at": job.CreatedAt,
		}
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"total": len(jobs),
		"jobs":  jobResults,
	})
}

// handleRAGStreamProcess processes pending jobs
func (s *Server) handleRAGStreamProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	var req struct {
		Batch int `json:"batch,omitempty"` // number of jobs to process, default 1
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Batch <= 0 {
		req.Batch = 1
	}

	streamIndexer := s.agent.StreamIndexer()
	if streamIndexer == nil {
		s.sendError(w, "stream indexer not initialized", http.StatusServiceUnavailable, "")
		return
	}

	jobs, docs, errs := streamIndexer.ProcessBatch(r.Context(), req.Batch)

	results := make([]map[string]interface{}, len(jobs))
	for i := range jobs {
		result := map[string]interface{}{
			"job_id":   jobs[i].ID,
			"path":     jobs[i].Path,
			"type":     jobs[i].JobType.String(),
			"error":    nil,
		}
		if errs[i] != nil {
			result["error"] = errs[i].Error()
		}
		if docs[i] != nil {
			result["doc_id"] = docs[i].ID
			result["title"] = docs[i].Title
			result["chunks"] = len(docs[i].Chunks)
		}
		results[i] = result
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"processed": len(jobs),
		"results":   results,
	})
}