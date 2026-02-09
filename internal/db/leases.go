package db

import (
	"context"
	"fmt"
	"time"
)

// CreateLease inserts a new lease.
func (db *DB) CreateLease(ctx context.Context, orgID *string, secretPath, leaseType string,
	valueCiphertext, encryptedDEK, nonce, dekNonce []byte,
	issuedTo string, expiresAt time.Time) (*Lease, error) {

	lease := &Lease{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO leases (org_id, secret_path, lease_type, value_ciphertext, encrypted_dek, nonce, dek_nonce, issued_to, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, org_id, secret_path, lease_type, value_ciphertext, encrypted_dek, nonce, dek_nonce, issued_to, issued_at, expires_at, revoked_at`,
		orgID, secretPath, leaseType, valueCiphertext, encryptedDEK, nonce, dekNonce, issuedTo, expiresAt,
	).Scan(&lease.ID, &lease.OrgID, &lease.SecretPath, &lease.LeaseType,
		&lease.ValueCiphertext, &lease.EncryptedDEK, &lease.Nonce, &lease.DEKNonce,
		&lease.IssuedTo, &lease.IssuedAt, &lease.ExpiresAt, &lease.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("creating lease: %w", err)
	}
	return lease, nil
}

// GetLeaseByID retrieves a lease by ID.
func (db *DB) GetLeaseByID(ctx context.Context, id string) (*Lease, error) {
	lease := &Lease{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, org_id, secret_path, lease_type, value_ciphertext, encrypted_dek, nonce, dek_nonce, issued_to, issued_at, expires_at, revoked_at
		 FROM leases WHERE id = $1`,
		id,
	).Scan(&lease.ID, &lease.OrgID, &lease.SecretPath, &lease.LeaseType,
		&lease.ValueCiphertext, &lease.EncryptedDEK, &lease.Nonce, &lease.DEKNonce,
		&lease.IssuedTo, &lease.IssuedAt, &lease.ExpiresAt, &lease.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("getting lease by id: %w", err)
	}
	return lease, nil
}

// ListActiveLeases returns all non-revoked, non-expired leases.
func (db *DB) ListActiveLeases(ctx context.Context) ([]Lease, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, org_id, secret_path, lease_type, value_ciphertext, encrypted_dek, nonce, dek_nonce, issued_to, issued_at, expires_at, revoked_at
		 FROM leases
		 WHERE revoked_at IS NULL AND expires_at > now()
		 ORDER BY issued_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing active leases: %w", err)
	}
	defer rows.Close()

	var leases []Lease
	for rows.Next() {
		var l Lease
		if err := rows.Scan(&l.ID, &l.OrgID, &l.SecretPath, &l.LeaseType,
			&l.ValueCiphertext, &l.EncryptedDEK, &l.Nonce, &l.DEKNonce,
			&l.IssuedTo, &l.IssuedAt, &l.ExpiresAt, &l.RevokedAt); err != nil {
			return nil, fmt.Errorf("scanning lease: %w", err)
		}
		leases = append(leases, l)
	}
	return leases, rows.Err()
}

// RevokeLease marks a lease as revoked.
func (db *DB) RevokeLease(ctx context.Context, id string) error {
	result, err := db.Pool.Exec(ctx,
		`UPDATE leases SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("revoking lease: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("lease not found or already revoked")
	}
	return nil
}

// ExpireLeases marks all expired but non-revoked leases as revoked.
// Returns the number of leases expired.
func (db *DB) ExpireLeases(ctx context.Context) (int64, error) {
	result, err := db.Pool.Exec(ctx,
		`UPDATE leases SET revoked_at = now()
		 WHERE revoked_at IS NULL AND expires_at <= now()`,
	)
	if err != nil {
		return 0, fmt.Errorf("expiring leases: %w", err)
	}
	return result.RowsAffected(), nil
}
