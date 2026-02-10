package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/teamvault/teamvault/internal/audit"
)

// webhookRegisterRequest is the request body for registering a webhook.
type webhookRegisterRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// webhookRegisterResponse is the response after registering a webhook.
type webhookRegisterResponse struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Secret    string   `json:"secret"` // Only returned once on creation
	CreatedAt string   `json:"created_at"`
}

// handleRegisterWebhook registers a new webhook.
// POST /api/v1/webhooks
func (s *Server) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookManager == nil {
		writeError(w, http.StatusServiceUnavailable, "webhooks not available")
		return
	}

	ctx := r.Context()
	claims := getUserClaims(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req webhookRegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "at least one event is required")
		return
	}

	// Validate URL starts with https:// (security)
	if !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "http://localhost") {
		writeError(w, http.StatusBadRequest, "webhook URL must use HTTPS")
		return
	}

	// Generate a random HMAC secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate webhook secret")
		return
	}
	webhookSecret := hex.EncodeToString(secretBytes)

	// Use user's associated org (fallback to user ID as org placeholder)
	orgID := claims.UserID // In a real system, this would be the user's org

	webhook, err := s.webhookManager.Register(ctx, orgID, req.URL, webhookSecret, req.Events)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Audit
	s.audit.Log(ctx, audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "webhook.created",
		Resource:  "webhook/" + webhook.ID,
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"url":"` + req.URL + `"}`),
	})

	writeJSON(w, http.StatusCreated, webhookRegisterResponse{
		ID:        webhook.ID,
		URL:       webhook.URL,
		Events:    webhook.Events,
		Secret:    webhookSecret, // Only shown once
		CreatedAt: webhook.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// handleListWebhooks lists all webhooks for the user's organization.
// GET /api/v1/webhooks
func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	if s.webhookManager == nil {
		writeError(w, http.StatusServiceUnavailable, "webhooks not available")
		return
	}

	ctx := r.Context()
	claims := getUserClaims(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	orgID := claims.UserID

	webhooks, err := s.webhookManager.List(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	type webhookItem struct {
		ID        string   `json:"id"`
		URL       string   `json:"url"`
		Events    []string `json:"events"`
		Active    bool     `json:"active"`
		CreatedAt string   `json:"created_at"`
	}

	items := make([]webhookItem, 0, len(webhooks))
	for _, wh := range webhooks {
		items = append(items, webhookItem{
			ID:        wh.ID,
			URL:       wh.URL,
			Events:    wh.Events,
			Active:    wh.Active,
			CreatedAt: wh.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, http.StatusOK, items)
}

// handleDeleteWebhook removes a webhook.
// DELETE /api/v1/webhooks/{id}
func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookManager == nil {
		writeError(w, http.StatusServiceUnavailable, "webhooks not available")
		return
	}

	ctx := r.Context()
	claims := getUserClaims(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	webhookID := r.PathValue("id")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	orgID := claims.UserID

	if err := s.webhookManager.Unregister(ctx, webhookID, orgID); err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	s.audit.Log(ctx, audit.Event{
		ActorType: "user",
		ActorID:   claims.UserID,
		Action:    "webhook.deleted",
		Resource:  "webhook/" + webhookID,
		Outcome:   "success",
		IP:        getClientIP(ctx),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleTestWebhook sends a test ping to a webhook.
// POST /api/v1/webhooks/{id}/test
func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookManager == nil {
		writeError(w, http.StatusServiceUnavailable, "webhooks not available")
		return
	}

	ctx := r.Context()
	claims := getUserClaims(ctx)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	webhookID := r.PathValue("id")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	orgID := claims.UserID

	statusCode, err := s.webhookManager.SendTestPing(ctx, webhookID, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "sent",
		"status_code": statusCode,
	})
}
