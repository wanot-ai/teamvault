// Package replication implements multi-region replication for TeamVault using
// a WAL-based approach with leader/follower roles and vector clock conflict resolution.
package replication

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeRole represents the replication role of this instance.
type NodeRole string

const (
	RoleLeader   NodeRole = "leader"
	RoleFollower NodeRole = "follower"
)

// ReplicationEntry represents a single entry in the replication log (WAL).
type ReplicationEntry struct {
	ID        string          `json:"id"`
	Operation string          `json:"operation"` // INSERT, UPDATE, DELETE
	TableName string          `json:"table_name"`
	RowID     string          `json:"row_id"`
	DataJSON  json.RawMessage `json:"data_json"`
	Timestamp time.Time       `json:"timestamp"`
	NodeID    string          `json:"node_id"`
	VectorClock VectorClock   `json:"vector_clock"`
}

// VectorClock implements a vector clock for conflict resolution.
type VectorClock map[string]int64

// Merge merges two vector clocks, taking the max of each entry.
func (vc VectorClock) Merge(other VectorClock) VectorClock {
	result := make(VectorClock)
	for k, v := range vc {
		result[k] = v
	}
	for k, v := range other {
		if existing, ok := result[k]; !ok || v > existing {
			result[k] = v
		}
	}
	return result
}

// Increment increments the clock for the given node.
func (vc VectorClock) Increment(nodeID string) {
	vc[nodeID]++
}

// HappensBefore returns true if vc happens before other.
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	atLeastOneSmaller := false
	for k, v := range vc {
		otherV, ok := other[k]
		if !ok {
			otherV = 0
		}
		if v > otherV {
			return false
		}
		if v < otherV {
			atLeastOneSmaller = true
		}
	}
	// Check keys in other but not in vc
	for k := range other {
		if _, ok := vc[k]; !ok {
			atLeastOneSmaller = true
		}
	}
	return atLeastOneSmaller
}

// Concurrent returns true if neither clock happens before the other.
func (vc VectorClock) Concurrent(other VectorClock) bool {
	return !vc.HappensBefore(other) && !other.HappensBefore(vc)
}

// PushRequest is sent from leader to followers.
type PushRequest struct {
	NodeID   string             `json:"node_id"`
	Entries  []ReplicationEntry `json:"entries"`
	AfterID  string             `json:"after_id,omitempty"` // Entries after this ID
}

// PushResponse is the follower's response to a push.
type PushResponse struct {
	Applied int    `json:"applied"`
	LastID  string `json:"last_id"`
	Error   string `json:"error,omitempty"`
}

// PullRequest is sent from followers to the leader.
type PullRequest struct {
	NodeID  string `json:"node_id"`
	AfterID string `json:"after_id"` // Pull entries after this ID
	Limit   int    `json:"limit"`
}

// PullResponse is the leader's response to a pull.
type PullResponse struct {
	Entries []ReplicationEntry `json:"entries"`
	HasMore bool               `json:"has_more"`
}

// ReplicationManager handles WAL-based replication between nodes.
type ReplicationManager struct {
	pool         *pgxpool.Pool
	nodeID       string
	role         NodeRole
	mu           sync.RWMutex
	followers    map[string]string // nodeID -> URL
	leaderURL    string
	vectorClock  VectorClock
}

// NewReplicationManager creates a new replication manager.
func NewReplicationManager(pool *pgxpool.Pool, nodeID string, role NodeRole) *ReplicationManager {
	return &ReplicationManager{
		pool:        pool,
		nodeID:      nodeID,
		role:        role,
		followers:   make(map[string]string),
		vectorClock: make(VectorClock),
	}
}

// NodeID returns this node's ID.
func (rm *ReplicationManager) NodeID() string {
	return rm.nodeID
}

// Role returns the current role.
func (rm *ReplicationManager) Role() NodeRole {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.role
}

// SetLeaderURL sets the leader URL for follower nodes.
func (rm *ReplicationManager) SetLeaderURL(url string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.leaderURL = url
}

// AddFollower registers a follower node.
func (rm *ReplicationManager) AddFollower(nodeID, url string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.followers[nodeID] = url
}

// RemoveFollower removes a follower node.
func (rm *ReplicationManager) RemoveFollower(nodeID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.followers, nodeID)
}

