package db

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateIAMPolicy inserts a new IAM policy.
func (db *DB) CreateIAMPolicy(ctx context.Context, orgID, name, description, policyType string, policyDoc json.RawMessage, hclSource, createdBy string) (*IAMPolicy, error) {
	policy := &IAMPolicy{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO iam_policies (org_id, name, description, policy_type, policy_doc, hcl_source, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, org_id, name, COALESCE(description, ''), policy_type, policy_doc, hcl_source, created_by, created_at, updated_at`,
		orgID, name, description, policyType, policyDoc, hclSource, createdBy,
	).Scan(&policy.ID, &policy.OrgID, &policy.Name, &policy.Description, &policy.PolicyType,
		&policy.PolicyDoc, &policy.HCLSource, &policy.CreatedBy, &policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating IAM policy: %w", err)
	}
	return policy, nil
}

// GetIAMPolicyByID retrieves an IAM policy by ID.
func (db *DB) GetIAMPolicyByID(ctx context.Context, id string) (*IAMPolicy, error) {
	policy := &IAMPolicy{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, org_id, name, COALESCE(description, ''), policy_type, policy_doc, COALESCE(hcl_source, ''), created_by, created_at, updated_at
		 FROM iam_policies WHERE id = $1`,
		id,
	).Scan(&policy.ID, &policy.OrgID, &policy.Name, &policy.Description, &policy.PolicyType,
		&policy.PolicyDoc, &policy.HCLSource, &policy.CreatedBy, &policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting IAM policy by id: %w", err)
	}
	return policy, nil
}

// ListIAMPolicies returns all IAM policies, optionally filtered by org.
func (db *DB) ListIAMPolicies(ctx context.Context, orgID string) ([]IAMPolicy, error) {
	var query string
	var args []interface{}

	if orgID != "" {
		query = `SELECT id, org_id, name, COALESCE(description, ''), policy_type, policy_doc, COALESCE(hcl_source, ''), created_by, created_at, updated_at
				 FROM iam_policies WHERE org_id = $1 ORDER BY created_at DESC`
		args = []interface{}{orgID}
	} else {
		query = `SELECT id, org_id, name, COALESCE(description, ''), policy_type, policy_doc, COALESCE(hcl_source, ''), created_by, created_at, updated_at
				 FROM iam_policies ORDER BY created_at DESC`
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing IAM policies: %w", err)
	}
	defer rows.Close()

	var policies []IAMPolicy
	for rows.Next() {
		var p IAMPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.PolicyType,
			&p.PolicyDoc, &p.HCLSource, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning IAM policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// ListIAMPoliciesByType returns IAM policies filtered by type within an org.
func (db *DB) ListIAMPoliciesByType(ctx context.Context, orgID, policyType string) ([]IAMPolicy, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description, ''), policy_type, policy_doc, COALESCE(hcl_source, ''), created_by, created_at, updated_at
		 FROM iam_policies WHERE org_id = $1 AND policy_type = $2 ORDER BY created_at DESC`,
		orgID, policyType,
	)
	if err != nil {
		return nil, fmt.Errorf("listing IAM policies by type: %w", err)
	}
	defer rows.Close()

	var policies []IAMPolicy
	for rows.Next() {
		var p IAMPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.PolicyType,
			&p.PolicyDoc, &p.HCLSource, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning IAM policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// UpdateIAMPolicy updates an existing IAM policy.
func (db *DB) UpdateIAMPolicy(ctx context.Context, id, name, description, policyType string, policyDoc json.RawMessage, hclSource string) (*IAMPolicy, error) {
	policy := &IAMPolicy{}
	err := db.Pool.QueryRow(ctx,
		`UPDATE iam_policies
		 SET name = $2, description = $3, policy_type = $4, policy_doc = $5, hcl_source = $6, updated_at = now()
		 WHERE id = $1
		 RETURNING id, org_id, name, COALESCE(description, ''), policy_type, policy_doc, COALESCE(hcl_source, ''), created_by, created_at, updated_at`,
		id, name, description, policyType, policyDoc, hclSource,
	).Scan(&policy.ID, &policy.OrgID, &policy.Name, &policy.Description, &policy.PolicyType,
		&policy.PolicyDoc, &policy.HCLSource, &policy.CreatedBy, &policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating IAM policy: %w", err)
	}
	return policy, nil
}

// DeleteIAMPolicy deletes an IAM policy by ID.
func (db *DB) DeleteIAMPolicy(ctx context.Context, id string) error {
	result, err := db.Pool.Exec(ctx, `DELETE FROM iam_policies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting IAM policy: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("IAM policy not found")
	}
	return nil
}
