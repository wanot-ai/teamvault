package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CreateAgent inserts a new agent for a team.
func (db *DB) CreateAgent(ctx context.Context, teamID, name, description, tokenHash string, scopes []string, metadata json.RawMessage, createdBy string, expiresAt *time.Time) (*Agent, error) {
	agent := &Agent{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO agents (team_id, name, description, token_hash, scopes, metadata, created_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, team_id, name, COALESCE(description, ''), token_hash, scopes, metadata, created_by, created_at, expires_at`,
		teamID, name, description, tokenHash, scopes, metadata, createdBy, expiresAt,
	).Scan(&agent.ID, &agent.TeamID, &agent.Name, &agent.Description, &agent.TokenHash,
		&agent.Scopes, &agent.Metadata, &agent.CreatedBy, &agent.CreatedAt, &agent.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating agent: %w", err)
	}
	return agent, nil
}

// GetAgentByID retrieves an agent by ID.
func (db *DB) GetAgentByID(ctx context.Context, id string) (*Agent, error) {
	agent := &Agent{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, team_id, name, COALESCE(description, ''), token_hash, scopes, metadata, created_by, created_at, expires_at
		 FROM agents WHERE id = $1`,
		id,
	).Scan(&agent.ID, &agent.TeamID, &agent.Name, &agent.Description, &agent.TokenHash,
		&agent.Scopes, &agent.Metadata, &agent.CreatedBy, &agent.CreatedAt, &agent.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("getting agent by id: %w", err)
	}
	return agent, nil
}

// ListAgentsByTeam returns all agents for a team.
func (db *DB) ListAgentsByTeam(ctx context.Context, teamID string) ([]Agent, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, team_id, name, COALESCE(description, ''), token_hash, scopes, metadata, created_by, created_at, expires_at
		 FROM agents WHERE team_id = $1 ORDER BY created_at DESC`,
		teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.TeamID, &a.Name, &a.Description, &a.TokenHash,
			&a.Scopes, &a.Metadata, &a.CreatedBy, &a.CreatedAt, &a.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// DeleteAgent deletes an agent by ID.
func (db *DB) DeleteAgent(ctx context.Context, id string) error {
	result, err := db.Pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("agent not found")
	}
	return nil
}

// FindAgentByToken finds an agent by iterating through all non-expired agents
// and comparing the token hash using the provided function.
func (db *DB) FindAgentByToken(ctx context.Context, checkFn func(hash string) bool) (*Agent, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, team_id, name, COALESCE(description, ''), token_hash, scopes, metadata, created_by, created_at, expires_at
		 FROM agents
		 WHERE (expires_at IS NULL OR expires_at > now())`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying agents: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.TeamID, &a.Name, &a.Description, &a.TokenHash,
			&a.Scopes, &a.Metadata, &a.CreatedBy, &a.CreatedAt, &a.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning agent: %w", err)
		}
		if checkFn(a.TokenHash) {
			return &a, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("agent not found")
}
