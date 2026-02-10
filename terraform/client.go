package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TeamVaultClient is the HTTP client for the TeamVault API.
type TeamVaultClient struct {
	Address    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new TeamVault API client.
func NewClient(address, token string) *TeamVaultClient {
	return &TeamVaultClient{
		Address: address,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- Request / Response Types ---

// SecretRequest represents a secret create/update request.
type SecretRequest struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
}

// SecretResponse represents the API response for a secret.
type SecretResponse struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	Project     string          `json:"project"`
	Path        string          `json:"path"`
	Description string          `json:"description,omitempty"`
	SecretType  string          `json:"secret_type"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Version     int             `json:"version"`
	Value       string          `json:"value,omitempty"`
	CreatedBy   string          `json:"created_by"`
}

// ProjectRequest represents a project create request.
type ProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ProjectResponse represents the API response for a project.
type ProjectResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
}

// PolicyRequest represents a policy create/update request.
type PolicyRequest struct {
	Name            string   `json:"name"`
	Effect          string   `json:"effect"`
	Actions         []string `json:"actions"`
	ResourcePattern string   `json:"resource_pattern"`
	SubjectType     string   `json:"subject_type"`
	SubjectID       *string  `json:"subject_id,omitempty"`
}

// PolicyResponse represents the API response for a policy.
type PolicyResponse struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Effect          string          `json:"effect"`
	Actions         []string        `json:"actions"`
	ResourcePattern string          `json:"resource_pattern"`
	SubjectType     string          `json:"subject_type"`
	SubjectID       *string         `json:"subject_id,omitempty"`
	Conditions      json.RawMessage `json:"conditions,omitempty"`
	CreatedAt       string          `json:"created_at"`
}

// IAMPolicyRequest represents an IAM policy create/update request.
type IAMPolicyRequest struct {
	OrgID       string `json:"org_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PolicyType  string `json:"policy_type"`
	HCLSource   string `json:"hcl_source,omitempty"`
}

// IAMPolicyResponse represents the API response for an IAM policy.
type IAMPolicyResponse struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"org_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	PolicyType  string          `json:"policy_type"`
	PolicyDoc   json.RawMessage `json:"policy_doc"`
	HCLSource   string          `json:"hcl_source,omitempty"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("TeamVault API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// --- Secret Operations ---

// PutSecret creates or updates a secret.
func (c *TeamVaultClient) PutSecret(project, path string, req *SecretRequest) (*SecretResponse, error) {
	url := fmt.Sprintf("%s/api/v1/secrets/%s/%s", c.Address, project, path)
	var resp SecretResponse
	if err := c.doRequest("PUT", url, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSecret reads a secret.
func (c *TeamVaultClient) GetSecret(project, path string) (*SecretResponse, error) {
	url := fmt.Sprintf("%s/api/v1/secrets/%s/%s", c.Address, project, path)
	var resp SecretResponse
	if err := c.doRequest("GET", url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteSecret deletes a secret.
func (c *TeamVaultClient) DeleteSecret(project, path string) error {
	url := fmt.Sprintf("%s/api/v1/secrets/%s/%s", c.Address, project, path)
	return c.doRequest("DELETE", url, nil, nil)
}

// --- Project Operations ---

// CreateProject creates a new project.
func (c *TeamVaultClient) CreateProject(req *ProjectRequest) (*ProjectResponse, error) {
	url := fmt.Sprintf("%s/api/v1/projects", c.Address)
	var resp ProjectResponse
	if err := c.doRequest("POST", url, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListProjects lists all projects.
func (c *TeamVaultClient) ListProjects() ([]ProjectResponse, error) {
	url := fmt.Sprintf("%s/api/v1/projects", c.Address)
	var resp []ProjectResponse
	if err := c.doRequest("GET", url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetProjectByName finds a project by name from the list.
func (c *TeamVaultClient) GetProjectByName(name string) (*ProjectResponse, error) {
	projects, err := c.ListProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: fmt.Sprintf("project %q not found", name)}
}

// --- Policy Operations ---

// CreatePolicy creates a new access policy.
func (c *TeamVaultClient) CreatePolicy(req *PolicyRequest) (*PolicyResponse, error) {
	url := fmt.Sprintf("%s/api/v1/policies", c.Address)
	var resp PolicyResponse
	if err := c.doRequest("POST", url, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListPolicies lists all policies.
func (c *TeamVaultClient) ListPolicies() ([]PolicyResponse, error) {
	url := fmt.Sprintf("%s/api/v1/policies", c.Address)
	var resp []PolicyResponse
	if err := c.doRequest("GET", url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetPolicyByID finds a policy by ID from the list.
func (c *TeamVaultClient) GetPolicyByID(id string) (*PolicyResponse, error) {
	policies, err := c.ListPolicies()
	if err != nil {
		return nil, err
	}
	for _, p := range policies {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: fmt.Sprintf("policy %q not found", id)}
}

// --- IAM Policy Operations ---

// CreateIAMPolicy creates a new IAM policy.
func (c *TeamVaultClient) CreateIAMPolicy(req *IAMPolicyRequest) (*IAMPolicyResponse, error) {
	url := fmt.Sprintf("%s/api/v1/iam-policies", c.Address)
	var resp IAMPolicyResponse
	if err := c.doRequest("POST", url, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetIAMPolicy gets an IAM policy by ID.
func (c *TeamVaultClient) GetIAMPolicy(id string) (*IAMPolicyResponse, error) {
	url := fmt.Sprintf("%s/api/v1/iam-policies/%s", c.Address, id)
	var resp IAMPolicyResponse
	if err := c.doRequest("GET", url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateIAMPolicy updates an IAM policy.
func (c *TeamVaultClient) UpdateIAMPolicy(id string, req *IAMPolicyRequest) (*IAMPolicyResponse, error) {
	url := fmt.Sprintf("%s/api/v1/iam-policies/%s", c.Address, id)
	var resp IAMPolicyResponse
	if err := c.doRequest("PUT", url, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteIAMPolicy deletes an IAM policy.
func (c *TeamVaultClient) DeleteIAMPolicy(id string) error {
	url := fmt.Sprintf("%s/api/v1/iam-policies/%s", c.Address, id)
	return c.doRequest("DELETE", url, nil, nil)
}

// --- HTTP Helpers ---

// doRequest performs an HTTP request to the TeamVault API.
func (c *TeamVaultClient) doRequest(method, url string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		// Try to extract error message from JSON response
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &errResp) == nil {
			msg := errResp.Error
			if msg == "" {
				msg = errResp.Message
			}
			if msg == "" {
				msg = string(respBody)
			}
			return &APIError{StatusCode: resp.StatusCode, Message: msg}
		}
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// IsNotFound checks if an error is a 404 Not Found error.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 404
	}
	return false
}
