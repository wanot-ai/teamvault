package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
)

// registerRequest represents the registration payload.
type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// loginRequest represents the login payload.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// tokenResponse represents a token response.
type tokenResponse struct {
	Token string `json:"token"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "email, password, and name are required")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := s.auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process registration")
		return
	}

	// First user gets admin role
	role := "member"

	user, err := s.db.CreateUser(r.Context(), req.Email, hash, req.Name, role)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Audit the registration
	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   user.ID,
		Action:    "auth.register",
		Resource:  "user:" + user.ID,
		Outcome:   "success",
		IP:        clientIP(r),
	})

	token, err := s.auth.GenerateJWT(user.ID, user.Email, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := s.db.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Don't reveal whether the email exists
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := s.auth.CheckPassword(req.Password, user.PasswordHash); err != nil {
		// Audit failed login attempt
		s.audit.Log(r.Context(), audit.Event{
			ActorType: "user",
			ActorID:   user.ID,
			Action:    "auth.login",
			Resource:  "user:" + user.ID,
			Outcome:   "denied",
			IP:        clientIP(r),
			Metadata:  json.RawMessage(`{"reason":"invalid_password"}`),
		})
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.auth.GenerateJWT(user.ID, user.Email, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Audit successful login
	s.audit.Log(r.Context(), audit.Event{
		ActorType: "user",
		ActorID:   user.ID,
		Action:    "auth.login",
		Resource:  "user:" + user.ID,
		Outcome:   "success",
		IP:        clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "user authentication required")
		return
	}

	user, err := s.db.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	writeJSON(w, http.StatusOK, user)
}
