package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// OIDCClient handles OpenID Connect authentication flows.
type OIDCClient struct {
	config   OIDCConfig
	provider *OIDCProvider
	mu       sync.RWMutex
	states   map[string]time.Time // state -> expiry (CSRF protection)
}

// OIDCProvider holds discovered OIDC provider endpoints.
type OIDCProvider struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
	Issuer                string `json:"issuer"`
}

// OIDCTokenResponse is the response from the token endpoint.
type OIDCTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// OIDCUserInfo is the response from the userinfo endpoint.
type OIDCUserInfo struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`
}

// IsConfigured returns true if all required OIDC env vars are set.
func (c *OIDCConfig) IsConfigured() bool {
	return c.Issuer != "" && c.ClientID != "" && c.ClientSecret != "" && c.RedirectURI != ""
}

// NewOIDCClient creates a new OIDC client. Returns nil if not configured.
func NewOIDCClient(config OIDCConfig) *OIDCClient {
	if !config.IsConfigured() {
		return nil
	}
	return &OIDCClient{
		config: config,
		states: make(map[string]time.Time),
	}
}

// Discover fetches the OIDC provider's well-known configuration.
func (c *OIDCClient) Discover(ctx context.Context) error {
	wellKnownURL := strings.TrimSuffix(c.config.Issuer, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, "GET", wellKnownURL, nil)
	if err != nil {
		return fmt.Errorf("creating discovery request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching OIDC discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	var provider OIDCProvider
	if err := json.NewDecoder(resp.Body).Decode(&provider); err != nil {
		return fmt.Errorf("decoding OIDC discovery: %w", err)
	}

	c.mu.Lock()
	c.provider = &provider
	c.mu.Unlock()

	return nil
}

// GetAuthorizationURL returns the URL to redirect the user to for OIDC login.
func (c *OIDCClient) GetAuthorizationURL() (string, string, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return "", "", fmt.Errorf("OIDC provider not discovered yet")
	}

	// Generate state token for CSRF protection
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", fmt.Errorf("generating state: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// Store state with expiry
	c.mu.Lock()
	c.states[state] = time.Now().Add(10 * time.Minute)
	// Clean expired states
	for k, v := range c.states {
		if time.Now().After(v) {
			delete(c.states, k)
		}
	}
	c.mu.Unlock()

	params := url.Values{
		"client_id":     {c.config.ClientID},
		"redirect_uri":  {c.config.RedirectURI},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
	}

	authURL := provider.AuthorizationEndpoint + "?" + params.Encode()
	return authURL, state, nil
}

// ValidateState checks if a state parameter is valid and removes it.
func (c *OIDCClient) ValidateState(state string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiry, ok := c.states[state]
	if !ok {
		return false
	}
	delete(c.states, state)
	return time.Now().Before(expiry)
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *OIDCClient) ExchangeCode(ctx context.Context, code string) (*OIDCTokenResponse, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("OIDC provider not discovered yet")
	}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.config.RedirectURI},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", provider.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp OIDCTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &tokenResp, nil
}

// GetUserInfo fetches user information from the userinfo endpoint.
func (c *OIDCClient) GetUserInfo(ctx context.Context, accessToken string) (*OIDCUserInfo, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("OIDC provider not discovered yet")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", provider.UserinfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo returned status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo OIDCUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("decoding userinfo: %w", err)
	}

	return &userInfo, nil
}

// Issuer returns the OIDC issuer URL.
func (c *OIDCClient) Issuer() string {
	return c.config.Issuer
}
