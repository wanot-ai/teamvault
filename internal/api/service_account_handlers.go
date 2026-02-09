package api

import (
	"net/http"
	"time"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/db"
)

type createServiceAccountRequest struct {
	Name      string   `json:"name"`
	ProjectID string   `json:"project_id"`
	Scopes    []string `json:"scopes"`
	TTL       string   `json:"ttl"` // e.g., "1h", "24h", "720h"
}

func (s *Server) handleCreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	var req createServiceAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "name and project_id are required")
		return
	}

	if len(req.Scopes) == 0 {
		req.Scopes = []string{"read"}
	}

	// Validate project exists
	_, err := s.db.GetProjectByID(r.Context(), req.ProjectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Generate token
	rawToken, tokenHash, err := s.auth.GenerateServiceAccountToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Parse TTL
	var expiresAt *time.Time
	if req.TTL != "" {
		duration, err := time.ParseDuration(req.TTL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid TTL format")
			return
		}
		t := time.Now().Add(duration)
		expiresAt = &t
	}

	sa, err := s.db.CreateServiceAccount(r.Context(), req.Name, tokenHash, req.ProjectID, req.Scopes, claims.UserID, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create service account")
		return
	}

	// Audit
	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "service_account.create",
		Resource:  "sa:" + sa.ID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	// Return the token only once - it can never be retrieved again
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"service_account": sa,
		"token":           "sa." + rawToken, // Prefixed so the auth middleware can identify it
	})
}

func (s *Server) handleListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.db.ListServiceAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list service accounts")
		return
	}

	if accounts == nil {
		accounts = []db.ServiceAccount{}
	}

	writeJSON(w, http.StatusOK, accounts)
}
