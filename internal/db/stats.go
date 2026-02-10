package db

import (
	"context"
	"fmt"
)

// DashboardStats holds aggregate counts for the dashboard.
type DashboardStats struct {
	TotalSecrets    int64 `json:"total_secrets"`
	TotalProjects   int64 `json:"total_projects"`
	ActiveLeases    int64 `json:"active_leases"`
	RecentRotations int64 `json:"recent_rotations"`
	TotalOrgs       int64 `json:"total_orgs"`
	TotalTeams      int64 `json:"total_teams"`
}

// GetDashboardStats queries aggregate counts from the database.
func (db *DB) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Total secrets (non-deleted)
	err := db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM secrets WHERE deleted_at IS NULL`,
	).Scan(&stats.TotalSecrets)
	if err != nil {
		return nil, fmt.Errorf("counting secrets: %w", err)
	}

	// Total projects
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM projects`,
	).Scan(&stats.TotalProjects)
	if err != nil {
		return nil, fmt.Errorf("counting projects: %w", err)
	}

	// Active leases (non-revoked, non-expired)
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leases WHERE revoked_at IS NULL AND expires_at > now()`,
	).Scan(&stats.ActiveLeases)
	if err != nil {
		return nil, fmt.Errorf("counting active leases: %w", err)
	}

	// Recent rotations (completed in last 24h)
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM rotation_schedules WHERE last_rotated_at IS NOT NULL AND last_rotated_at > now() - interval '24 hours'`,
	).Scan(&stats.RecentRotations)
	if err != nil {
		return nil, fmt.Errorf("counting recent rotations: %w", err)
	}

	// Total orgs
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orgs`,
	).Scan(&stats.TotalOrgs)
	if err != nil {
		return nil, fmt.Errorf("counting orgs: %w", err)
	}

	// Total teams
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM teams`,
	).Scan(&stats.TotalTeams)
	if err != nil {
		return nil, fmt.Errorf("counting teams: %w", err)
	}

	return stats, nil
}

// ListAllTeams returns all teams across all organizations.
func (db *DB) ListAllTeams(ctx context.Context) ([]Team, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description, ''), created_at
		 FROM teams ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing all teams: %w", err)
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.Description, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning team: %w", err)
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}
