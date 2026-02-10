package api

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/tee"
)

// TEEHandlers holds the TEE-related dependencies.
type TEEHandlers struct {
	enclave  tee.EnclaveService
	verifier *tee.AttestationVerifier
	mu       sync.RWMutex
	channels map[string]*tee.SecureChannel // session_key_hash -> channel
}

// NewTEEHandlers creates TEE handlers with the given enclave and verifier.
func NewTEEHandlers(enclave tee.EnclaveService, verifier *tee.AttestationVerifier) *TEEHandlers {
	return &TEEHandlers{
		enclave:  enclave,
		verifier: verifier,
		channels: make(map[string]*tee.SecureChannel),
	}
}

// handleTEEAttestation returns the enclave's attestation evidence.
// GET /api/v1/tee/attestation
func (s *Server) handleTEEAttestation(w http.ResponseWriter, r *http.Request) {
	if s.teeHandlers == nil || s.teeHandlers.enclave == nil {
		writeError(w, http.StatusServiceUnavailable, "TEE not available")
		return
	}

	evidence, err := s.teeHandlers.enclave.Attest()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "attestation failed")
		return
	}

	writeJSON(w, http.StatusOK, evidence)
}

// teeSessionRequest is the request body for establishing a secure channel.
type teeSessionRequest struct {
	ClientPubKey string `json:"client_pub_key"` // Hex-encoded ECDH public key
}

// teeSessionResponse is the response from session establishment.
type teeSessionResponse struct {
	ServerPubKey   string                   `json:"server_pub_key"`    // Hex-encoded server ECDH public key
	SessionKeyHash string                   `json:"session_key_hash"`  // For session tracking
	ExpiresAt      string                   `json:"expires_at"`
	Attestation    tee.AttestationEvidence   `json:"attestation"`
}

// handleTEESession establishes an attested secure channel.
// POST /api/v1/tee/session
func (s *Server) handleTEESession(w http.ResponseWriter, r *http.Request) {
	if s.teeHandlers == nil || s.teeHandlers.enclave == nil {
		writeError(w, http.StatusServiceUnavailable, "TEE not available")
		return
	}

	var req teeSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ClientPubKey == "" {
		writeError(w, http.StatusBadRequest, "client_pub_key is required")
		return
	}

	clientPubKeyBytes, err := hex.DecodeString(req.ClientPubKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client_pub_key hex encoding")
		return
	}

	// Get attestation evidence
	evidence, err := s.teeHandlers.enclave.Attest()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "attestation failed")
		return
	}

	// Create secure channel bound to attestation
	channel, err := tee.NewSecureChannel(evidence)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create secure channel")
		return
	}

	// Complete key exchange with client's public key
	if err := channel.EstablishWithPeerKey(clientPubKeyBytes); err != nil {
		writeError(w, http.StatusBadRequest, "key exchange failed")
		return
	}

	// Store the channel
	sessionHash := channel.SessionKeyHash()
	s.teeHandlers.mu.Lock()
	s.teeHandlers.channels[sessionHash] = channel
	s.teeHandlers.mu.Unlock()

	// Store session in DB
	ctx := r.Context()
	_, err = s.db.Pool.Exec(ctx,
		`INSERT INTO tee_sessions (session_key_hash, client_pubkey, expires_at)
		 VALUES ($1, $2, $3)`,
		sessionHash, req.ClientPubKey, channel.ExpiresAt(),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store session")
		return
	}

	writeJSON(w, http.StatusOK, teeSessionResponse{
		ServerPubKey:   channel.PublicKeyHex(),
		SessionKeyHash: sessionHash,
		ExpiresAt:      channel.ExpiresAt().Format("2006-01-02T15:04:05Z"),
		Attestation:    evidence,
	})
}

// teeReadRequest is the request body for reading a secret through TEE.
type teeReadRequest struct {
	SessionKeyHash string `json:"session_key_hash"`
	Project        string `json:"project"`
	Path           string `json:"path"`
}

// teeReadResponse contains the encrypted secret value.
type teeReadResponse struct {
	EncryptedValue string `json:"encrypted_value"` // Base64-encoded encrypted payload
	Version        int    `json:"version"`
}

// handleTEERead reads a secret through the TEE secure channel.
// The secret is decrypted inside the enclave, then re-encrypted through the secure channel.
// POST /api/v1/tee/read
func (s *Server) handleTEERead(w http.ResponseWriter, r *http.Request) {
	if s.teeHandlers == nil || s.teeHandlers.enclave == nil {
		writeError(w, http.StatusServiceUnavailable, "TEE not available")
		return
	}

	ctx := r.Context()

	var req teeReadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SessionKeyHash == "" || req.Project == "" || req.Path == "" {
		writeError(w, http.StatusBadRequest, "session_key_hash, project, and path are required")
		return
	}

	// Find the secure channel
	s.teeHandlers.mu.RLock()
	channel, ok := s.teeHandlers.channels[req.SessionKeyHash]
	s.teeHandlers.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	if channel.IsExpired() {
		s.teeHandlers.mu.Lock()
		delete(s.teeHandlers.channels, req.SessionKeyHash)
		s.teeHandlers.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "session expired")
		return
	}

	// Get the secret from DB
	project, err := s.db.GetProjectByName(ctx, req.Project)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	secret, err := s.db.GetSecret(ctx, project.ID, req.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	sv, err := s.db.GetLatestSecretVersion(ctx, secret.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no versions found")
		return
	}

	// Decrypt inside the enclave
	plaintext, err := s.teeHandlers.enclave.Decrypt(
		sv.Ciphertext, sv.EncryptedDEK, sv.Nonce, sv.DEKNonce,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "enclave decryption failed")
		return
	}

	// Re-encrypt through the secure channel
	encryptedPayload, err := channel.Encrypt(plaintext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "channel encryption failed")
		return
	}

	// Zero plaintext immediately
	for i := range plaintext {
		plaintext[i] = 0
	}

	// Audit the TEE read
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)
	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "secret.tee_read",
		Resource:  req.Project + "/" + req.Path,
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"version":` + itoa(sv.Version) + `,"tee":"true"}`),
	})

	writeJSON(w, http.StatusOK, teeReadResponse{
		EncryptedValue: base64.StdEncoding.EncodeToString(encryptedPayload),
		Version:        sv.Version,
	})
}
