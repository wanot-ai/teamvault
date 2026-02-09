package api

import (
	"encoding/json"
	"net/http"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/lease"
)

type issueDatabaseLeaseRequest struct {
	TTLSeconds int    `json:"ttl_seconds"` // Default: 3600 (1h)
	OrgID      string `json:"org_id,omitempty"`
}

func (s *Server) handleIssueDatabaseLease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	if s.leaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "lease manager not available")
		return
	}

	var req issueDatabaseLeaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var orgID *string
	if req.OrgID != "" {
		orgID = &req.OrgID
	}

	leaseResp, err := s.leaseManager.IssueDatabaseLease(ctx, actorID, req.TTLSeconds, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue lease")
		return
	}

	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "lease.issue",
		Resource:  "dynamic/database",
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"lease_id":"` + leaseResp.LeaseID + `","ttl":` + itoa(leaseResp.TTL) + `}`),
	})

	writeJSON(w, http.StatusOK, leaseResp)
}

func (s *Server) handleRevokeLease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	leaseID := r.PathValue("id")
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	if leaseID == "" {
		writeError(w, http.StatusBadRequest, "lease id is required")
		return
	}

	if s.leaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "lease manager not available")
		return
	}

	if err := s.leaseManager.RevokeLease(ctx, leaseID); err != nil {
		writeError(w, http.StatusNotFound, "lease not found or already revoked")
		return
	}

	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "lease.revoke",
		Resource:  "lease:" + leaseID,
		Outcome:   "success",
		IP:        getClientIP(ctx),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) handleListLeases(w http.ResponseWriter, r *http.Request) {
	if s.leaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "lease manager not available")
		return
	}

	leases, err := s.leaseManager.ListActiveLeases(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list leases")
		return
	}

	if leases == nil {
		leases = []lease.LeaseInfo{}
	}

	writeJSON(w, http.StatusOK, leases)
}
