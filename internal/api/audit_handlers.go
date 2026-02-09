package api

import (
	"net/http"
	"strconv"

	"github.com/teamvault/teamvault/internal/db"
)

func (s *Server) handleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	q := db.AuditQuery{
		ActorType: r.URL.Query().Get("actor_type"),
		ActorID:   r.URL.Query().Get("actor_id"),
		Action:    r.URL.Query().Get("action"),
		Resource:  r.URL.Query().Get("resource"),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			q.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			q.Offset = offset
		}
	}

	events, err := s.db.ListAuditEvents(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit events")
		return
	}

	if events == nil {
		events = []db.AuditEvent{}
	}

	writeJSON(w, http.StatusOK, events)
}
