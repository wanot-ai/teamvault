package db

import (
	"context"
	"fmt"
)

// CreateUser inserts a new user.
func (db *DB) CreateUser(ctx context.Context, email, passwordHash, name, role string) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, password_hash, name, role, created_at`,
		email, passwordHash, name, role,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email address.
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, created_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return user, nil
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return user, nil
}

// GetUserByOIDC retrieves a user by OIDC issuer and subject.
func (db *DB) GetUserByOIDC(ctx context.Context, issuer, subject string) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, created_at
		 FROM users WHERE oidc_issuer = $1 AND oidc_subject = $2`,
		issuer, subject,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting user by OIDC: %w", err)
	}
	return user, nil
}

// CreateOIDCUser creates a user via OIDC login.
func (db *DB) CreateOIDCUser(ctx context.Context, email, name, role, oidcIssuer, oidcSubject string) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name, role, oidc_issuer, oidc_subject)
		 VALUES ($1, '', $2, $3, $4, $5)
		 RETURNING id, email, password_hash, name, role, created_at`,
		email, name, role, oidcIssuer, oidcSubject,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating OIDC user: %w", err)
	}
	return user, nil
}

// Ping checks the database connection.
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
