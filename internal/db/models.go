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
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	Path        string          `json:"path"`
	Description string          `json:"description,omitempty"`
	SecretType  string          `json:"secret_type"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
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

// Org represents an organization.
type Org struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Team represents a team within an organization.
type Team struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// TeamMember represents a user's membership in a team.
type TeamMember struct {
	TeamID   string    `json:"team_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// Agent represents a machine/CI agent belonging to a team.
type Agent struct {
	ID          string          `json:"id"`
	TeamID      string          `json:"team_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	TokenHash   string          `json:"-"` // Never expose in JSON
	Scopes      []string        `json:"scopes"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
}

// IAMPolicy represents an enterprise IAM policy (RBAC, ABAC, or PBAC).
type IAMPolicy struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"org_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	PolicyType  string          `json:"policy_type"`
	PolicyDoc   json.RawMessage `json:"policy_doc"`
	HCLSource   string          `json:"hcl_source,omitempty"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
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

// RotationSchedule represents a secret rotation schedule.
type RotationSchedule struct {
	ID              string          `json:"id"`
	SecretID        string          `json:"secret_id"`
	CronExpression  string          `json:"cron_expression"`
	ConnectorType   string          `json:"connector_type"`
	ConnectorConfig json.RawMessage `json:"connector_config,omitempty"`
	LastRotatedAt   *time.Time      `json:"last_rotated_at,omitempty"`
	NextRotationAt  time.Time       `json:"next_rotation_at"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Lease represents a dynamic secret lease.
type Lease struct {
	ID              string     `json:"id"`
	OrgID           *string    `json:"org_id,omitempty"`
	SecretPath      string     `json:"secret_path"`
	LeaseType       string     `json:"lease_type"`
	ValueCiphertext []byte     `json:"-"`
	EncryptedDEK    []byte     `json:"-"`
	Nonce           []byte     `json:"-"`
	DEKNonce        []byte     `json:"-"`
	IssuedTo        string     `json:"issued_to"`
	IssuedAt        time.Time  `json:"issued_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
}
