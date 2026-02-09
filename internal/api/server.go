package api

import (
	"net/http"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/auth"
	"github.com/teamvault/teamvault/internal/crypto"
	"github.com/teamvault/teamvault/internal/db"
	"github.com/teamvault/teamvault/internal/policy"
)

// Server holds all dependencies for the HTTP API.
type Server struct {
	db     *db.DB
	auth   *auth.Auth
	crypto *crypto.EnvelopeCrypto
	policy *policy.Engine
	audit  *audit.Logger
	mux    *http.ServeMux
}

// NewServer creates a new API server with all routes configured.
func NewServer(database *db.DB, authSvc *auth.Auth, cryptoSvc *crypto.EnvelopeCrypto, policySvc *policy.Engine, auditSvc *audit.Logger) *Server {
	s := &Server{
		db:     database,
		auth:   authSvc,
		crypto: cryptoSvc,
		policy: policySvc,
		audit:  auditSvc,
		mux:    http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

// Handler returns the HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	return s.loggingMiddleware(s.mux)
}

// setupRoutes configures all API routes.
func (s *Server) setupRoutes() {
	// Health check
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Auth endpoints (no auth required)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)

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
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
