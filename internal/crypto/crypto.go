// Package crypto implements envelope encryption for secret values.
// Each secret version gets a unique DEK (Data Encryption Key) encrypted by a master key.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// EncryptedData holds all the components of an envelope-encrypted value.
type EncryptedData struct {
	Ciphertext       []byte
	Nonce            []byte
	EncryptedDEK     []byte
	DEKNonce         []byte
	MasterKeyVersion int
}

// EnvelopeCrypto handles envelope encryption using AES-256-GCM.
type EnvelopeCrypto struct {
	masterKey        []byte
	masterKeyVersion int
}

// NewEnvelopeCrypto creates a new EnvelopeCrypto instance.
// The master key can be provided via:
//   - MASTER_KEY env var (hex-encoded 32 bytes)
//   - MASTER_KEY_FILE env var (path to file containing hex-encoded key)
//   - Direct byte slice
func NewEnvelopeCrypto(masterKey []byte) (*EnvelopeCrypto, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be exactly 32 bytes, got %d", len(masterKey))
	}
	return &EnvelopeCrypto{
		masterKey:        masterKey,
		masterKeyVersion: 1,
	}, nil
}

// NewEnvelopeCryptoFromEnv loads the master key from environment variables.
func NewEnvelopeCryptoFromEnv() (*EnvelopeCrypto, error) {
	keyHex := os.Getenv("MASTER_KEY")
	if keyHex == "" {
		keyFile := os.Getenv("MASTER_KEY_FILE")
		if keyFile == "" {
			return nil, errors.New("MASTER_KEY or MASTER_KEY_FILE environment variable required")
		}
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("reading master key file: %w", err)
		}
		keyHex = strings.TrimSpace(string(data))
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding master key hex: %w", err)
	}

	return NewEnvelopeCrypto(key)
}

// Encrypt performs envelope encryption on plaintext.
// 1. Generate a random DEK
// 2. Encrypt plaintext with DEK using AES-256-GCM
// 3. Encrypt DEK with master key using AES-256-GCM
func (ec *EnvelopeCrypto) Encrypt(plaintext []byte) (*EncryptedData, error) {
	// Generate random DEK (32 bytes for AES-256)
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("generating DEK: %w", err)
	}

	// Encrypt plaintext with DEK
	ciphertext, nonce, err := aesGCMEncrypt(dek, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypting plaintext with DEK: %w", err)
	}

	// Encrypt DEK with master key
	encryptedDEK, dekNonce, err := aesGCMEncrypt(ec.masterKey, dek)
	if err != nil {
		return nil, fmt.Errorf("encrypting DEK with master key: %w", err)
	}

	// Zero out plaintext DEK from memory
	for i := range dek {
		dek[i] = 0
	}

	return &EncryptedData{
		Ciphertext:       ciphertext,
		Nonce:            nonce,
		EncryptedDEK:     encryptedDEK,
		DEKNonce:         dekNonce,
		MasterKeyVersion: ec.masterKeyVersion,
	}, nil
}

// Decrypt performs envelope decryption.
// 1. Decrypt DEK with master key
// 2. Decrypt ciphertext with DEK
func (ec *EnvelopeCrypto) Decrypt(data *EncryptedData) ([]byte, error) {
	// Decrypt DEK with master key
	dek, err := aesGCMDecrypt(ec.masterKey, data.EncryptedDEK, data.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting DEK: %w", err)
	}

	// Decrypt ciphertext with DEK
	plaintext, err := aesGCMDecrypt(dek, data.Ciphertext, data.Nonce)
	if err != nil {
		// Zero out DEK before returning error
		for i := range dek {
			dek[i] = 0
		}
		return nil, fmt.Errorf("decrypting ciphertext: %w", err)
	}

	// Zero out DEK from memory
	for i := range dek {
		dek[i] = 0
	}

	return plaintext, nil
}

// aesGCMEncrypt encrypts data using AES-256-GCM with a random nonce.
func aesGCMEncrypt(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// aesGCMDecrypt decrypts data using AES-256-GCM.
func aesGCMDecrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}