// RecordChange writes a change to the replication log.
func (rm *ReplicationManager) RecordChange(ctx context.Context, operation, tableName, rowID string, data interface{}) error {
	rm.mu.Lock()
	rm.vectorClock.Increment(rm.nodeID)
	vc := make(VectorClock)
	for k, v := range rm.vectorClock {
		vc[k] = v
	}
	rm.mu.Unlock()

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}

	vcJSON, err := json.Marshal(vc)
	if err != nil {
		return fmt.Errorf("marshaling vector clock: %w", err)
	}

	_, err = rm.pool.Exec(ctx,
		`INSERT INTO replication_log (operation, table_name, row_id, data_json, node_id, vector_clock)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		operation, tableName, rowID, dataJSON, rm.nodeID, vcJSON,
	)
	if err != nil {
		return fmt.Errorf("recording replication entry: %w", err)
	}

	return nil
}

// GetEntries retrieves replication log entries after a given ID.
func (rm *ReplicationManager) GetEntries(ctx context.Context, afterID string, limit int) ([]ReplicationEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var query string
	var args []interface{}

	if afterID == "" {
		query = `SELECT id, operation, table_name, row_id, data_json, timestamp, node_id, vector_clock
				 FROM replication_log
				 ORDER BY id ASC
				 LIMIT $1`
		args = []interface{}{limit}
	} else {
		query = `SELECT id, operation, table_name, row_id, data_json, timestamp, node_id, vector_clock
				 FROM replication_log
				 WHERE id > $1::uuid
				 ORDER BY id ASC
				 LIMIT $2`
		args = []interface{}{afterID, limit}
	}

	rows, err := rm.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying replication log: %w", err)
	}
	defer rows.Close()

	var entries []ReplicationEntry
	for rows.Next() {
		var e ReplicationEntry
		var vcJSON []byte
		if err := rows.Scan(&e.ID, &e.Operation, &e.TableName, &e.RowID, &e.DataJSON,
			&e.Timestamp, &e.NodeID, &vcJSON); err != nil {
			return nil, fmt.Errorf("scanning replication entry: %w", err)
		}
		if vcJSON != nil {
			json.Unmarshal(vcJSON, &e.VectorClock)
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// ApplyEntries applies replication entries from a remote node.
// Uses last-write-wins with vector clock conflict resolution.
func (rm *ReplicationManager) ApplyEntries(ctx context.Context, entries []ReplicationEntry) (int, error) {
	applied := 0

	for _, entry := range entries {
		// Skip entries from ourselves
		if entry.NodeID == rm.nodeID {
			continue
		}

		// Check for conflicts using vector clocks
		conflict, err := rm.checkConflict(ctx, entry)
		if err != nil {
			log.Printf("replication: conflict check failed for %s/%s: %v", entry.TableName, entry.RowID, err)
			continue
		}

		if conflict {
			// Last-write-wins: compare timestamps
			resolved, err := rm.resolveConflict(ctx, entry)
			if err != nil {
				log.Printf("replication: conflict resolution failed for %s/%s: %v", entry.TableName, entry.RowID, err)
				continue
			}
			if !resolved {
				continue // Local version wins
			}
		}

		// Record the entry in our replication log
		vcJSON, _ := json.Marshal(entry.VectorClock)
		_, err = rm.pool.Exec(ctx,
			`INSERT INTO replication_log (operation, table_name, row_id, data_json, node_id, vector_clock, timestamp)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT DO NOTHING`,
			entry.Operation, entry.TableName, entry.RowID, entry.DataJSON, entry.NodeID, vcJSON, entry.Timestamp,
		)
		if err != nil {
			log.Printf("replication: failed to record entry %s: %v", entry.ID, err)
			continue
		}

		// Merge vector clock
		rm.mu.Lock()
		rm.vectorClock = rm.vectorClock.Merge(entry.VectorClock)
		rm.mu.Unlock()

		applied++
	}

	return applied, nil
}

// checkConflict determines if a replication entry conflicts with local state.
func (rm *ReplicationManager) checkConflict(ctx context.Context, entry ReplicationEntry) (bool, error) {
	var count int
	err := rm.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM replication_log
		 WHERE table_name = $1 AND row_id = $2 AND node_id != $3
		 AND timestamp > $4 - INTERVAL '5 seconds'`,
		entry.TableName, entry.RowID, entry.NodeID, entry.Timestamp,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// resolveConflict uses last-write-wins to resolve a conflict.
// Returns true if the remote entry should be applied (remote wins).
func (rm *ReplicationManager) resolveConflict(ctx context.Context, entry ReplicationEntry) (bool, error) {
	// Get the latest local entry for this row
	var localTimestamp time.Time
	var localVCJSON []byte
	err := rm.pool.QueryRow(ctx,
		`SELECT timestamp, vector_clock FROM replication_log
		 WHERE table_name = $1 AND row_id = $2
		 ORDER BY timestamp DESC LIMIT 1`,
		entry.TableName, entry.RowID,
	).Scan(&localTimestamp, &localVCJSON)
	if err != nil {
		// No local entry, remote wins
		return true, nil
	}

	var localVC VectorClock
	if localVCJSON != nil {
		json.Unmarshal(localVCJSON, &localVC)
	}

	// If the remote vector clock happens after local, remote wins
	if localVC.HappensBefore(entry.VectorClock) {
		return true, nil
	}

	// If concurrent (true conflict), use timestamp as tiebreaker (last-write-wins)
	if localVC.Concurrent(entry.VectorClock) {
		return entry.Timestamp.After(localTimestamp), nil
	}

	// Local happens after remote: local wins
	return false, nil
}

// GetLastEntryID returns the ID of the last replication log entry.
func (rm *ReplicationManager) GetLastEntryID(ctx context.Context) (string, error) {
	var id *string
	err := rm.pool.QueryRow(ctx,
		`SELECT id FROM replication_log ORDER BY timestamp DESC LIMIT 1`,
	).Scan(&id)
	if err != nil {
		return "", nil // No entries yet
	}
	if id == nil {
		return "", nil
	}
	return *id, nil
}

// Status returns replication status information.
func (rm *ReplicationManager) Status(ctx context.Context) (map[string]interface{}, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var entryCount int
	rm.pool.QueryRow(ctx, `SELECT COUNT(*) FROM replication_log`).Scan(&entryCount)

	lastID, _ := rm.GetLastEntryID(ctx)

	return map[string]interface{}{
		"node_id":      rm.nodeID,
		"role":         rm.role,
		"entry_count":  entryCount,
		"last_entry":   lastID,
		"vector_clock": rm.vectorClock,
		"followers":    len(rm.followers),
	}, nil
}
