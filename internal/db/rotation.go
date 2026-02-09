package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CreateRotationSchedule creates a new rotation schedule for a secret.
func (db *DB) CreateRotationSchedule(ctx context.Context, secretID, cronExpr, connectorType string, connectorConfig json.RawMessage, nextRotation time.Time) (*RotationSchedule, error) {
	rs := &RotationSchedule{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO rotation_schedules (secret_id, cron_expression, connector_type, connector_config, next_rotation_at, status)
		 VALUES ($1, $2, $3, $4, $5, 'active')
		 ON CONFLICT (secret_id) DO UPDATE SET
		   cron_expression = $2, connector_type = $3, connector_config = $4,
		   next_rotation_at = $5, status = 'active', updated_at = now()
		 RETURNING id, secret_id, cron_expression, connector_type, connector_config, last_rotated_at, next_rotation_at, status, created_at, updated_at`,
		secretID, cronExpr, connectorType, connectorConfig, nextRotation,
	).Scan(&rs.ID, &rs.SecretID, &rs.CronExpression, &rs.ConnectorType, &rs.ConnectorConfig,
		&rs.LastRotatedAt, &rs.NextRotationAt, &rs.Status, &rs.CreatedAt, &rs.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating rotation schedule: %w", err)
	}
	return rs, nil
}

// GetRotationSchedule retrieves a rotation schedule by secret ID.
func (db *DB) GetRotationSchedule(ctx context.Context, secretID string) (*RotationSchedule, error) {
	rs := &RotationSchedule{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, secret_id, cron_expression, connector_type, connector_config, last_rotated_at, next_rotation_at, status, created_at, updated_at
		 FROM rotation_schedules WHERE secret_id = $1`,
		secretID,
	).Scan(&rs.ID, &rs.SecretID, &rs.CronExpression, &rs.ConnectorType, &rs.ConnectorConfig,
		&rs.LastRotatedAt, &rs.NextRotationAt, &rs.Status, &rs.CreatedAt, &rs.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting rotation schedule: %w", err)
	}
	return rs, nil
}

// ListDueRotations returns all active rotation schedules that are past their next_rotation_at.
func (db *DB) ListDueRotations(ctx context.Context) ([]RotationSchedule, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, secret_id, cron_expression, connector_type, connector_config, last_rotated_at, next_rotation_at, status, created_at, updated_at
		 FROM rotation_schedules
		 WHERE status = 'active' AND next_rotation_at <= now()
		 ORDER BY next_rotation_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing due rotations: %w", err)
	}
	defer rows.Close()

	var schedules []RotationSchedule
	for rows.Next() {
		var rs RotationSchedule
		if err := rows.Scan(&rs.ID, &rs.SecretID, &rs.CronExpression, &rs.ConnectorType, &rs.ConnectorConfig,
			&rs.LastRotatedAt, &rs.NextRotationAt, &rs.Status, &rs.CreatedAt, &rs.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning rotation schedule: %w", err)
		}
		schedules = append(schedules, rs)
	}
	return schedules, rows.Err()
}

// MarkRotationCompleted updates a rotation schedule after a successful rotation.
func (db *DB) MarkRotationCompleted(ctx context.Context, scheduleID string, nextRotation time.Time) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE rotation_schedules SET last_rotated_at = now(), next_rotation_at = $2, updated_at = now() WHERE id = $1`,
		scheduleID, nextRotation,
	)
	if err != nil {
		return fmt.Errorf("marking rotation completed: %w", err)
	}
	return nil
}

// MarkRotationFailed sets the rotation status to 'failed'.
func (db *DB) MarkRotationFailed(ctx context.Context, scheduleID string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE rotation_schedules SET status = 'failed', updated_at = now() WHERE id = $1`,
		scheduleID,
	)
	if err != nil {
		return fmt.Errorf("marking rotation failed: %w", err)
	}
	return nil
}

// DeleteRotationSchedule removes a rotation schedule.
func (db *DB) DeleteRotationSchedule(ctx context.Context, secretID string) error {
	result, err := db.Pool.Exec(ctx,
		`DELETE FROM rotation_schedules WHERE secret_id = $1`,
		secretID,
	)
	if err != nil {
		return fmt.Errorf("deleting rotation schedule: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("rotation schedule not found")
	}
	return nil
}
