package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/crypto"
	"github.com/teamvault/teamvault/internal/policy"
)

type putSecretRequest struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

type secretResponse struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Project     string `json:"project"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Version     int    `json:"version"`
	Value       string `json:"value,omitempty"` // Only present on read
	CreatedBy   string `json:"created_by"`
}

func (s *Server) handlePutSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")
	secretPath := r.PathValue("path")
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	if projectName == "" || secretPath == "" {
		writeError(w, http.StatusBadRequest, "project and path are required")
		return
	}

	// Strip /versions suffix if accidentally routed here
	secretPath = strings.TrimSuffix(secretPath, "/versions")

	resource := projectName + "/" + secretPath

	// Policy check
	policyResult, err := s.policy.Evaluate(ctx, policy.Request{
		SubjectType: actorType,
		SubjectID:   actorID,
		Action:      "write",
		Resource:    resource,
		IsAdmin:     isAdmin(ctx),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy evaluation failed")
		return
	}
	if !policyResult.Allowed {
		s.audit.Log(ctx, audit.Event{
			ActorType: actorType,
			ActorID:   actorID,
			Action:    "secret.write",
			Resource:  resource,
			Outcome:   "denied",
			IP:        getClientIP(ctx),
			Metadata:  json.RawMessage(`{"reason":"` + policyResult.Reason + `"}`),
		})
		writeError(w, http.StatusForbidden, policyResult.Reason)
		return
	}

	// Check SA scope
	if saClaims := getSAClaims(ctx); saClaims != nil {
		if !hasScope(saClaims.Scopes, "write") {
			writeError(w, http.StatusForbidden, "service account lacks write scope")
			return
		}
	}

	var req putSecretRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Value == "" {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	// Get or create the project
	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Get or create the secret
	secret, err := s.db.GetSecret(ctx, project.ID, secretPath)
	if err != nil {
		// Create a new secret
		secret, err = s.db.CreateSecret(ctx, project.ID, secretPath, req.Description, actorID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create secret")
			return
		}
	}

	// Encrypt the secret value using envelope encryption
	encrypted, err := s.crypto.Encrypt([]byte(req.Value))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	// Get next version number
	nextVersion, err := s.db.GetNextSecretVersion(ctx, secret.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to determine version")
		return
	}

	// Store encrypted version
	sv, err := s.db.CreateSecretVersion(ctx, secret.ID, nextVersion,
		encrypted.Ciphertext, encrypted.Nonce,
		encrypted.EncryptedDEK, encrypted.DEKNonce,
		encrypted.MasterKeyVersion, actorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store secret version")
		return
	}

	// Audit the write (NEVER log the secret value)
	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "secret.write",
		Resource:  resource,
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"version":` + itoa(sv.Version) + `}`),
	})

	writeJSON(w, http.StatusOK, secretResponse{
		ID:          secret.ID,
		ProjectID:   project.ID,
		Project:     project.Name,
		Path:        secret.Path,
		Description: secret.Description,
		Version:     sv.Version,
		CreatedBy:   actorID,
	})
}

