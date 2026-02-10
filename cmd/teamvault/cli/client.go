package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// APIClient handles HTTP communication with the TeamVault server.
type APIClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Message    string `json:"error"`
	Detail     string `json:"detail"`
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("API error (%d): %s — %s", e.StatusCode, e.Message, e.Detail)
	}
	if e.Message != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error (%d)", e.StatusCode)
}

// NewClient creates a new APIClient from stored credentials.
func NewClient() (*APIClient, error) {
	tokenData, err := LoadToken()
	if err != nil {
		return nil, err
	}
	return &APIClient{
		BaseURL: strings.TrimRight(tokenData.Server, "/"),
		Token:   tokenData.Token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// NewClientWithURL creates a new APIClient with an explicit server URL (for login).
func NewClientWithURL(serverURL string) *APIClient {
	return &APIClient{
		BaseURL: strings.TrimRight(serverURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *APIClient) do(method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		// Try to parse structured error
		_ = json.Unmarshal(respBody, apiErr)
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
		return apiErr
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// Login authenticates with email/password and returns a JWT token.
func (c *APIClient) Login(email, password string) (string, error) {
	var resp struct {
		Token string `json:"token"`
	}

	err := c.do("POST", "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, &resp)

	if err != nil {
		return "", err
	}

	if resp.Token == "" {
		return "", fmt.Errorf("server returned empty token")
	}

	return resp.Token, nil
}

// SecretResponse represents a secret returned by the API.
type SecretResponse struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Path        string `json:"path"`
	Value       string `json:"value"`
	Version     int    `json:"version"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
}

// SecretListItem is a secret in list output (no value exposed).
type SecretListItem struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	CreatedAt   string `json:"created_at"`
}

// GetSecret fetches a secret value.
func (c *APIClient) GetSecret(project, path string) (*SecretResponse, error) {
	var resp SecretResponse
	err := c.do("GET", fmt.Sprintf("/api/v1/secrets/%s/%s", project, path), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// PutSecret creates or updates a secret.
func (c *APIClient) PutSecret(project, path, value string) error {
	return c.do("PUT", fmt.Sprintf("/api/v1/secrets/%s/%s", project, path), map[string]string{
		"value": value,
	}, nil)
}

// ListSecrets lists secrets in a project.
func (c *APIClient) ListSecrets(project string) ([]SecretListItem, error) {
	var resp []SecretListItem
	err := c.do("GET", fmt.Sprintf("/api/v1/secrets/%s", project), nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ServiceAccountToken is the response from creating a service account.
type ServiceAccountToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// CreateServiceAccountToken creates a new service account token.
func (c *APIClient) CreateServiceAccountToken(project string, scopes []string, ttl string) (*ServiceAccountToken, error) {
	var resp ServiceAccountToken
	err := c.do("POST", "/api/v1/service-accounts", map[string]interface{}{
		"project": project,
		"scopes":  scopes,
		"ttl":     ttl,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
