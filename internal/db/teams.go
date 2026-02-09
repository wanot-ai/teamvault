package db

import (
	"context"
	"fmt"
)

// CreateTeam inserts a new team within an organization.
func (db *DB) CreateTeam(ctx context.Context, orgID, name, description string) (*Team, error) {
	team := &Team{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO teams (org_id, name, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, org_id, name, COALESCE(description, ''), created_at`,
		orgID, name, description,
	).Scan(&team.ID, &team.OrgID, &team.Name, &team.Description, &team.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating team: %w", err)
	}
	return team, nil
}

// GetTeamByID retrieves a team by ID.
func (db *DB) GetTeamByID(ctx context.Context, id string) (*Team, error) {
	team := &Team{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, org_id, name, COALESCE(description, ''), created_at
		 FROM teams WHERE id = $1`,
		id,
	).Scan(&team.ID, &team.OrgID, &team.Name, &team.Description, &team.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting team by id: %w", err)
	}
	return team, nil
}

// ListTeamsByOrg returns all teams for an organization.
func (db *DB) ListTeamsByOrg(ctx context.Context, orgID string) ([]Team, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description, ''), created_at
		 FROM teams WHERE org_id = $1 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing teams: %w", err)
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

// DeleteTeam deletes a team by ID.
func (db *DB) DeleteTeam(ctx context.Context, id string) error {
	result, err := db.Pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting team: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("team not found")
	}
	return nil
}

// AddTeamMember adds a user to a team with a given role.
func (db *DB) AddTeamMember(ctx context.Context, teamID, userID, role string) (*TeamMember, error) {
	member := &TeamMember{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO team_members (team_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (team_id, user_id) DO UPDATE SET role = $3
		 RETURNING team_id, user_id, role, joined_at`,
		teamID, userID, role,
	).Scan(&member.TeamID, &member.UserID, &member.Role, &member.JoinedAt)
	if err != nil {
		return nil, fmt.Errorf("adding team member: %w", err)
	}
	return member, nil
}

// RemoveTeamMember removes a user from a team.
func (db *DB) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	result, err := db.Pool.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	)
	if err != nil {
		return fmt.Errorf("removing team member: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("team member not found")
	}
	return nil
}

// ListTeamMembers returns all members of a team.
func (db *DB) ListTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT team_id, user_id, role, joined_at
		 FROM team_members WHERE team_id = $1 ORDER BY joined_at`,
		teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing team members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scanning team member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// GetTeamMember retrieves a specific team membership.
func (db *DB) GetTeamMember(ctx context.Context, teamID, userID string) (*TeamMember, error) {
	member := &TeamMember{}
	err := db.Pool.QueryRow(ctx,
		`SELECT team_id, user_id, role, joined_at
		 FROM team_members WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&member.TeamID, &member.UserID, &member.Role, &member.JoinedAt)
	if err != nil {
		return nil, fmt.Errorf("getting team member: %w", err)
	}
	return member, nil
}

// GetUserTeams returns all teams a user belongs to.
func (db *DB) GetUserTeams(ctx context.Context, userID string) ([]TeamMember, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT team_id, user_id, role, joined_at
		 FROM team_members WHERE user_id = $1 ORDER BY joined_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting user teams: %w", err)
	}
	defer rows.Close()

	var memberships []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scanning team member: %w", err)
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}
