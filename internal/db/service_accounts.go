package db

import (
	"context"
	"fmt"
	"time"
)

// CreateServiceAccount inserts a new service account.
func (db *DB) CreateServiceAccount(ctx context.Context, name, tokenHash, projectID string, scopes []string, createdBy string, expiresAt *time.Time) (*ServiceAccount, error) {
	sa := &ServiceAccount{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO service_accounts (name, token_hash, project_id, scopes, created_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, token_hash, project_id, scopes, created_by, created_at, expires_at`,
		name, tokenHash, projectID, scopes, createdBy, expiresAt,
	).Scan(&sa.ID, &sa.Name, &sa.TokenHash, &sa.ProjectID, &sa.Scopes, &sa.CreatedBy, &sa.CreatedAt, &sa.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating service account: %w", err)
	}
	return sa, nil
}

// ListServiceAccounts returns all service accounts.
func (db *DB) ListServiceAccounts(ctx context.Context) ([]ServiceAccount, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, token_hash, project_id, scopes, created_by, created_at, expires_at
		 FROM service_accounts ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing service accounts: %w", err)
	}
	defer rows.Close()

	var accounts []ServiceAccount
	for rows.Next() {
		var sa ServiceAccount
		if err := rows.Scan(&sa.ID, &sa.Name, &sa.TokenHash, &sa.ProjectID, &sa.Scopes,
			&sa.CreatedBy, &sa.CreatedAt, &sa.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning service account: %w", err)
		}
		accounts = append(accounts, sa)
	}
	return accounts, rows.Err()
}

// GetServiceAccountByID retrieves a service account by ID.
func (db *DB) GetServiceAccountByID(ctx context.Context, id string) (*ServiceAccount, error) {
	sa := &ServiceAccount{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, token_hash, project_id, scopes, created_by, created_at, expires_at
		 FROM service_accounts WHERE id = $1`,
		id,
	).Scan(&sa.ID, &sa.Name, &sa.TokenHash, &sa.ProjectID, &sa.Scopes,
		&sa.CreatedBy, &sa.CreatedAt, &sa.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("getting service account: %w", err)
	}
	return sa, nil
}

// ListServiceAccountsByProject returns all service accounts for a project.
func (db *DB) ListServiceAccountsByProject(ctx context.Context, projectID string) ([]ServiceAccount, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, token_hash, project_id, scopes, created_by, created_at, expires_at
		 FROM service_accounts WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing service accounts by project: %w", err)
	}
	defer rows.Close()

	var accounts []ServiceAccount
	for rows.Next() {
		var sa ServiceAccount
		if err := rows.Scan(&sa.ID, &sa.Name, &sa.TokenHash, &sa.ProjectID, &sa.Scopes,
			&sa.CreatedBy, &sa.CreatedAt, &sa.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning service account: %w", err)
		}
		accounts = append(accounts, sa)
	}
	return accounts, rows.Err()
}

// FindServiceAccountByToken finds a service account by iterating through all accounts
// and comparing the token hash. This is O(n) but necessary since we can't look up by token directly.
func (db *DB) FindServiceAccountByToken(ctx context.Context, checkFn func(hash string) bool) (*ServiceAccount, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, token_hash, project_id, scopes, created_by, created_at, expires_at
		 FROM service_accounts
		 WHERE (expires_at IS NULL OR expires_at > now())`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying service accounts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sa ServiceAccount
		if err := rows.Scan(&sa.ID, &sa.Name, &sa.TokenHash, &sa.ProjectID, &sa.Scopes,
			&sa.CreatedBy, &sa.CreatedAt, &sa.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning service account: %w", err)
		}
		if checkFn(sa.TokenHash) {
			return &sa, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("service account not found")
}
