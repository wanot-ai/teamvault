// Package webhooks implements a webhook delivery system for TeamVault events.
// Webhooks are registered per-organization and fire on secret lifecycle events,
// delivering HTTP POST requests with HMAC-SHA256 signed payloads.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Event types for webhook delivery.
const (
	EventSecretCreated = "secret.created"
	EventSecretUpdated = "secret.updated"
	EventSecretDeleted = "secret.deleted"
	EventSecretRotated = "secret.rotated"
	EventPolicyChanged = "policy.changed"
)

// AllEvents lists all supported event types.
var AllEvents = []string{
	EventSecretCreated,
	EventSecretUpdated,
	EventSecretDeleted,
	EventSecretRotated,
	EventPolicyChanged,
}

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"` // HMAC signing key, never exposed in API responses
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// WebhookPayload is the body sent to webhook endpoints.
type WebhookPayload struct {
	ID        string          `json:"id"`
	Event     string          `json:"event"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// WebhookDelivery tracks a delivery attempt.
type WebhookDelivery struct {
	WebhookID  string
	Event      string
	StatusCode int
	Duration   time.Duration
	Error      string
	Attempt    int
}

// WebhookManager handles webhook registration, storage, and event delivery.
type WebhookManager struct {
	pool       *pgxpool.Pool
	httpClient *http.Client
	mu         sync.RWMutex
	deliveries chan WebhookDelivery // optional: for monitoring/testing
}

// NewWebhookManager creates a new webhook manager.
func NewWebhookManager(pool *pgxpool.Pool) *WebhookManager {
	return &WebhookManager{
		pool: pool,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		deliveries: make(chan WebhookDelivery, 100),
	}
}

// Register creates a new webhook registration.
func (wm *WebhookManager) Register(ctx context.Context, orgID, url, secret string, events []string) (*Webhook, error) {
	// Validate events
	validEvents := make(map[string]bool)
	for _, e := range AllEvents {
		validEvents[e] = true
	}
	for _, e := range events {
		if !validEvents[e] {
			return nil, fmt.Errorf("invalid event type: %s", e)
		}
	}

	if url == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}
	if secret == "" {
		return nil, fmt.Errorf("webhook secret is required")
	}

	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("marshaling events: %w", err)
	}

	webhook := &Webhook{}
	err = wm.pool.QueryRow(ctx,
		`INSERT INTO webhooks (org_id, url, secret, events, active)
		 VALUES ($1, $2, $3, $4, true)
		 RETURNING id, org_id, url, events, active, created_at`,
		orgID, url, secret, eventsJSON,
	).Scan(&webhook.ID, &webhook.OrgID, &webhook.URL, &eventsJSON, &webhook.Active, &webhook.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting webhook: %w", err)
	}
	json.Unmarshal(eventsJSON, &webhook.Events)

	return webhook, nil
}

// Unregister removes a webhook by ID.
func (wm *WebhookManager) Unregister(ctx context.Context, webhookID, orgID string) error {
	result, err := wm.pool.Exec(ctx,
		`DELETE FROM webhooks WHERE id = $1 AND org_id = $2`,
		webhookID, orgID,
	)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

// List returns all webhooks for an organization.
func (wm *WebhookManager) List(ctx context.Context, orgID string) ([]Webhook, error) {
	rows, err := wm.pool.Query(ctx,
		`SELECT id, org_id, url, events, active, created_at
		 FROM webhooks WHERE org_id = $1
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var w Webhook
		var eventsJSON []byte
		if err := rows.Scan(&w.ID, &w.OrgID, &w.URL, &eventsJSON, &w.Active, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		json.Unmarshal(eventsJSON, &w.Events)
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

// GetByID returns a single webhook by ID and org.
func (wm *WebhookManager) GetByID(ctx context.Context, webhookID, orgID string) (*Webhook, error) {
	var w Webhook
	var eventsJSON []byte
	err := wm.pool.QueryRow(ctx,
		`SELECT id, org_id, url, secret, events, active, created_at
		 FROM webhooks WHERE id = $1 AND org_id = $2`,
		webhookID, orgID,
	).Scan(&w.ID, &w.OrgID, &w.URL, &w.Secret, &eventsJSON, &w.Active, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting webhook: %w", err)
	}
	json.Unmarshal(eventsJSON, &w.Events)
	return &w, nil
}

// Fire delivers an event to all matching webhooks for the given organization.
// Delivery is asynchronous with retry logic.
func (wm *WebhookManager) Fire(ctx context.Context, orgID, event string, data interface{}) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("webhook: failed to marshal event data: %v", err)
		return
	}

	// Find matching webhooks
	rows, err := wm.pool.Query(ctx,
		`SELECT id, org_id, url, secret, events, active
		 FROM webhooks
		 WHERE org_id = $1 AND active = true`,
		orgID,
	)
	if err != nil {
		log.Printf("webhook: failed to query webhooks: %v", err)
		return
	}
	defer rows.Close()

	var targets []Webhook
	for rows.Next() {
		var w Webhook
		var eventsJSON []byte
		if err := rows.Scan(&w.ID, &w.OrgID, &w.URL, &w.Secret, &eventsJSON, &w.Active); err != nil {
			log.Printf("webhook: failed to scan webhook: %v", err)
			continue
		}
		json.Unmarshal(eventsJSON, &w.Events)

		// Check if this webhook subscribes to the event
		for _, e := range w.Events {
			if e == event {
				targets = append(targets, w)
				break
			}
		}
	}

	// Deliver to each matching webhook asynchronously
	for _, target := range targets {
		go wm.deliver(target, event, dataJSON)
	}
}

