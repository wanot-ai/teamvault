package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/teamvault/teamvault/internal/auth"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	ctxUserClaims  contextKey = "user_claims"
	ctxSAClaims    contextKey = "sa_claims"
	ctxActorType   contextKey = "actor_type"
	ctxActorID     contextKey = "actor_id"
	ctxClientIP    contextKey = "client_ip"
)

// loggingMiddleware logs request info (never logs secret values).
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(wrapped, r)
		log.Printf("%s %s %d %s [%s]",
			r.Method, r.URL.Path, wrapped.status,
			time.Since(start).Round(time.Millisecond),
			clientIP(r),
		)
	})
}

// statusRecorder wraps http.ResponseWriter to capture status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// authMiddleware validates JWT or service account tokens.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxClientIP, clientIP(r))

		// Check if this is a service account token (prefixed with "sa.")
		if strings.HasPrefix(token, "sa.") {
			saToken := strings.TrimPrefix(token, "sa.")
			sa, err := s.db.FindServiceAccountByToken(ctx, func(hash string) bool {
				return s.auth.ValidateServiceAccountToken(saToken, hash) == nil
			})
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid service account token")
				return
			}

			// Check expiration
			if sa.ExpiresAt != nil && sa.ExpiresAt.Before(time.Now()) {
				writeError(w, http.StatusUnauthorized, "service account token expired")
				return
			}

			saClaims := &auth.ServiceAccountClaims{
				ServiceAccountID: sa.ID,
				ProjectID:        sa.ProjectID,
				Scopes:           sa.Scopes,
			}
			ctx = context.WithValue(ctx, ctxSAClaims, saClaims)
			ctx = context.WithValue(ctx, ctxActorType, "service_account")
			ctx = context.WithValue(ctx, ctxActorID, sa.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Regular JWT token
		claims, err := s.auth.ValidateJWT(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx = context.WithValue(ctx, ctxUserClaims, claims)
		ctx = context.WithValue(ctx, ctxActorType, "user")
		ctx = context.WithValue(ctx, ctxActorID, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getUserClaims extracts user claims from context.
func getUserClaims(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(ctxUserClaims).(*auth.Claims)
	return claims
}

// getSAClaims extracts service account claims from context.
func getSAClaims(ctx context.Context) *auth.ServiceAccountClaims {
	claims, _ := ctx.Value(ctxSAClaims).(*auth.ServiceAccountClaims)
	return claims
}

// getActorType returns the actor type from context.
func getActorType(ctx context.Context) string {
	t, _ := ctx.Value(ctxActorType).(string)
	return t
}

// getActorID returns the actor ID from context.
func getActorID(ctx context.Context) string {
	id, _ := ctx.Value(ctxActorID).(string)
	return id
}

// getClientIP returns the client IP from context.
func getClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(ctxClientIP).(string)
	return ip
}

// isAdmin checks if the current user is an admin.
func isAdmin(ctx context.Context) bool {
	claims := getUserClaims(ctx)
	return claims != nil && claims.Role == "admin"
}

// adminOnly middleware restricts access to admin users.
func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAdmin(r.Context()) {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the client IP from the request.
func clientIP(r *http.Request) string {
	// Check X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
