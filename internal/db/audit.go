package db

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateAuditEvent inserts an audit event.
func (db *DB) CreateAuditEvent(ctx context.Context, actorType, actorID, action, resource, outcome, ip string, metadata json.RawMessage, prevHash, hash string) (*AuditEvent, error) {
	event := &AuditEvent{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO audit_events (actor_type, actor_id, action, resource, outcome, ip, metadata, prev_hash, hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, timestamp, actor_type, actor_id, action, resource, outcome, ip, metadata, prev_hash, hash`,
		actorType, actorID, action, resource, outcome, ip, metadata, prevHash, hash,
	).Scan(&event.ID, &event.Timestamp, &event.ActorType, &event.ActorID, &event.Action,
		&event.Resource, &event.Outcome, &event.IP, &event.Metadata, &event.PrevHash, &event.Hash)
	if err != nil {
		return nil, fmt.Errorf("creating audit event: %w", err)
	}
	return event, nil
}

// AuditQuery defines filters for querying audit events.
type AuditQuery struct {
	ActorType string
	ActorID   string
	Action    string
	Resource  string
	Limit     int
	Offset    int
}

// ListAuditEvents queries audit events with optional filters.
func (db *DB) ListAuditEvents(ctx context.Context, q AuditQuery) ([]AuditEvent, error) {
	query := `SELECT id, timestamp, actor_type, actor_id, action, resource, outcome, ip, metadata, COALESCE(prev_hash, ''), COALESCE(hash, '')
		 FROM audit_events WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if q.ActorType != "" {
		query += fmt.Sprintf(" AND actor_type = $%d", argIdx)
		args = append(args, q.ActorType)
		argIdx++
	}
	if q.ActorID != "" {
		query += fmt.Sprintf(" AND actor_id = $%d", argIdx)
		args = append(args, q.ActorID)
		argIdx++
	}
	if q.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, q.Action)
		argIdx++
	}
	if q.Resource != "" {
		query += fmt.Sprintf(" AND resource LIKE $%d", argIdx)
		args = append(args, q.Resource+"%")
		argIdx++
	}

	query += " ORDER BY timestamp DESC"

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit)
	argIdx++

	if q.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, q.Offset)
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing audit events: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.ActorType, &e.ActorID, &e.Action,
			&e.Resource, &e.Outcome, &e.IP, &e.Metadata, &e.PrevHash, &e.Hash); err != nil {
			return nil, fmt.Errorf("scanning audit event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetLastAuditHash retrieves the hash of the most recent audit event for chain linking.
func (db *DB) GetLastAuditHash(ctx context.Context) (string, error) {
	var hash *string
	err := db.Pool.QueryRow(ctx,
		`SELECT hash FROM audit_events ORDER BY timestamp DESC LIMIT 1`,
	).Scan(&hash)
	if err != nil {
		// If no rows, return empty string (genesis event)
		return "", nil
	}
	if hash == nil {
		return "", nil
	}
	return *hash, nil
}
