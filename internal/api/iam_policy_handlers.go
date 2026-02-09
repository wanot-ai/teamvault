package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/db"
	"github.com/teamvault/teamvault/internal/policy"
)

type createIAMPolicyRequest struct {
	OrgID       string `json:"org_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PolicyType  string `json:"policy_type"`
	HCLSource   string `json:"hcl_source,omitempty"`
	// If hcl_source is provided, policy_doc is parsed from it.
	// If policy_doc is provided directly, it's used as-is.
	PolicyDoc json.RawMessage `json:"policy_doc,omitempty"`
}

type updateIAMPolicyRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	PolicyType  string          `json:"policy_type"`
	HCLSource   string          `json:"hcl_source,omitempty"`
	PolicyDoc   json.RawMessage `json:"policy_doc,omitempty"`
}

func (s *Server) handleCreateIAMPolicy(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	var req createIAMPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrgID == "" || req.Name == "" || req.PolicyType == "" {
		writeError(w, http.StatusBadRequest, "org_id, name, and policy_type are required")
		return
	}

	if req.PolicyType != "rbac" && req.PolicyType != "abac" && req.PolicyType != "pbac" {
		writeError(w, http.StatusBadRequest, "policy_type must be 'rbac', 'abac', or 'pbac'")
		return
	}

	// Verify the org exists
	_, err := s.db.GetOrgByID(r.Context(), req.OrgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	var policyDoc json.RawMessage
	hclSource := req.HCLSource

	// If HCL source is provided, parse it into a policy document
	if hclSource != "" {
		doc, err := policy.ParseHCL([]byte(hclSource), "inline.hcl")
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to parse HCL: "+err.Error())
			return
		}
		policyDoc, err = json.Marshal(doc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to marshal policy document")
			return
		}
	} else if req.PolicyDoc != nil {
		policyDoc = req.PolicyDoc
	} else {
		writeError(w, http.StatusBadRequest, "either hcl_source or policy_doc is required")
		return
	}

	pol, err := s.db.CreateIAMPolicy(r.Context(), req.OrgID, req.Name, req.Description, req.PolicyType, policyDoc, hclSource, claims.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "policy name already exists in this organization")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create IAM policy")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "iam_policy.create",
		Resource:  "iam_policy:" + pol.ID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusCreated, pol)
}

func (s *Server) handleListIAMPolicies(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")

	policies, err := s.db.ListIAMPolicies(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list IAM policies")
		return
	}

	if policies == nil {
		policies = []db.IAMPolicy{}
	}

	writeJSON(w, http.StatusOK, policies)
}

func (s *Server) handleGetIAMPolicy(w http.ResponseWriter, r *http.Request) {
	policyID := r.PathValue("id")
	if policyID == "" {
		writeError(w, http.StatusBadRequest, "policy id is required")
		return
	}

	pol, err := s.db.GetIAMPolicyByID(r.Context(), policyID)
	if err != nil {
		writeError(w, http.StatusNotFound, "IAM policy not found")
		return
	}

	writeJSON(w, http.StatusOK, pol)
}

func (s *Server) handleUpdateIAMPolicy(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	policyID := r.PathValue("id")
	if policyID == "" {
		writeError(w, http.StatusBadRequest, "policy id is required")
		return
	}

	// Verify policy exists
	existing, err := s.db.GetIAMPolicyByID(r.Context(), policyID)
	if err != nil {
		writeError(w, http.StatusNotFound, "IAM policy not found")
		return
	}

	var req updateIAMPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Use existing values as defaults
	name := req.Name
	if name == "" {
		name = existing.Name
	}
	description := req.Description
	policyType := req.PolicyType
	if policyType == "" {
		policyType = existing.PolicyType
	}

	if policyType != "rbac" && policyType != "abac" && policyType != "pbac" {
		writeError(w, http.StatusBadRequest, "policy_type must be 'rbac', 'abac', or 'pbac'")
		return
	}

	var policyDoc json.RawMessage
	hclSource := req.HCLSource

	if hclSource != "" {
		doc, err := policy.ParseHCL([]byte(hclSource), "inline.hcl")
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to parse HCL: "+err.Error())
			return
		}
		policyDoc, err = json.Marshal(doc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to marshal policy document")
			return
		}
	} else if req.PolicyDoc != nil {
		policyDoc = req.PolicyDoc
	} else {
		policyDoc = existing.PolicyDoc
		hclSource = existing.HCLSource
	}

	pol, err := s.db.UpdateIAMPolicy(r.Context(), policyID, name, description, policyType, policyDoc, hclSource)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update IAM policy")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "iam_policy.update",
		Resource:  "iam_policy:" + pol.ID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusOK, pol)
}

func (s *Server) handleDeleteIAMPolicy(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	policyID := r.PathValue("id")
	if policyID == "" {
		writeError(w, http.StatusBadRequest, "policy id is required")
		return
	}

	if err := s.db.DeleteIAMPolicy(r.Context(), policyID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "IAM policy not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete IAM policy")
		return
	}

	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "iam_policy.delete",
		Resource:  "iam_policy:" + policyID,
		Outcome:   "success",
		IP:        getClientIP(r.Context()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
