package db

import (
	"encoding/json"
	"time"
)

// User represents a user account.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// Project represents a secrets project/namespace.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Secret represents a secret entry (metadata only, no value).
type Secret struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Path        string     `json:"path"`
	Description string     `json:"description,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// SecretVersion represents an encrypted version of a secret value.
type SecretVersion struct {
	ID               string    `json:"id"`
	SecretID         string    `json:"secret_id"`
	Version          int       `json:"version"`
	Ciphertext       []byte    `json:"-"` // Never expose raw crypto in JSON
	Nonce            []byte    `json:"-"`
	EncryptedDEK     []byte    `json:"-"`
	DEKNonce         []byte    `json:"-"`
	MasterKeyVersion int       `json:"-"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
}

// ServiceAccount represents a service account for programmatic access.
type ServiceAccount struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"-"` // Never expose in JSON
	ProjectID string    `json:"project_id"`
	Scopes    []string  `json:"scopes"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Policy represents an access control policy.
type Policy struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Effect          string          `json:"effect"`
	Actions         []string        `json:"actions"`
	ResourcePattern string          `json:"resource_pattern"`
	SubjectType     string          `json:"subject_type"`
	SubjectID       *string         `json:"subject_id,omitempty"`
	Conditions      json.RawMessage `json:"conditions,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	ActorType string          `json:"actor_type"`
	ActorID   string          `json:"actor_id"`
	Action    string          `json:"action"`
	Resource  string          `json:"resource"`
	Outcome   string          `json:"outcome"`
	IP        string          `json:"ip,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	PrevHash  string          `json:"prev_hash,omitempty"`
	Hash      string          `json:"hash"`
}
