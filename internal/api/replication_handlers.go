package api

import (
	"net/http"

	"github.com/teamvault/teamvault/internal/replication"
)

// handleReplicationPush receives replication entries from the leader.
// POST /api/v1/replication/push
func (s *Server) handleReplicationPush(w http.ResponseWriter, r *http.Request) {
	if s.replicationManager == nil {
		writeError(w, http.StatusServiceUnavailable, "replication not available")
		return
	}

	ctx := r.Context()

	var req replication.PushRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Entries) == 0 {
		writeJSON(w, http.StatusOK, replication.PushResponse{
			Applied: 0,
			LastID:  "",
		})
		return
	}

	applied, err := s.replicationManager.ApplyEntries(ctx, req.Entries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply entries")
		return
	}

	lastID := ""
	if len(req.Entries) > 0 {
		lastID = req.Entries[len(req.Entries)-1].ID
	}

	writeJSON(w, http.StatusOK, replication.PushResponse{
		Applied: applied,
		LastID:  lastID,
	})
}

// handleReplicationPull returns replication entries for a follower.
// POST /api/v1/replication/pull
func (s *Server) handleReplicationPull(w http.ResponseWriter, r *http.Request) {
	if s.replicationManager == nil {
		writeError(w, http.StatusServiceUnavailable, "replication not available")
		return
	}

	ctx := r.Context()

	var req replication.PullRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	entries, err := s.replicationManager.GetEntries(ctx, req.AfterID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get entries")
		return
	}

	hasMore := len(entries) >= limit

	writeJSON(w, http.StatusOK, replication.PullResponse{
		Entries: entries,
		HasMore: hasMore,
	})
}

// handleReplicationStatus returns the replication status.
// GET /api/v1/replication/status
func (s *Server) handleReplicationStatus(w http.ResponseWriter, r *http.Request) {
	if s.replicationManager == nil {
		writeError(w, http.StatusServiceUnavailable, "replication not available")
		return
	}

	ctx := r.Context()

	status, err := s.replicationManager.Status(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get replication status")
		return
	}

	writeJSON(w, http.StatusOK, status)
}
