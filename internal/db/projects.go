package db

import (
	"context"
	"fmt"
)

// CreateProject inserts a new project.
func (db *DB) CreateProject(ctx context.Context, name, description, createdBy string) (*Project, error) {
	project := &Project{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO projects (name, description, created_by)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, description, created_by, created_at`,
		name, description, createdBy,
	).Scan(&project.ID, &project.Name, &project.Description, &project.CreatedBy, &project.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}
	return project, nil
}

// ListProjects returns all projects.
func (db *DB) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at
		 FROM projects ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProjectByName retrieves a project by name.
func (db *DB) GetProjectByName(ctx context.Context, name string) (*Project, error) {
	project := &Project{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at
		 FROM projects WHERE name = $1`,
		name,
	).Scan(&project.ID, &project.Name, &project.Description, &project.CreatedBy, &project.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting project by name: %w", err)
	}
	return project, nil
}

// GetProjectByID retrieves a project by ID.
func (db *DB) GetProjectByID(ctx context.Context, id string) (*Project, error) {
	project := &Project{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at
		 FROM projects WHERE id = $1`,
		id,
	).Scan(&project.ID, &project.Name, &project.Description, &project.CreatedBy, &project.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting project by id: %w", err)
	}
	return project, nil
}
