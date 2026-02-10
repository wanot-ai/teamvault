package api

import (
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/auth"
	"github.com/teamvault/teamvault/internal/crypto"
	"github.com/teamvault/teamvault/internal/db"
	"github.com/teamvault/teamvault/internal/lease"
	"github.com/teamvault/teamvault/internal/policy"
	"github.com/teamvault/teamvault/internal/replication"
	"github.com/teamvault/teamvault/internal/rotation"
	"github.com/teamvault/teamvault/internal/webhooks"
)

// Server holds all dependencies for the HTTP API.
type Server struct {
	db                  *db.DB
	auth                *auth.Auth
	crypto              *crypto.EnvelopeCrypto
	policy              *policy.Engine
	audit               *audit.Logger
	oidcClient          *auth.OIDCClient
	rotationScheduler   *rotation.Scheduler
	leaseManager        *lease.Manager
	teeHandlers         *TEEHandlers
	zkHandlers          *ZKHandlers
	webhookManager      *webhooks.WebhookManager
	replicationManager  *replication.ReplicationManager
	mux                 *http.ServeMux
	rl                  *rateLimiter
}

// ServerConfig holds optional dependencies for the server.
type ServerConfig struct {
	OIDCClient         *auth.OIDCClient
	RotationScheduler  *rotation.Scheduler
	LeaseManager       *lease.Manager
	TEEHandlers        *TEEHandlers
	ZKHandlers         *ZKHandlers
	WebhookManager     *webhooks.WebhookManager
	ReplicationManager *replication.ReplicationManager
}

// NewServer creates a new API server with all routes configured.
func NewServer(database *db.DB, authSvc *auth.Auth, cryptoSvc *crypto.EnvelopeCrypto, policySvc *policy.Engine, auditSvc *audit.Logger) *Server {
	return NewServerWithConfig(database, authSvc, cryptoSvc, policySvc, auditSvc, ServerConfig{})
}

// NewServerWithConfig creates a new API server with optional production dependencies.
func NewServerWithConfig(database *db.DB, authSvc *auth.Auth, cryptoSvc *crypto.EnvelopeCrypto, policySvc *policy.Engine, auditSvc *audit.Logger, config ServerConfig) *Server {
	s := &Server{
		db:                  database,
		auth:                authSvc,
		crypto:              cryptoSvc,
		policy:              policySvc,
		audit:               auditSvc,
		oidcClient:          config.OIDCClient,
		rotationScheduler:   config.RotationScheduler,
		leaseManager:        config.LeaseManager,
		teeHandlers:         config.TEEHandlers,
		zkHandlers:          config.ZKHandlers,
		webhookManager:      config.WebhookManager,
		replicationManager:  config.ReplicationManager,
		mux:                 http.NewServeMux(),
		rl:                  newRateLimiter(100, 200), // 100 req/s per IP, burst 200
	}

	s.setupRoutes()
	return s
}

// Handler returns the HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	// Chain middleware: request ID → rate limiting → logging → redaction → routes
	var handler http.Handler = s.mux
	handler = s.loggingMiddleware(handler)
	handler = rateLimitMiddleware(s.rl)(handler)
	handler = requestIDMiddleware(handler)
	handler = corsMiddleware(handler)
	return handler
}

// DB returns the database for use by health checks.
func (s *Server) DB() *db.DB {
	return s.db
}

