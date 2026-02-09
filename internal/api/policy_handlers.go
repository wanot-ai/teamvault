package api

import (
	"encoding/json"
	"net/http"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/db"
)

type createPolicyRequest struct {
	Name            string          `json:"name"`
	Effect          string          `json:"effect"`
	Actions         []string        `json:"actions"`
	ResourcePattern string          `json:"resource_pattern"`
	SubjectType     string          `json:"subject_type"`
	SubjectID       *string         `json:"subject_id"`
	Conditions      json.RawMessage `json:"conditions"`
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	var req createPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || len(req.Actions) == 0 || req.ResourcePattern == "" || req.SubjectType == "" {
		writeError(w, http.StatusBadRequest, "name, actions, resource_pattern, and subject_type are required")
		return
	}

	if req.Effect == "" {
		req.Effect = "allow"
	}
	if req.Effect != "allow" && req.Effect != "deny" {
		writeError(w, http.StatusBadRequest, "effect must be 'allow' or 'deny'")
		return
	}
	if req.SubjectType != "user" && req.SubjectType != "service_account" {
		writeError(w, http.StatusBadRequest, "subject_type must be 'user' or 'service_account'")
		return
	}

	pol, err := s.db.CreatePolicy(r.Context(), req.Name, req.Effect, req.Actions,
		req.ResourcePattern, req.SubjectType, req.SubjectID, req.Conditions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create policy")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "policy.create",
		Resource:  "policy:" + pol.ID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusCreated, pol)
}

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := s.db.ListPolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list policies")
		return
	}

	if policies == nil {
		policies = []db.Policy{}
	}

	writeJSON(w, http.StatusOK, policies)
}
