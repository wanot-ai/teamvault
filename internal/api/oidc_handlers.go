package api

import (
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
)

func (s *Server) handleOIDCAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.oidcClient == nil {
		writeError(w, http.StatusNotImplemented, "OIDC is not configured")
		return
	}

	authURL, _, err := s.oidcClient.GetAuthorizationURL()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate authorization URL")
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidcClient == nil {
		writeError(w, http.StatusNotImplemented, "OIDC is not configured")
		return
	}

	ctx := r.Context()

	// Validate state parameter
	state := r.URL.Query().Get("state")
	if state == "" {
		writeError(w, http.StatusBadRequest, "missing state parameter")
		return
	}
	if !s.oidcClient.ValidateState(state) {
		writeError(w, http.StatusBadRequest, "invalid or expired state parameter")
		return
	}

	// Check for error response from provider
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		writeError(w, http.StatusBadRequest, "OIDC error: "+errParam+": "+errDesc)
		return
	}

	// Exchange authorization code for tokens
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	tokenResp, err := s.oidcClient.ExchangeCode(ctx, code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to exchange code: "+err.Error())
		return
	}

	// Get user info
	userInfo, err := s.oidcClient.GetUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user info")
		return
	}

	if userInfo.Email == "" {
		writeError(w, http.StatusBadRequest, "OIDC provider did not return an email")
		return
	}

	issuer := s.oidcClient.Issuer()

	// Try to find existing user by OIDC subject
	user, err := s.db.GetUserByOIDC(ctx, issuer, userInfo.Subject)
	if err != nil {
		// Try to find by email
		user, err = s.db.GetUserByEmail(ctx, userInfo.Email)
		if err != nil {
			// Create new user
			name := userInfo.Name
			if name == "" {
				name = strings.Split(userInfo.Email, "@")[0]
			}
			user, err = s.db.CreateOIDCUser(ctx, userInfo.Email, name, "member", issuer, userInfo.Subject)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to create user")
				return
			}
		}
	}

	// Generate JWT token
	token, err := s.auth.GenerateJWT(user.ID, user.Email, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Audit the OIDC login
	s.audit.Log(ctx, audit.Event{
		ActorType: "user",
		ActorID:   user.ID,
		Action:    "auth.oidc_login",
		Resource:  "user:" + user.ID,
		Outcome:   "success",
		IP:        clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}
