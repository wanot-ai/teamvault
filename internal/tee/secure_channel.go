package tee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

// SecureChannel implements an attested secure channel using ECDH key exchange
// with attestation binding. The shared secret is derived from ECDH + attestation
// measurement, ensuring the channel is bound to a specific enclave identity.
type SecureChannel struct {
	mu            sync.Mutex
	privateKey    *ecdh.PrivateKey
	sharedSecret  []byte
	sendCipher    cipher.AEAD
	recvCipher    cipher.AEAD
	evidence      AttestationEvidence
	established   bool
	createdAt     time.Time
	expiresAt     time.Time
	sendCounter   uint64
	recvCounter   uint64
}

// SecureChannelConfig holds configuration for creating a secure channel.
type SecureChannelConfig struct {
	// TTL is how long the channel remains valid. Default: 1 hour.
	TTL time.Duration
}

// NewSecureChannel creates a new secure channel bound to the given attestation evidence.
// It generates an ECDH key pair and prepares for key exchange.
func NewSecureChannel(evidence AttestationEvidence) (*SecureChannel, error) {
	return NewSecureChannelWithConfig(evidence, SecureChannelConfig{})
}

// NewSecureChannelWithConfig creates a secure channel with custom configuration.
func NewSecureChannelWithConfig(evidence AttestationEvidence, config SecureChannelConfig) (*SecureChannel, error) {
	// Generate ECDH key pair (P-256)
	curve := ecdh.P256()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ECDH key: %w", err)
	}

	ttl := config.TTL
	if ttl == 0 {
		ttl = 1 * time.Hour
	}

	now := time.Now().UTC()
	return &SecureChannel{
		privateKey: privateKey,
		evidence:   evidence,
		createdAt:  now,
		expiresAt:  now.Add(ttl),
	}, nil
}

// PublicKey returns the channel's ECDH public key bytes for sending to the peer.
func (sc *SecureChannel) PublicKey() []byte {
	return sc.privateKey.PublicKey().Bytes()
}

// PublicKeyHex returns the hex-encoded public key.
func (sc *SecureChannel) PublicKeyHex() string {
	return hex.EncodeToString(sc.PublicKey())
}

// EstablishWithPeerKey completes the key exchange using the peer's public key.
// The shared secret is derived from: ECDH(privKey, peerPubKey) || measurement
// This binds the channel to the enclave's identity.
func (sc *SecureChannel) EstablishWithPeerKey(peerPubKeyBytes []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.established {
		return fmt.Errorf("channel already established")
	}

	// Parse peer public key
	curve := ecdh.P256()
	peerPubKey, err := curve.NewPublicKey(peerPubKeyBytes)
	if err != nil {
		return fmt.Errorf("parsing peer public key: %w", err)
	}

	// ECDH shared secret
	rawShared, err := sc.privateKey.ECDH(peerPubKey)
	if err != nil {
		return fmt.Errorf("ECDH key exchange: %w", err)
	}

	// Derive channel keys: SHA-256(ECDH_shared || measurement || "teamvault-tee-channel")
	// This binds the key to the enclave identity.
	h := sha256.New()
	h.Write(rawShared)
	h.Write([]byte(sc.evidence.Measurement))
	h.Write([]byte("teamvault-tee-channel"))
	sc.sharedSecret = h.Sum(nil)

	// Derive separate send and receive keys for bidirectional communication
	sendKeyMaterial := sha256.Sum256(append(sc.sharedSecret, []byte("send")...))
	recvKeyMaterial := sha256.Sum256(append(sc.sharedSecret, []byte("recv")...))

	// Create AES-GCM ciphers
	sc.sendCipher, err = createAEAD(sendKeyMaterial[:])
	if err != nil {
		return fmt.Errorf("creating send cipher: %w", err)
	}

	sc.recvCipher, err = createAEAD(recvKeyMaterial[:])
	if err != nil {
		return fmt.Errorf("creating recv cipher: %w", err)
	}

	sc.established = true

	// Zero raw ECDH output
	for i := range rawShared {
		rawShared[i] = 0
	}

	return nil
}

// Encrypt encrypts plaintext using the send cipher.
func (sc *SecureChannel) Encrypt(plaintext []byte) ([]byte, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.established {
		return nil, fmt.Errorf("channel not established")
	}
	if time.Now().UTC().After(sc.expiresAt) {
		return nil, fmt.Errorf("channel expired")
	}

	nonce := make([]byte, sc.sendCipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := sc.sendCipher.Seal(nil, nonce, plaintext, nil)
	sc.sendCounter++

	// Prepend nonce to ciphertext: [nonce || ciphertext]
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// Decrypt decrypts ciphertext using the receive cipher.
func (sc *SecureChannel) Decrypt(data []byte) ([]byte, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.established {
		return nil, fmt.Errorf("channel not established")
	}
	if time.Now().UTC().After(sc.expiresAt) {
		return nil, fmt.Errorf("channel expired")
	}

	nonceSize := sc.recvCipher.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	plaintext, err := sc.recvCipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	sc.recvCounter++
	return plaintext, nil
}

// IsEstablished returns whether the key exchange is complete.
func (sc *SecureChannel) IsEstablished() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.established
}

// IsExpired returns whether the channel has expired.
func (sc *SecureChannel) IsExpired() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return time.Now().UTC().After(sc.expiresAt)
}

// ExpiresAt returns the channel expiration time.
func (sc *SecureChannel) ExpiresAt() time.Time {
	return sc.expiresAt
}

// Evidence returns the attestation evidence bound to this channel.
func (sc *SecureChannel) Evidence() AttestationEvidence {
	return sc.evidence
}

// SessionKeyHash returns the SHA-256 of the shared secret for session tracking.
func (sc *SecureChannel) SessionKeyHash() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.sharedSecret == nil {
		return ""
	}
	h := sha256.Sum256(sc.sharedSecret)
	return hex.EncodeToString(h[:])
}

// Close zeros out all key material.
func (sc *SecureChannel) Close() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	for i := range sc.sharedSecret {
		sc.sharedSecret[i] = 0
	}
	sc.sharedSecret = nil
	sc.established = false
}

// createAEAD creates an AES-256-GCM AEAD cipher from a 32-byte key.
func createAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
