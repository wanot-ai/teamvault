package api

import (
	"encoding/json"
	"net/http"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/zk"
)

// ZKHandlers holds ZK-related dependencies.
type ZKHandlers struct {
	authority *zk.ZKAuthority
}

// NewZKHandlers creates ZK handlers.
func NewZKHandlers(authority *zk.ZKAuthority) *ZKHandlers {
	return &ZKHandlers{authority: authority}
}

// zkCredentialRequest is the request for issuing a ZK credential.
type zkCredentialRequest struct {
	Team string `json:"team"`
	MFA  string `json:"mfa"` // "enabled" or "disabled"
}

// zkCredentialResponse contains the issued credential.
type zkCredentialResponse struct {
	Credential zk.Credential `json:"credential"`
	PublicKey  zk.PublicKey  `json:"public_key"`
}

// handleZKCredential issues a ZK credential after JWT authentication.
// POST /api/v1/auth/zk/credential
func (s *Server) handleZKCredential(w http.ResponseWriter, r *http.Request) {
	if s.zkHandlers == nil || s.zkHandlers.authority == nil {
		writeError(w, http.StatusServiceUnavailable, "ZK auth not available")
		return
	}

	ctx := r.Context()
	claims := getUserClaims(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "JWT authentication required for ZK credential issuance")
		return
	}

	var req zkCredentialRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Team == "" {
		req.Team = "default"
	}
	if req.MFA == "" {
		req.MFA = "disabled"
	}

	// Issue BBS+ credential with user's claims
	cred, err := s.zkHandlers.authority.IssueZKCredential(
		claims.UserID,
		claims.Role,
		req.Team,
		req.MFA,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue credential")
		return
	}

	// Audit credential issuance
	s.audit.Log(ctx, audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "zk.credential_issued",
		Resource:  "zk/credential",
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"team":"` + req.Team + `"}`),
	})

	writeJSON(w, http.StatusOK, zkCredentialResponse{
		Credential: cred,
		PublicKey:  s.zkHandlers.authority.PublicKey(),
	})
}

// zkVerifyRequest is the request for verifying a ZK proof.
type zkVerifyRequest struct {
	Proof          zk.Proof `json:"proof"`
	RequiredClaims []string `json:"required_claims"`
}

// zkVerifyResponse contains the verification result.
type zkVerifyResponse struct {
	Valid           bool       `json:"valid"`
	DisclosedClaims []zk.Claim `json:"disclosed_claims,omitempty"`
}

// handleZKVerify verifies a ZK selective disclosure proof.
// POST /api/v1/auth/zk/verify
func (s *Server) handleZKVerify(w http.ResponseWriter, r *http.Request) {
	if s.zkHandlers == nil || s.zkHandlers.authority == nil {
		writeError(w, http.StatusServiceUnavailable, "ZK auth not available")
		return
	}

	var req zkVerifyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.RequiredClaims) == 0 {
		writeError(w, http.StatusBadRequest, "required_claims must be specified")
		return
	}

	valid := s.zkHandlers.authority.VerifyZKToken(req.Proof, req.RequiredClaims)

	ctx := r.Context()
	outcome := "success"
	if !valid {
		outcome = "denied"
	}

	// Audit verification attempt
	reqClaimsJSON, _ := json.Marshal(req.RequiredClaims)
	s.audit.Log(ctx, audit.Event{
		ActorType: "system",
		ActorID:   "zk_verifier",
		Action:    "zk.verify",
		Resource:  "zk/proof",
		Outcome:   outcome,
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"required_claims":` + string(reqClaimsJSON) + `}`),
	})

	resp := zkVerifyResponse{
		Valid: valid,
	}
	if valid {
		resp.DisclosedClaims = req.Proof.DisclosedClaims
	}

	writeJSON(w, http.StatusOK, resp)
}
