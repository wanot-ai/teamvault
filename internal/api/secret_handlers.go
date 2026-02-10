package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/crypto"
	"github.com/teamvault/teamvault/internal/db"
	"github.com/teamvault/teamvault/internal/policy"
)

type putSecretRequest struct {
	Value       string `json:"value"`
	Description string `json:"description"`
	// File/blob secret fields
	Type        string `json:"type,omitempty"`         // "kv", "json", "file" (default: "kv")
	Filename    string `json:"filename,omitempty"`      // For file type
	ContentType string `json:"content_type,omitempty"`  // For file type (e.g., "application/x-pem-file")
}

type secretResponse struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	Project     string          `json:"project"`
	Path        string          `json:"path"`
	Description string          `json:"description,omitempty"`
	SecretType  string          `json:"secret_type"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Version     int             `json:"version"`
	Value       string          `json:"value,omitempty"` // Only present on read
	CreatedBy   string          `json:"created_by"`
}

// fileMetadata is stored in the secret's metadata column for file-type secrets.
type fileMetadata struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
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

	// Redirect rotation PUTs to the rotation handler
	if strings.HasSuffix(secretPath, "/rotation") {
		s.handleSetRotation(w, r)
		return
	}
	if strings.HasSuffix(secretPath, "/rotate") {
		s.handleManualRotate(w, r)
		return
	}

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

	// Determine secret type
	secretType := req.Type
	if secretType == "" {
		secretType = "kv"
	}
	if secretType != "kv" && secretType != "json" && secretType != "file" {
		writeError(w, http.StatusBadRequest, "type must be 'kv', 'json', or 'file'")
		return
	}

	// For file type, filename is required
	if secretType == "file" && req.Filename == "" {
		writeError(w, http.StatusBadRequest, "filename is required for file type secrets")
		return
	}

	// Build metadata for file type
	var metadata json.RawMessage
	if secretType == "file" {
		ct := req.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		fm := fileMetadata{
			Filename:    req.Filename,
			ContentType: ct,
		}
		metadata, _ = json.Marshal(fm)
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
		// Create a new secret with type
		secret, err = s.db.CreateSecretWithType(ctx, project.ID, secretPath, req.Description, secretType, metadata, actorID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create secret")
			return
		}
	} else {
		// Update existing secret's type and metadata if changed
		if secretType != secret.SecretType || metadata != nil {
			if err := s.db.UpdateSecretType(ctx, secret.ID, secretType, metadata); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update secret type")
				return
			}
			secret.SecretType = secretType
			secret.Metadata = metadata
		}
	}

	// Encrypt the secret value using envelope encryption
	encrypted, err := s.crypto.Encrypt([]byte(req.Value))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	// Get next version number and create the version.
	// Retry on conflict (concurrent writes can race on version number).
	var sv *db.SecretVersion
	for attempts := 0; attempts < 3; attempts++ {
		nextVersion, err := s.db.GetNextSecretVersion(ctx, secret.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to determine version")
			return
		}

		sv, err = s.db.CreateSecretVersion(ctx, secret.ID, nextVersion,
			encrypted.Ciphertext, encrypted.Nonce,
			encrypted.EncryptedDEK, encrypted.DEKNonce,
			encrypted.MasterKeyVersion, actorID)
		if err != nil {
			if isDBConflictError(err) && attempts < 2 {
				continue // retry with next version number
			}
			if isDBConflictError(err) {
				writeError(w, http.StatusConflict, "concurrent write conflict, please retry")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to store secret version")
			return
		}
		break
	}

	// Audit the write (NEVER log the secret value)
	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "secret.write",
		Resource:  resource,
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"version":` + itoa(sv.Version) + `,"type":"` + secretType + `"}`),
	})

	writeJSON(w, http.StatusOK, secretResponse{
		ID:          secret.ID,
		ProjectID:   project.ID,
		Project:     project.Name,
		Path:        secret.Path,
		Description: secret.Description,
		SecretType:  secretType,
		Metadata:    metadata,
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

	// Check if this is a rotation request (POST only, handled elsewhere)
	if strings.HasSuffix(secretPath, "/rotation") || strings.HasSuffix(secretPath, "/rotate") {
		writeError(w, http.StatusMethodNotAllowed, "use POST for rotation endpoints")
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

	// For file-type secrets, set content-type header if metadata contains it
	if secret.SecretType == "file" && secret.Metadata != nil {
		var fm fileMetadata
		if err := json.Unmarshal(secret.Metadata, &fm); err == nil && fm.ContentType != "" {
			w.Header().Set("X-Secret-Content-Type", fm.ContentType)
			w.Header().Set("X-Secret-Filename", fm.Filename)
		}
	}

	writeJSON(w, http.StatusOK, secretResponse{
		ID:          secret.ID,
		ProjectID:   project.ID,
		Project:     project.Name,
		Path:        secret.Path,
		Description: secret.Description,
		SecretType:  secret.SecretType,
		Metadata:    secret.Metadata,
		Version:     sv.Version,
		Value:       string(plaintext),
		CreatedBy:   sv.CreatedBy,
	})
}

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Policy check: require "list" or "read" permission on the project
	policyResult, err := s.policy.Evaluate(ctx, policy.Request{
		SubjectType: actorType,
		SubjectID:   actorID,
		Action:      "read",
		Resource:    projectName + "/*",
		IsAdmin:     isAdmin(ctx),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy evaluation failed")
		return
	}
	if !policyResult.Allowed {
		// Fall back: allow if user created the project
		claims := getUserClaims(ctx)
		if claims == nil || (project.CreatedBy != claims.UserID && claims.Role != "admin") {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	secrets, err := s.db.ListSecrets(ctx, project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list secrets")
		return
	}

	// Return metadata only (no values)
	type secretListItem struct {
		ID          string          `json:"id"`
		Path        string          `json:"path"`
		Description string          `json:"description,omitempty"`
		SecretType  string          `json:"secret_type"`
		Metadata    json.RawMessage `json:"metadata,omitempty"`
		CreatedBy   string          `json:"created_by"`
		CreatedAt   string          `json:"created_at"`
	}

	items := make([]secretListItem, 0, len(secrets))
	for _, sec := range secrets {
		items = append(items, secretListItem{
			ID:          sec.ID,
			Path:        sec.Path,
			Description: sec.Description,
			SecretType:  sec.SecretType,
			Metadata:    sec.Metadata,
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
