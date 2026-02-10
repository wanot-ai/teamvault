package api

import (
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/db"
)

type createOrgRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	var req createOrgRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	org, err := s.db.CreateOrg(r.Context(), req.Name, req.Description, claims.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "organization name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create organization")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "org.create",
		Resource:  "org:" + org.ID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusCreated, org)
}

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := getUserClaims(ctx)

	orgs, err := s.db.ListOrgs(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}

	if orgs == nil {
		orgs = []db.Org{}
	}

	// Non-admin users can only see orgs they created
	if claims != nil && claims.Role != "admin" {
		filtered := make([]db.Org, 0)
		for _, o := range orgs {
			if o.CreatedBy == claims.UserID {
				filtered = append(filtered, o)
			}
		}
		orgs = filtered
	}

	writeJSON(w, http.StatusOK, orgs)
}

func (s *Server) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org id is required")
		return
	}

	org, err := s.db.GetOrgByID(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	writeJSON(w, http.StatusOK, org)
}