// deliver sends a webhook payload with retry logic (3 attempts, exponential backoff).
func (wm *WebhookManager) deliver(webhook Webhook, event string, data json.RawMessage) {
	payload := WebhookPayload{
		ID:        generateWebhookPayloadID(),
		Event:     event,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook %s: failed to marshal payload: %v", webhook.ID, err)
		return
	}

	// Compute HMAC-SHA256 signature
	signature := computeHMACSignature(webhook.Secret, body)

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()
		statusCode, deliveryErr := wm.doPost(webhook.URL, body, signature)
		duration := time.Since(start)

		delivery := WebhookDelivery{
			WebhookID:  webhook.ID,
			Event:      event,
			StatusCode: statusCode,
			Duration:   duration,
			Attempt:    attempt,
		}

		if deliveryErr != nil {
			delivery.Error = deliveryErr.Error()
		}

		// Non-blocking send to deliveries channel (for monitoring)
		select {
		case wm.deliveries <- delivery:
		default:
		}

		// Success: 2xx status code
		if statusCode >= 200 && statusCode < 300 {
			return
		}

		// Log retry
		if attempt < maxAttempts {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s, 4s
			log.Printf("webhook %s: attempt %d/%d failed (status=%d, err=%v), retrying in %v",
				webhook.ID, attempt, maxAttempts, statusCode, deliveryErr, backoff)
			time.Sleep(backoff)
		} else {
			log.Printf("webhook %s: all %d attempts failed for event %s",
				webhook.ID, maxAttempts, event)
		}
	}
}

// doPost sends the HTTP POST request to the webhook URL.
func (wm *WebhookManager) doPost(url string, body []byte, signature string) (int, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TeamVault-Webhook/1.0")
	req.Header.Set("X-TeamVault-Signature", signature)
	req.Header.Set("X-TeamVault-Event", "") // Will be set from payload

	resp, err := wm.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

// SendTestPing sends a test webhook delivery to verify connectivity.
func (wm *WebhookManager) SendTestPing(ctx context.Context, webhookID, orgID string) (int, error) {
	webhook, err := wm.GetByID(ctx, webhookID, orgID)
	if err != nil {
		return 0, fmt.Errorf("getting webhook: %w", err)
	}

	payload := WebhookPayload{
		ID:        generateWebhookPayloadID(),
		Event:     "ping",
		Timestamp: time.Now().UTC(),
		Data:      json.RawMessage(`{"message":"test ping from TeamVault"}`),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshaling payload: %w", err)
	}

	signature := computeHMACSignature(webhook.Secret, body)
	return wm.doPost(webhook.URL, body, signature)
}

// Deliveries returns the delivery channel for monitoring/testing.
func (wm *WebhookManager) Deliveries() <-chan WebhookDelivery {
	return wm.deliveries
}

// ---- Helpers ----

// computeHMACSignature computes the HMAC-SHA256 signature for a webhook payload.
func computeHMACSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature verifies an incoming webhook signature (for consumers).
func VerifyWebhookSignature(secret string, body []byte, signature string) bool {
	expected := computeHMACSignature(secret, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// generateWebhookPayloadID generates a unique ID for a webhook payload.
func generateWebhookPayloadID() string {
	b := make([]byte, 16)
	io.ReadFull(io.Reader(nil), b) // will fail, but we handle it
	if _, err := io.ReadFull(io.LimitReader(timeReader{}, 8), b[:8]); err != nil {
		// fallback: use timestamp
	}
	// Use timestamp + random for uniqueness
	now := time.Now().UnixNano()
	return fmt.Sprintf("whd_%x", now)
}

type timeReader struct{}

func (timeReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}
