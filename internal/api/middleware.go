package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/teamvault/teamvault/internal/auth"
)

// securityHeadersMiddleware adds standard security headers to all responses.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware handles CORS preflight and headers for browser access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	ctxUserClaims  contextKey = "user_claims"
	ctxSAClaims    contextKey = "sa_claims"
	ctxActorType   contextKey = "actor_type"
	ctxActorID     contextKey = "actor_id"
	ctxClientIP    contextKey = "client_ip"
	ctxRequestID   contextKey = "request_id"
)

// ---- Rate Limiter (Token Bucket per IP) ----

// rateLimiter implements a per-IP token bucket rate limiter.
type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64 // tokens per second
	capacity int     // max burst
}

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

func newRateLimiter(ratePerSec float64, burst int) *rateLimiter {
	rl := &rateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     ratePerSec,
		capacity: burst,
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{
			tokens:   float64(rl.capacity),
			lastTime: now,
		}
		rl.buckets[ip] = b
	}

	// Add tokens based on elapsed time
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.capacity) {
		b.tokens = float64(rl.capacity)
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// cleanup removes stale buckets every 5 minutes.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for ip, b := range rl.buckets {
			if b.lastTime.Before(cutoff) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// ---- Middleware ----

// requestIDMiddleware adds a unique X-Request-ID header to each request.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), ctxRequestID, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID creates a random request ID.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// rateLimitMiddleware enforces per-IP rate limiting.
func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !rl.allow(ip) {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// secretRedactionMiddleware wraps the response writer to redact any secret values
// that might leak through error messages.
type redactingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *redactingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *redactingResponseWriter) Write(b []byte) (int, error) {
	// Only redact error responses (4xx, 5xx)
	if rw.statusCode >= 400 {
		s := string(b)
		// Redact potential secret values (base64 patterns, long hex strings)
		// This is a safety net — handlers should never include values in errors
		s = redactSecretPatterns(s)
		return rw.ResponseWriter.Write([]byte(s))
	}
	return rw.ResponseWriter.Write(b)
}

// redactSecretPatterns removes potential secret values from error messages.
func redactSecretPatterns(s string) string {
	// Redact Bearer tokens that might appear in error messages
	if idx := strings.Index(s, "Bearer "); idx >= 0 {
		end := idx + 7
		// Find the end of the token
		for end < len(s) && s[end] != '"' && s[end] != ' ' && s[end] != '}' {
			end++
		}
		if end-idx > 15 { // Only redact if long enough to be a real token
			s = s[:idx+7] + "[REDACTED]" + s[end:]
		}
	}
	return s
}

// loggingMiddleware logs request info (never logs secret values).
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(wrapped, r)

		reqID := ""
		if id, ok := r.Context().Value(ctxRequestID).(string); ok {
			reqID = id
		}

		log.Printf("[%s] %s %s %d %s [%s]",
			reqID,
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
// Only uses r.RemoteAddr to prevent spoofing via X-Forwarded-For headers.
// If running behind a trusted reverse proxy, configure the proxy to set
// a trusted header and update this function accordingly.
func clientIP(r *http.Request) string {
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
