package api

import (
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/db"
)

type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	var req createProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	project, err := s.db.CreateProject(r.Context(), req.Name, req.Description, claims.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "project name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "project.create",
		Resource:  project.Name,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := getUserClaims(ctx)

	projects, err := s.db.ListProjects(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	if projects == nil {
		projects = []db.Project{}
	}

	// Non-admin users can only see projects they created
	if claims != nil && claims.Role != "admin" {
		filtered := make([]db.Project, 0)
		for _, p := range projects {
			if p.CreatedBy == claims.UserID {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	writeJSON(w, http.StatusOK, projects)
}
