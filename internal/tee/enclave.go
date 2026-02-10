// Package tee implements a Trusted Execution Environment (TEE) confidential data plane.
// It provides enclave-based decryption, attestation, and secure channels for reading
// secrets without exposing plaintext outside the enclave boundary.
package tee

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/teamvault/teamvault/internal/crypto"
)

// EnclaveService defines the interface for a TEE enclave.
// Implementations range from full hardware TEE (SGX/SEV/TrustZone) to
// a software-only fallback for development.
type EnclaveService interface {
	// Decrypt performs envelope decryption within the enclave boundary.
	Decrypt(ciphertext, encryptedDEK, nonce, dekNonce []byte) ([]byte, error)
	// Attest produces cryptographic evidence of the enclave's identity and integrity.
	Attest() (AttestationEvidence, error)
	// SessionKey returns a fresh ephemeral key for establishing a secure channel.
	SessionKey() ([]byte, error)
}

// AttestationEvidence contains cryptographic proof of enclave identity.
type AttestationEvidence struct {
	// Measurement is a hash of the enclave code/configuration (analogous to MRENCLAVE).
	Measurement string `json:"measurement"`
	// Timestamp is when the attestation was produced.
	Timestamp time.Time `json:"timestamp"`
	// SessionKeyHash is SHA-256 of the current session key, binding it to the attestation.
	SessionKeyHash string `json:"session_key_hash"`
	// HardwareSig is a signature from the TEE hardware (empty in software mode).
	HardwareSig string `json:"hardware_sig"`
	// Platform identifies the TEE platform (e.g., "sgx", "sev", "software").
	Platform string `json:"platform"`
}

// AttestationVerifier verifies attestation evidence against known-good measurements.
type AttestationVerifier struct {
	mu                sync.RWMutex
	allowedMeasurements map[string]bool
	maxAgeDuration    time.Duration
}

// NewAttestationVerifier creates a verifier with allowed measurement hashes.
func NewAttestationVerifier(measurements []string, maxAge time.Duration) *AttestationVerifier {
	allowed := make(map[string]bool, len(measurements))
	for _, m := range measurements {
		allowed[m] = true
	}
	if maxAge == 0 {
		maxAge = 5 * time.Minute
	}
	return &AttestationVerifier{
		allowedMeasurements: allowed,
		maxAgeDuration:      maxAge,
	}
}

// Verify checks that the attestation evidence is valid:
// - The measurement is in the allowlist
// - The timestamp is recent (within maxAge)
// - The session key hash is non-empty
func (v *AttestationVerifier) Verify(evidence AttestationEvidence) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check measurement against allowlist
	if len(v.allowedMeasurements) > 0 && !v.allowedMeasurements[evidence.Measurement] {
		return fmt.Errorf("measurement %s not in allowlist", evidence.Measurement)
	}

	// Check freshness
	age := time.Since(evidence.Timestamp)
	if age > v.maxAgeDuration {
		return fmt.Errorf("attestation too old: %v (max %v)", age, v.maxAgeDuration)
	}
	if age < -30*time.Second {
		return fmt.Errorf("attestation timestamp is in the future")
	}

	// Verify session key hash is present
	if evidence.SessionKeyHash == "" {
		return fmt.Errorf("attestation missing session key hash")
	}

	return nil
}

// AddMeasurement adds a trusted measurement to the allowlist.
func (v *AttestationVerifier) AddMeasurement(measurement string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.allowedMeasurements[measurement] = true
}

// RemoveMeasurement removes a measurement from the allowlist.
func (v *AttestationVerifier) RemoveMeasurement(measurement string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.allowedMeasurements, measurement)
}

// ---- SoftwareEnclave: non-TEE fallback ----

// SoftwareEnclave is a software-only implementation of EnclaveService.
// It uses the existing EnvelopeCrypto for decryption and produces
// software-mode attestation evidence. Suitable for development/testing.
type SoftwareEnclave struct {
	crypto      *crypto.EnvelopeCrypto
	measurement string
	sessionKey  []byte
	mu          sync.Mutex
}

// NewSoftwareEnclave creates a software enclave backed by EnvelopeCrypto.
func NewSoftwareEnclave(c *crypto.EnvelopeCrypto) (*SoftwareEnclave, error) {
	// Generate a stable measurement from the build (simulated in software mode).
	measurement := computeSoftwareMeasurement()

	// Generate initial session key
	sessionKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessionKey); err != nil {
		return nil, fmt.Errorf("generating session key: %w", err)
	}

	return &SoftwareEnclave{
		crypto:      c,
		measurement: measurement,
		sessionKey:  sessionKey,
	}, nil
}

// Decrypt performs envelope decryption inside the (simulated) enclave boundary.
func (se *SoftwareEnclave) Decrypt(ciphertext, encryptedDEK, nonce, dekNonce []byte) ([]byte, error) {
	data := &crypto.EncryptedData{
		Ciphertext:       ciphertext,
		Nonce:            nonce,
		EncryptedDEK:     encryptedDEK,
		DEKNonce:         dekNonce,
		MasterKeyVersion: 1,
	}
	return se.crypto.Decrypt(data)
}

// Attest produces attestation evidence for the software enclave.
func (se *SoftwareEnclave) Attest() (AttestationEvidence, error) {
	se.mu.Lock()
	keyHash := sha256.Sum256(se.sessionKey)
	se.mu.Unlock()

	// In software mode, hardware_sig is a self-signed hash (not a real hardware attestation).
	sigInput := fmt.Sprintf("%s:%s:%s", se.measurement, hex.EncodeToString(keyHash[:]), time.Now().UTC().Format(time.RFC3339Nano))
	sig := sha256.Sum256([]byte(sigInput))

	return AttestationEvidence{
		Measurement:    se.measurement,
		Timestamp:      time.Now().UTC(),
		SessionKeyHash: hex.EncodeToString(keyHash[:]),
		HardwareSig:    hex.EncodeToString(sig[:]),
		Platform:       "software",
	}, nil
}

// SessionKey returns the current session key.
func (se *SoftwareEnclave) SessionKey() ([]byte, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	keyCopy := make([]byte, len(se.sessionKey))
	copy(keyCopy, se.sessionKey)
	return keyCopy, nil
}

// RotateSessionKey generates a fresh session key.
func (se *SoftwareEnclave) RotateSessionKey() error {
	se.mu.Lock()
	defer se.mu.Unlock()

	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return fmt.Errorf("generating session key: %w", err)
	}

	// Zero old key
	for i := range se.sessionKey {
		se.sessionKey[i] = 0
	}
	se.sessionKey = newKey
	return nil
}

// Measurement returns the enclave's measurement hash.
func (se *SoftwareEnclave) Measurement() string {
	return se.measurement
}

// computeSoftwareMeasurement generates a deterministic measurement for the software enclave.
// In a real TEE, this would be MRENCLAVE or a launch digest.
func computeSoftwareMeasurement() string {
	// Use a fixed identifier for the software enclave build.
	// In production, this would incorporate the binary hash.
	h := sha256.Sum256([]byte("teamvault-software-enclave-v1"))
	return hex.EncodeToString(h[:])
}
