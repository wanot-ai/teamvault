package db

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateSecret creates a new secret entry (metadata only).
func (db *DB) CreateSecret(ctx context.Context, projectID, path, description, createdBy string) (*Secret, error) {
	return db.CreateSecretWithType(ctx, projectID, path, description, "kv", nil, createdBy)
}

// CreateSecretWithType creates a new secret entry with a specific type and metadata.
func (db *DB) CreateSecretWithType(ctx context.Context, projectID, path, description, secretType string, metadata json.RawMessage, createdBy string) (*Secret, error) {
	if secretType == "" {
		secretType = "kv"
	}
	secret := &Secret{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO secrets (project_id, path, description, secret_type, metadata, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, project_id, path, COALESCE(description, ''), secret_type, metadata, created_by, created_at, deleted_at`,
		projectID, path, description, secretType, metadata, createdBy,
	).Scan(&secret.ID, &secret.ProjectID, &secret.Path, &secret.Description,
		&secret.SecretType, &secret.Metadata, &secret.CreatedBy, &secret.CreatedAt, &secret.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("creating secret: %w", err)
	}
	return secret, nil
}

// UpdateSecretType updates the type and metadata of a secret.
func (db *DB) UpdateSecretType(ctx context.Context, secretID, secretType string, metadata json.RawMessage) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE secrets SET secret_type = $2, metadata = $3 WHERE id = $1`,
		secretID, secretType, metadata,
	)
	if err != nil {
		return fmt.Errorf("updating secret type: %w", err)
	}
	return nil
}

// GetSecret retrieves a secret by project ID and path.
func (db *DB) GetSecret(ctx context.Context, projectID, path string) (*Secret, error) {
	secret := &Secret{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, project_id, path, COALESCE(description, ''), COALESCE(secret_type, 'kv'), metadata, created_by, created_at, deleted_at
		 FROM secrets WHERE project_id = $1 AND path = $2 AND deleted_at IS NULL`,
		projectID, path,
	).Scan(&secret.ID, &secret.ProjectID, &secret.Path, &secret.Description,
		&secret.SecretType, &secret.Metadata, &secret.CreatedBy, &secret.CreatedAt, &secret.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}
	return secret, nil
}

// GetSecretByID retrieves a secret by its ID.
func (db *DB) GetSecretByID(ctx context.Context, id string) (*Secret, error) {
	secret := &Secret{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, project_id, path, COALESCE(description, ''), COALESCE(secret_type, 'kv'), metadata, created_by, created_at, deleted_at
		 FROM secrets WHERE id = $1`,
		id,
	).Scan(&secret.ID, &secret.ProjectID, &secret.Path, &secret.Description,
		&secret.SecretType, &secret.Metadata, &secret.CreatedBy, &secret.CreatedAt, &secret.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("getting secret by id: %w", err)
	}
	return secret, nil
}

// ListSecrets lists all active secrets in a project.
func (db *DB) ListSecrets(ctx context.Context, projectID string) ([]Secret, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, project_id, path, COALESCE(description, ''), COALESCE(secret_type, 'kv'), metadata, created_by, created_at, deleted_at
		 FROM secrets WHERE project_id = $1 AND deleted_at IS NULL
		 ORDER BY path`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Path, &s.Description,
			&s.SecretType, &s.Metadata, &s.CreatedBy, &s.CreatedAt, &s.DeletedAt); err != nil {
			return nil, fmt.Errorf("scanning secret: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

// SoftDeleteSecret marks a secret as deleted.
func (db *DB) SoftDeleteSecret(ctx context.Context, projectID, path string) error {
	result, err := db.Pool.Exec(ctx,
		`UPDATE secrets SET deleted_at = now()
		 WHERE project_id = $1 AND path = $2 AND deleted_at IS NULL`,
		projectID, path,
	)
	if err != nil {
		return fmt.Errorf("soft deleting secret: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("secret not found")
	}
	return nil
}

// CreateSecretVersion inserts a new encrypted version of a secret.
func (db *DB) CreateSecretVersion(ctx context.Context, secretID string, version int,
	ciphertext, nonce, encryptedDEK, dekNonce []byte, masterKeyVersion int, createdBy string) (*SecretVersion, error) {

	sv := &SecretVersion{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO secret_versions (secret_id, version, ciphertext, nonce, encrypted_dek, dek_nonce, master_key_version, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, secret_id, version, ciphertext, nonce, encrypted_dek, dek_nonce, master_key_version, created_by, created_at`,
		secretID, version, ciphertext, nonce, encryptedDEK, dekNonce, masterKeyVersion, createdBy,
	).Scan(&sv.ID, &sv.SecretID, &sv.Version, &sv.Ciphertext, &sv.Nonce,
		&sv.EncryptedDEK, &sv.DEKNonce, &sv.MasterKeyVersion, &sv.CreatedBy, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating secret version: %w", err)
	}
	return sv, nil
}

// GetLatestSecretVersion retrieves the latest version of a secret.
func (db *DB) GetLatestSecretVersion(ctx context.Context, secretID string) (*SecretVersion, error) {
	sv := &SecretVersion{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, secret_id, version, ciphertext, nonce, encrypted_dek, dek_nonce, master_key_version, created_by, created_at
		 FROM secret_versions WHERE secret_id = $1
		 ORDER BY version DESC LIMIT 1`,
		secretID,
	).Scan(&sv.ID, &sv.SecretID, &sv.Version, &sv.Ciphertext, &sv.Nonce,
		&sv.EncryptedDEK, &sv.DEKNonce, &sv.MasterKeyVersion, &sv.CreatedBy, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting latest secret version: %w", err)
	}
	return sv, nil
}

// GetNextSecretVersion returns the next version number for a secret.
func (db *DB) GetNextSecretVersion(ctx context.Context, secretID string) (int, error) {
	var maxVersion *int
	err := db.Pool.QueryRow(ctx,
		`SELECT MAX(version) FROM secret_versions WHERE secret_id = $1`,
		secretID,
	).Scan(&maxVersion)
	if err != nil {
		return 0, fmt.Errorf("getting max version: %w", err)
	}
	if maxVersion == nil {
		return 1, nil
	}
	return *maxVersion + 1, nil
}

// ListSecretVersions returns all versions of a secret (metadata only, no ciphertext).
func (db *DB) ListSecretVersions(ctx context.Context, secretID string) ([]SecretVersion, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, secret_id, version, created_by, created_at
		 FROM secret_versions WHERE secret_id = $1
		 ORDER BY version DESC`,
		secretID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing secret versions: %w", err)
	}
	defer rows.Close()

	var versions []SecretVersion
	for rows.Next() {
		var sv SecretVersion
		if err := rows.Scan(&sv.ID, &sv.SecretID, &sv.Version, &sv.CreatedBy, &sv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning secret version: %w", err)
		}
		versions = append(versions, sv)
	}
	return versions, rows.Err()
}
