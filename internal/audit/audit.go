// Package audit handles audit event recording with hash chaining
// to ensure tamper-evident logs.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/teamvault/teamvault/internal/db"
)

// Logger handles audit event creation with hash chaining.
type Logger struct {
	database *db.DB
	mu       sync.Mutex // Ensures serial hash chaining
}

// NewLogger creates a new audit logger.
func NewLogger(database *db.DB) *Logger {
	return &Logger{database: database}
}

// Event represents the data for creating an audit event.
type Event struct {
	ActorType string          // "user" or "service_account"
	ActorID   string
	Action    string          // e.g., "secret.read", "secret.write", "auth.login"
	Resource  string          // e.g., "myproject/api-keys/stripe"
	Outcome   string          // "success", "denied", "error"
	IP        string
	Metadata  json.RawMessage // Additional context (NEVER contains secret values)
}

// Log records an audit event with hash chaining.
// The hash chain provides tamper evidence: each event's hash includes the previous event's hash.
func (l *Logger) Log(ctx context.Context, event Event) (*db.AuditEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Get the previous hash for chaining
	prevHash, err := l.database.GetLastAuditHash(ctx)
	if err != nil {
		// Don't fail the operation because of audit, but log it
		prevHash = ""
	}

	// Compute hash: SHA-256 of (prev_hash + timestamp + actor + action + resource + outcome)
	hash := computeHash(prevHash, event)

	return l.database.CreateAuditEvent(
		ctx,
		event.ActorType,
		event.ActorID,
		event.Action,
		event.Resource,
		event.Outcome,
		event.IP,
		event.Metadata,
		prevHash,
		hash,
	)
}

// computeHash creates a SHA-256 hash for an audit event, chained to the previous hash.
func computeHash(prevHash string, event Event) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		prevHash,
		time.Now().UTC().Format(time.RFC3339Nano),
		event.ActorType+":"+event.ActorID,
		event.Action,
		event.Resource,
		event.Outcome,
		string(event.Metadata),
	)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}
