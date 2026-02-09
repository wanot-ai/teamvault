package db

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreatePolicy inserts a new policy.
func (db *DB) CreatePolicy(ctx context.Context, name, effect string, actions []string, resourcePattern, subjectType string, subjectID *string, conditions json.RawMessage) (*Policy, error) {
	policy := &Policy{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO policies (name, effect, actions, resource_pattern, subject_type, subject_id, conditions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, name, effect, actions, resource_pattern, subject_type, subject_id, conditions, created_at`,
		name, effect, actions, resourcePattern, subjectType, subjectID, conditions,
	).Scan(&policy.ID, &policy.Name, &policy.Effect, &policy.Actions, &policy.ResourcePattern,
		&policy.SubjectType, &policy.SubjectID, &policy.Conditions, &policy.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating policy: %w", err)
	}
	return policy, nil
}

// ListPolicies returns all policies.
func (db *DB) ListPolicies(ctx context.Context) ([]Policy, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, effect, actions, resource_pattern, subject_type, subject_id, conditions, created_at
		 FROM policies ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var p Policy
		if err := rows.Scan(&p.ID, &p.Name, &p.Effect, &p.Actions, &p.ResourcePattern,
			&p.SubjectType, &p.SubjectID, &p.Conditions, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// GetPoliciesForSubject retrieves all policies matching a subject.
func (db *DB) GetPoliciesForSubject(ctx context.Context, subjectType, subjectID string) ([]Policy, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, name, effect, actions, resource_pattern, subject_type, subject_id, conditions, created_at
		 FROM policies
		 WHERE subject_type = $1 AND (subject_id = $2 OR subject_id IS NULL)
		 ORDER BY created_at`,
		subjectType, subjectID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting policies for subject: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var p Policy
		if err := rows.Scan(&p.ID, &p.Name, &p.Effect, &p.Actions, &p.ResourcePattern,
			&p.SubjectType, &p.SubjectID, &p.Conditions, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}
