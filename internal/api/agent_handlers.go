package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/db"
)

type createAgentRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Scopes      []string        `json:"scopes"`
	Metadata    json.RawMessage `json:"metadata"`
	ExpiresIn   string          `json:"expires_in"` // e.g., "24h", "720h"
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	teamID := r.PathValue("id")
	if teamID == "" {
		writeError(w, http.StatusBadRequest, "team id is required")
		return
	}

	// Verify the team exists
	_, err := s.db.GetTeamByID(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	var req createAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if len(req.Scopes) == 0 {
		req.Scopes = []string{"read"}
	}

	// Generate agent token (reusing the service account token generation)
	rawToken, tokenHash, err := s.auth.GenerateServiceAccountToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate agent token")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		duration, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid expires_in duration")
			return
		}
		t := time.Now().Add(duration)
		expiresAt = &t
	}

	agent, err := s.db.CreateAgent(r.Context(), teamID, req.Name, req.Description, tokenHash, req.Scopes, req.Metadata, claims.UserID, expiresAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "agent name already exists in this team")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "agent.create",
		Resource:  "agent:" + agent.ID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	// Return the agent with the raw token (only shown once)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"agent": agent,
		"token": "agent." + rawToken, // prefix to distinguish from SA tokens
	})
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("id")
	if teamID == "" {
		writeError(w, http.StatusBadRequest, "team id is required")
		return
	}

	agents, err := s.db.ListAgentsByTeam(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}

	if agents == nil {
		agents = []db.Agent{}
	}

	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	agent, err := s.db.GetAgentByID(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, agent)
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	agentID := r.PathValue("agentId")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	if !isValidUUID(agentID) {
		writeError(w, http.StatusBadRequest, "agent id must be a valid UUID")
		return
	}

	if err := s.db.DeleteAgent(r.Context(), agentID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		if isDBInvalidInputError(err) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "agent.delete",
		Resource:  "agent:" + agentID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
