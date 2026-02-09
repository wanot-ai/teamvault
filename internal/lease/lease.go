// Package lease manages dynamic secrets with TTL-based leases.
package lease

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/teamvault/teamvault/internal/crypto"
	"github.com/teamvault/teamvault/internal/db"
)

// Manager handles dynamic secret leases.
type Manager struct {
	database  *db.DB
	cryptoSvc *crypto.EnvelopeCrypto
	stopCh    chan struct{}
}

// NewManager creates a new lease manager.
func NewManager(database *db.DB, cryptoSvc *crypto.EnvelopeCrypto) *Manager {
	return &Manager{
		database:  database,
		cryptoSvc: cryptoSvc,
		stopCh:    make(chan struct{}),
	}
}

// DatabaseCredentials represents mock temporary database credentials.
type DatabaseCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
}

// LeaseResponse is returned when a lease is issued.
type LeaseResponse struct {
	LeaseID   string          `json:"lease_id"`
	ExpiresAt time.Time       `json:"expires_at"`
	TTL       int             `json:"ttl_seconds"`
	Data      json.RawMessage `json:"data"`
}

// LeaseInfo provides metadata about a lease (without secret data).
type LeaseInfo struct {
	ID         string     `json:"id"`
	SecretPath string     `json:"secret_path"`
	LeaseType  string     `json:"lease_type"`
	IssuedTo   string     `json:"issued_to"`
	IssuedAt   time.Time  `json:"issued_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// IssueDatabaseLease creates a mock temporary database credential with a TTL.
func (m *Manager) IssueDatabaseLease(ctx context.Context, issuedTo string, ttlSeconds int, orgID *string) (*LeaseResponse, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 3600 // Default 1 hour
	}
	if ttlSeconds > 86400 {
		ttlSeconds = 86400 // Max 24 hours
	}

	// Generate mock credentials
	username := "tv_" + randomHex(8)
	password := randomHex(32)

	creds := DatabaseCredentials{
		Username: username,
		Password: password,
		Host:     "localhost",
		Port:     5432,
		Database: "app_db",
	}

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("marshaling credentials: %w", err)
	}

	// Encrypt the credentials
	encrypted, err := m.cryptoSvc.Encrypt(credsJSON)
	if err != nil {
		return nil, fmt.Errorf("encrypting lease value: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	lease, err := m.database.CreateLease(ctx, orgID, "dynamic/database",
		"database", encrypted.Ciphertext, encrypted.EncryptedDEK,
		encrypted.Nonce, encrypted.DEKNonce, issuedTo, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating lease: %w", err)
	}

	return &LeaseResponse{
		LeaseID:   lease.ID,
		ExpiresAt: lease.ExpiresAt,
		TTL:       ttlSeconds,
		Data:      credsJSON,
	}, nil
}

// RevokeLease revokes a lease early.
func (m *Manager) RevokeLease(ctx context.Context, leaseID string) error {
	return m.database.RevokeLease(ctx, leaseID)
}

// ListActiveLeases returns metadata for all active leases.
func (m *Manager) ListActiveLeases(ctx context.Context) ([]LeaseInfo, error) {
	leases, err := m.database.ListActiveLeases(ctx)
	if err != nil {
		return nil, err
	}

	infos := make([]LeaseInfo, 0, len(leases))
	for _, l := range leases {
		infos = append(infos, LeaseInfo{
			ID:         l.ID,
			SecretPath: l.SecretPath,
			LeaseType:  l.LeaseType,
			IssuedTo:   l.IssuedTo,
			IssuedAt:   l.IssuedAt,
			ExpiresAt:  l.ExpiresAt,
			RevokedAt:  l.RevokedAt,
		})
	}
	return infos, nil
}

// StartCleanup begins the lease cleanup goroutine that marks expired leases every 30 seconds.
func (m *Manager) StartCleanup(ctx context.Context) {
	log.Println("Lease cleanup goroutine started (interval: 30s)")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Lease cleanup stopped (context cancelled)")
			return
		case <-m.stopCh:
			log.Println("Lease cleanup stopped")
			return
		case <-ticker.C:
			count, err := m.database.ExpireLeases(ctx)
			if err != nil {
				log.Printf("Lease cleanup error: %v", err)
			} else if count > 0 {
				log.Printf("Lease cleanup: expired %d leases", count)
			}
		}
	}
}

// Stop signals the cleanup goroutine to stop.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// randomHex generates a random hex string of the given byte length.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based if crypto/rand fails (should never happen)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
