package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/teamvault/teamvault/internal/audit"
	"github.com/teamvault/teamvault/internal/rotation"
)

type setRotationRequest struct {
	CronExpression  string          `json:"cron_expression"`  // e.g., "@every 24h", "0 0 * * *"
	ConnectorType   string          `json:"connector_type"`   // e.g., "random_password"
	ConnectorConfig json.RawMessage `json:"connector_config"` // Connector-specific config
}

type rotationResponse struct {
	ID              string     `json:"id"`
	SecretID        string     `json:"secret_id"`
	CronExpression  string     `json:"cron_expression"`
	ConnectorType   string     `json:"connector_type"`
	LastRotatedAt   *time.Time `json:"last_rotated_at,omitempty"`
	NextRotationAt  time.Time  `json:"next_rotation_at"`
	Status          string     `json:"status"`
}

func (s *Server) handleSetRotation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")
	secretPath := r.PathValue("path")
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	// Strip /rotation suffix
	secretPath = strings.TrimSuffix(secretPath, "/rotation")

	if projectName == "" || secretPath == "" {
		writeError(w, http.StatusBadRequest, "project and path are required")
		return
	}

	resource := projectName + "/" + secretPath

	// Only admins can set rotation schedules
	if !isAdmin(ctx) {
		writeError(w, http.StatusForbidden, "admin access required to set rotation")
		return
	}

	var req setRotationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CronExpression == "" {
		writeError(w, http.StatusBadRequest, "cron_expression is required")
		return
	}
	if req.ConnectorType == "" {
		writeError(w, http.StatusBadRequest, "connector_type is required")
		return
	}

	// Validate connector exists
	if s.rotationScheduler != nil {
		connector, err := s.rotationScheduler.Registry().Get(req.ConnectorType)
		if err != nil {
			writeError(w, http.StatusBadRequest, "unknown connector_type: "+req.ConnectorType)
			return
		}
		if err := connector.Validate(req.ConnectorConfig); err != nil {
			writeError(w, http.StatusBadRequest, "invalid connector_config: "+err.Error())
			return
		}
	}

	// Resolve the secret
	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	secret, err := s.db.GetSecret(ctx, project.ID, secretPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	// Calculate initial next rotation
	nextRotation := rotation.ComputeNextRotationExported(req.CronExpression)

	schedule, err := s.db.CreateRotationSchedule(ctx, secret.ID, req.CronExpression,
		req.ConnectorType, req.ConnectorConfig, nextRotation)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rotation schedule")
		return
	}

	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "rotation.set",
		Resource:  resource,
		Outcome:   "success",
		IP:        getClientIP(ctx),
		Metadata:  json.RawMessage(`{"connector":"` + req.ConnectorType + `","cron":"` + req.CronExpression + `"}`),
	})

	writeJSON(w, http.StatusOK, rotationResponse{
		ID:             schedule.ID,
		SecretID:       schedule.SecretID,
		CronExpression: schedule.CronExpression,
		ConnectorType:  schedule.ConnectorType,
		LastRotatedAt:  schedule.LastRotatedAt,
		NextRotationAt: schedule.NextRotationAt,
		Status:         schedule.Status,
	})
}

func (s *Server) handleManualRotate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectName := r.PathValue("project")
	secretPath := r.PathValue("path")
	actorType := getActorType(ctx)
	actorID := getActorID(ctx)

	// Strip /rotate suffix
	secretPath = strings.TrimSuffix(secretPath, "/rotate")

	if projectName == "" || secretPath == "" {
		writeError(w, http.StatusBadRequest, "project and path are required")
		return
	}

	resource := projectName + "/" + secretPath

	// Only admins can manually rotate
	if !isAdmin(ctx) {
		writeError(w, http.StatusForbidden, "admin access required to rotate secrets")
		return
	}

	// Resolve the secret
	project, err := s.db.GetProjectByName(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	secret, err := s.db.GetSecret(ctx, project.ID, secretPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	if s.rotationScheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "rotation scheduler not available")
		return
	}

	if err := s.rotationScheduler.RotateSecret(ctx, secret.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "rotation failed: "+err.Error())
		return
	}

	s.audit.Log(ctx, audit.Event{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    "rotation.manual",
		Resource:  resource,
		Outcome:   "success",
		IP:        getClientIP(ctx),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "rotated"})
}
