package api

import (
	"net/http"
)

// handleDashboardStats returns aggregate counts for the dashboard.
func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetDashboardStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get dashboard stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