// setupRoutes configures all API routes.
func (s *Server) setupRoutes() {
	// Health check (no auth required)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)

	// Auth endpoints (no auth required)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)

	// OIDC endpoints (no auth required)
	s.mux.HandleFunc("GET /api/v1/auth/oidc/authorize", s.handleOIDCAuthorize)
	s.mux.HandleFunc("GET /api/v1/auth/oidc/callback", s.handleOIDCCallback)

	// Auth-required endpoints
	s.mux.Handle("GET /api/v1/auth/me", s.authMiddleware(http.HandlerFunc(s.handleMe)))

	// Projects
	s.mux.Handle("POST /api/v1/projects", s.authMiddleware(http.HandlerFunc(s.handleCreateProject)))
	s.mux.Handle("GET /api/v1/projects", s.authMiddleware(http.HandlerFunc(s.handleListProjects)))

	// Secrets
	s.mux.Handle("PUT /api/v1/secrets/{project}/{path...}", s.authMiddleware(http.HandlerFunc(s.handlePutSecret)))
	s.mux.Handle("GET /api/v1/secrets/{project}/{path...}", s.authMiddleware(http.HandlerFunc(s.handleGetSecret)))
	s.mux.Handle("GET /api/v1/secrets/{project}", s.authMiddleware(http.HandlerFunc(s.handleListSecrets)))
	s.mux.Handle("DELETE /api/v1/secrets/{project}/{path...}", s.authMiddleware(http.HandlerFunc(s.handleDeleteSecret)))

	// Rotation (POST to path-based endpoints, handled via path suffix matching)
	s.mux.Handle("POST /api/v1/secrets/{project}/{path...}", s.authMiddleware(http.HandlerFunc(s.handleSecretPost)))

	// Secret versions
	s.mux.Handle("GET /api/v1/secret-versions/{project}/{path...}", s.authMiddleware(http.HandlerFunc(s.handleListSecretVersions)))

	// Service Accounts
	s.mux.Handle("POST /api/v1/service-accounts", s.authMiddleware(http.HandlerFunc(s.handleCreateServiceAccount)))
	s.mux.Handle("GET /api/v1/service-accounts", s.authMiddleware(http.HandlerFunc(s.handleListServiceAccounts)))

	// Policies
	s.mux.Handle("POST /api/v1/policies", s.authMiddleware(s.adminOnly(http.HandlerFunc(s.handleCreatePolicy))))
	s.mux.Handle("GET /api/v1/policies", s.authMiddleware(http.HandlerFunc(s.handleListPolicies)))

	// Audit
	s.mux.Handle("GET /api/v1/audit", s.authMiddleware(http.HandlerFunc(s.handleListAuditEvents)))

	// Organizations
	s.mux.Handle("POST /api/v1/orgs", s.authMiddleware(http.HandlerFunc(s.handleCreateOrg)))
	s.mux.Handle("GET /api/v1/orgs", s.authMiddleware(http.HandlerFunc(s.handleListOrgs)))
	s.mux.Handle("GET /api/v1/orgs/{id}", s.authMiddleware(http.HandlerFunc(s.handleGetOrg)))

	// Teams (nested under orgs)
	s.mux.Handle("POST /api/v1/orgs/{id}/teams", s.authMiddleware(http.HandlerFunc(s.handleCreateTeam)))
	s.mux.Handle("GET /api/v1/orgs/{id}/teams", s.authMiddleware(http.HandlerFunc(s.handleListTeams)))

	// Team Members
	s.mux.Handle("POST /api/v1/teams/{id}/members", s.authMiddleware(http.HandlerFunc(s.handleAddTeamMember)))
	s.mux.Handle("DELETE /api/v1/teams/{id}/members", s.authMiddleware(http.HandlerFunc(s.handleRemoveTeamMember)))
	s.mux.Handle("GET /api/v1/teams/{id}/members", s.authMiddleware(http.HandlerFunc(s.handleListTeamMembers)))

	// Agents (nested under teams)
	s.mux.Handle("POST /api/v1/teams/{id}/agents", s.authMiddleware(http.HandlerFunc(s.handleCreateAgent)))
	s.mux.Handle("GET /api/v1/teams/{id}/agents", s.authMiddleware(http.HandlerFunc(s.handleListAgents)))
	s.mux.Handle("GET /api/v1/agents/{agentId}", s.authMiddleware(http.HandlerFunc(s.handleGetAgent)))
	s.mux.Handle("DELETE /api/v1/agents/{agentId}", s.authMiddleware(http.HandlerFunc(s.handleDeleteAgent)))

	// IAM Policies
	s.mux.Handle("POST /api/v1/iam-policies", s.authMiddleware(http.HandlerFunc(s.handleCreateIAMPolicy)))
	s.mux.Handle("GET /api/v1/iam-policies", s.authMiddleware(http.HandlerFunc(s.handleListIAMPolicies)))
	s.mux.Handle("GET /api/v1/iam-policies/{id}", s.authMiddleware(http.HandlerFunc(s.handleGetIAMPolicy)))
	s.mux.Handle("PUT /api/v1/iam-policies/{id}", s.authMiddleware(http.HandlerFunc(s.handleUpdateIAMPolicy)))
	s.mux.Handle("DELETE /api/v1/iam-policies/{id}", s.authMiddleware(http.HandlerFunc(s.handleDeleteIAMPolicy)))

	// Leases (Dynamic Secrets)
	s.mux.Handle("POST /api/v1/lease/database", s.authMiddleware(http.HandlerFunc(s.handleIssueDatabaseLease)))
	s.mux.Handle("POST /api/v1/lease/{id}/revoke", s.authMiddleware(http.HandlerFunc(s.handleRevokeLease)))
	s.mux.Handle("GET /api/v1/leases", s.authMiddleware(http.HandlerFunc(s.handleListLeases)))

	// Dashboard
	s.mux.Handle("GET /api/v1/dashboard/stats", s.authMiddleware(http.HandlerFunc(s.handleDashboardStats)))

	// Teams (all teams, not scoped to an org)
	s.mux.Handle("GET /api/v1/teams", s.authMiddleware(http.HandlerFunc(s.handleListAllTeams)))

	// Rotation status (GET via separate route to avoid {path...}/rotation conflict)
	s.mux.Handle("GET /api/v1/rotation-status/{project}/{path...}", s.authMiddleware(http.HandlerFunc(s.handleGetRotationStatus)))

	// TEE (Trusted Execution Environment)
	s.mux.Handle("GET /api/v1/tee/attestation", s.authMiddleware(http.HandlerFunc(s.handleTEEAttestation)))
	s.mux.Handle("POST /api/v1/tee/session", s.authMiddleware(http.HandlerFunc(s.handleTEESession)))
	s.mux.Handle("POST /api/v1/tee/read", s.authMiddleware(http.HandlerFunc(s.handleTEERead)))

	// ZK (Zero-Knowledge) Auth
	s.mux.Handle("POST /api/v1/auth/zk/credential", s.authMiddleware(http.HandlerFunc(s.handleZKCredential)))
	s.mux.HandleFunc("POST /api/v1/auth/zk/verify", s.handleZKVerify) // No auth required for verification

	// Webhooks
	s.mux.Handle("POST /api/v1/webhooks", s.authMiddleware(http.HandlerFunc(s.handleRegisterWebhook)))
	s.mux.Handle("GET /api/v1/webhooks", s.authMiddleware(http.HandlerFunc(s.handleListWebhooks)))
	s.mux.Handle("DELETE /api/v1/webhooks/{id}", s.authMiddleware(http.HandlerFunc(s.handleDeleteWebhook)))
	s.mux.Handle("POST /api/v1/webhooks/{id}/test", s.authMiddleware(http.HandlerFunc(s.handleTestWebhook)))

	// Replication
	s.mux.Handle("POST /api/v1/replication/push", s.authMiddleware(http.HandlerFunc(s.handleReplicationPush)))
	s.mux.Handle("POST /api/v1/replication/pull", s.authMiddleware(http.HandlerFunc(s.handleReplicationPull)))
	s.mux.Handle("GET /api/v1/replication/status", s.authMiddleware(http.HandlerFunc(s.handleReplicationStatus)))
}

// handleSecretPost dispatches POST requests to secrets paths based on suffix.
// POST /api/v1/secrets/{project}/{path...} handles:
//   - .../rotation → set rotation schedule
//   - .../rotate → manual rotate
func (s *Server) handleSecretPost(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	if strings.HasSuffix(path, "/rotation") {
		s.handleSetRotation(w, r)
		return
	}
	if strings.HasSuffix(path, "/rotate") {
		s.handleManualRotate(w, r)
		return
	}

	writeError(w, http.StatusBadRequest, "use PUT to create/update secrets, or POST to .../rotation or .../rotate")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
