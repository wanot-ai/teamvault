package db

import (
	"context"
	"fmt"
)

// CreateOrg inserts a new organization.
func (db *DB) CreateOrg(ctx context.Context, name, description, createdBy string) (*Org, error) {
	org := &Org{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO orgs (name, description, created_by)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, COALESCE(description, ''), created_by, created_at`,
		name, description, createdBy,
	).Scan(&org.ID, &org.Name, &org.Description, &org.CreatedBy, &org.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating org: %w", err)
	}
	return org, nil
}

// GetOrgByID retrieves an organization by ID.
func (db *DB) GetOrgByID(ctx context.Context, id string) (*Org, error) {
	org := &Org{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at
		 FROM orgs WHERE id = $1`,
		id,
	).Scan(&org.ID, &org.Name, &org.Description, &org.CreatedBy, &org.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting org by id: %w", err)
	}
	return org, nil
}

// GetOrgByName retrieves an organization by name.
func (db *DB) GetOrgByName(ctx context.Context, name string) (*Org, error) {
	org := &Org{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at
		 FROM orgs WHERE name = $1`,
		name,
	).Scan(&org.ID, &org.Name, &org.Description, &org.CreatedBy, &org.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting org by name: %w", err)
	}
	return org, nil
}

// ListOrgs returns all organizations.
func (db *DB) ListOrgs(ctx context.Context) ([]Org, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at
		 FROM orgs ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing orgs: %w", err)
	}
	defer rows.Close()

	var orgs []Org
	for rows.Next() {
		var o Org
		if err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.CreatedBy, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning org: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// DeleteOrg deletes an organization by ID.
func (db *DB) DeleteOrg(ctx context.Context, id string) error {
	result, err := db.Pool.Exec(ctx, `DELETE FROM orgs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting org: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("org not found")
	}
	return nil
}