func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")
	secretPath := r.PathValue("path")
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	// Check if this is actually a versions request
	if strings.HasSuffix(secretPath, "/versions") {
		s.handleListSecretVersions(w, r)
		return
	}

	if projectName == "" || secretPath == "" {
		writeError(w, http.StatusBadRequest, "project and path are required")
		return
	}

	resource := projectName + "/" + secretPath

	// Policy check
	policyResult, err := s.policy.Evaluate(ctx, policy.Request{
		SubjectType: actorType,
		SubjectID:   actorID,
		Action:      "read",
		Resource:    resource,
		IsAdmin:     isAdmin(ctx),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy evaluation failed")
		return
	}
	if !policyResult.Allowed {
		s.audit.Log(ctx, audit.Event{
			ActorType: actorType,
			ActorID:   actorID,
			Action:    "secret.read",
			Resource:  resource,
			Outcome:   "denied",
			IP:        getClientIP(ctx),
			Metadata:  json.RawMessage(`{"reason":"` + policyResult.Reason + `"}`),
		})
		writeError(w, http.StatusForbidden, policyResult.Reason)
		return
	}

	// Check SA scope
	if saClaims := getSAClaims(ctx); saClaims != nil {
		if !hasScope(saClaims.Scopes, "read") {
			writeError(w, http.StatusForbidden, "service account lacks read scope")
			return
		}
	}

	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	secret, err := s.db.GetSecret(ctx, project.ID, secretPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	sv, err := s.db.GetLatestSecretVersion(ctx, secret.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no versions found")
		return
	}

	// Decrypt the secret value
	plaintext, err := s.crypto.Decrypt(&crypto.EncryptedData{
		Ciphertext:       sv.Ciphertext,
		Nonce:            sv.Nonce,
		EncryptedDEK:     sv.EncryptedDEK,
		DEKNonce:         sv.DEKNonce,
		MasterKeyVersion: sv.MasterKeyVersion,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decryption failed")
		return
	}

	// Audit the read (NEVER log the secret value)
	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "secret.read",
		Resource:  resource,
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"version":` + itoa(sv.Version) + `}`),
	})

	writeJSON(w, http.StatusOK, secretResponse{
		ID:          secret.ID,
		ProjectID:   project.ID,
		Project:     project.Name,
		Path:        secret.Path,
		Description: secret.Description,
		Version:     sv.Version,
		Value:       string(plaintext),
		CreatedBy:   sv.CreatedBy,
	})
}

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")

	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	secrets, err := s.db.ListSecrets(ctx, project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list secrets")
		return
	}

	// Return metadata only (no values)
	type secretListItem struct {
		ID          string `json:"id"`
		Path        string `json:"path"`
		Description string `json:"description,omitempty"`
		CreatedBy   string `json:"created_by"`
		CreatedAt   string `json:"created_at"`
	}

	items := make([]secretListItem, 0, len(secrets))
	for _, sec := range secrets {
		items = append(items, secretListItem{
			ID:          sec.ID,
			Path:        sec.Path,
			Description: sec.Description,
			CreatedBy:   sec.CreatedBy,
			CreatedAt:   sec.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")
	secretPath := r.PathValue("path")
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	resource := projectName + "/" + secretPath

	// Policy check
	policyResult, err := s.policy.Evaluate(ctx, policy.Request{
		SubjectType: actorType,
		SubjectID:   actorID,
		Action:      "delete",
		Resource:    resource,
		IsAdmin:     isAdmin(ctx),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy evaluation failed")
		return
	}
	if !policyResult.Allowed {
		s.audit.Log(ctx, audit.Event{
			ActorType: actorType,
			ActorID:   actorID,
			Action:    "secret.delete",
			Resource:  resource,
			Outcome:   "denied",
			IP:        getClientIP(ctx),
		})
		writeError(w, http.StatusForbidden, policyResult.Reason)
		return
	}

	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if err := s.db.SoftDeleteSecret(ctx, project.ID, secretPath); err != nil {
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "secret.delete",
		Resource:  resource,
		Outcome:   "success",
		IP:        getClientIP(ctx),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListSecretVersions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")
	secretPath := r.PathValue("path")

	// Strip /versions suffix
	secretPath = strings.TrimSuffix(secretPath, "/versions")

	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	secret, err := s.db.GetSecret(ctx, project.ID, secretPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	versions, err := s.db.ListSecretVersions(ctx, secret.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list versions")
		return
	}

	type versionItem struct {
		ID        string `json:"id"`
		Version   int    `json:"version"`
		CreatedBy string `json:"created_by"`
		CreatedAt string `json:"created_at"`
	}

	items := make([]versionItem, 0, len(versions))
	for _, v := range versions {
		items = append(items, versionItem{
			ID:        v.ID,
			Version:   v.Version,
			CreatedBy: v.CreatedBy,
			CreatedAt: v.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, http.StatusOK, items)
}

// hasScope checks if a scope list contains the requested scope.
func hasScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

// itoa converts an int to string without importing strconv in every file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
