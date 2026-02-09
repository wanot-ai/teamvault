package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// --- Doctor types ---

// HealthResponse represents the server health check API response.
type HealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Uptime    string                 `json:"uptime"`
	Database  HealthComponentStatus  `json:"database"`
	Cache     *HealthComponentStatus `json:"cache,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// HealthComponentStatus represents the status of an individual component.
type HealthComponentStatus struct {
	Status  string `json:"status"` // healthy, degraded, unhealthy
	Latency string `json:"latency,omitempty"`
	Message string `json:"message,omitempty"`
}

// --- API client methods ---

// Health queries the server health endpoint (no auth required).
func (c *APIClient) Health() (*HealthResponse, error) {
	var resp HealthResponse
	err := c.do("GET", "/api/v1/health", nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// WhoAmI returns the current user identity from the token.
func (c *APIClient) WhoAmI() (map[string]interface{}, error) {
	var resp map[string]interface{}
	err := c.do("GET", "/api/v1/auth/me", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// --- Cobra commands ---

var (
	doctorServer string
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks against TeamVault server",
	Long: `Run a series of diagnostic checks against a TeamVault server:

  1. Server reachability — can we connect at all?
  2. Server health — is the /health endpoint reporting OK?
  3. Database — is the backing datastore healthy?
  4. Authentication — is the stored token valid?
  5. Token expiry — is the token close to expiration?

Use this command to diagnose connectivity or configuration issues.

Examples:
  teamvault doctor
  teamvault doctor --server https://vault.example.com`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().StringVar(&doctorServer, "server", "", "TeamVault server URL (defaults to stored config)")
}

type checkResult struct {
	name   string
	ok     bool
	detail string
	warn   bool
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// Determine server URL
	server := doctorServer
	if server == "" {
		cfg, err := LoadConfig()
		if err == nil && cfg.Server != "" {
			server = cfg.Server
		}
		if server == "" {
			tokenData, err := LoadToken()
			if err == nil && tokenData.Server != "" {
				server = tokenData.Server
			}
		}
	}

	if server == "" {
		return fmt.Errorf("no server specified. Use --server flag or run 'teamvault login' first")
	}

	server = strings.TrimRight(server, "/")
	fmt.Fprintf(os.Stderr, "Running health checks against %s\n\n", server)

	var checks []checkResult

	// Check 1: Server reachability
	checks = append(checks, checkServerReachable(server))

	// Check 2: Server health endpoint
	healthResp, healthCheck := checkServerHealth(server)
	checks = append(checks, healthCheck)

	// Check 3: Database health (from health response)
	if healthResp != nil {
		checks = append(checks, checkDatabaseHealth(healthResp))
	} else {
		checks = append(checks, checkResult{
			name:   "Database",
			ok:     false,
			detail: "could not determine (health endpoint unreachable)",
		})
	}

	// Check 4: Authentication
	tokenData, authCheck := checkAuthentication(server)
	checks = append(checks, authCheck)

	// Check 5: Token expiry
	if tokenData.Token != "" {
		checks = append(checks, checkTokenExpiry(tokenData.Token))
	} else {
		checks = append(checks, checkResult{
			name:   "Token Expiry",
			ok:     false,
			detail: "no token available",
		})
	}

	// Print results
	fmt.Println()
	allOK := true
	hasWarnings := false
	for _, c := range checks {
		icon := "✓"
		if !c.ok && !c.warn {
			icon = "✗"
			allOK = false
		} else if c.warn {
			icon = "⚠"
			hasWarnings = true
		}
		fmt.Fprintf(os.Stdout, "  %s  %-20s %s\n", icon, c.name, c.detail)
	}

	fmt.Println()
	if allOK && !hasWarnings {
		fmt.Fprintf(os.Stderr, "All checks passed ✓\n")
	} else if allOK && hasWarnings {
		fmt.Fprintf(os.Stderr, "Checks passed with warnings ⚠\n")
	} else {
		fmt.Fprintf(os.Stderr, "Some checks failed ✗\n")
		return fmt.Errorf("health check failed")
	}

	return nil
}

func checkServerReachable(server string) checkResult {
	client := &http.Client{Timeout: 10 * time.Second}

	start := time.Now()
	resp, err := client.Get(server + "/api/v1/health")
	elapsed := time.Since(start)

	if err != nil {
		return checkResult{
			name:   "Reachability",
			ok:     false,
			detail: fmt.Sprintf("cannot connect: %v", err),
		}
	}
	resp.Body.Close()

	return checkResult{
		name:   "Reachability",
		ok:     true,
		detail: fmt.Sprintf("connected (%dms)", elapsed.Milliseconds()),
	}
}

func checkServerHealth(server string) (*HealthResponse, checkResult) {
	client := NewClientWithURL(server)
	health, err := client.Health()
	if err != nil {
		return nil, checkResult{
			name:   "Server Health",
			ok:     false,
			detail: fmt.Sprintf("health endpoint error: %v", err),
		}
	}

	detail := fmt.Sprintf("status=%s", health.Status)
	if health.Version != "" {
		detail += fmt.Sprintf(", version=%s", health.Version)
	}
	if health.Uptime != "" {
		detail += fmt.Sprintf(", uptime=%s", health.Uptime)
	}

	ok := health.Status == "ok" || health.Status == "healthy"
	return health, checkResult{
		name:   "Server Health",
		ok:     ok,
		detail: detail,
	}
}

func checkDatabaseHealth(health *HealthResponse) checkResult {
	if health.Database.Status == "" {
		return checkResult{
			name:   "Database",
			ok:     false,
			detail: "no database status reported",
		}
	}

	detail := fmt.Sprintf("status=%s", health.Database.Status)
	if health.Database.Latency != "" {
		detail += fmt.Sprintf(", latency=%s", health.Database.Latency)
	}
	if health.Database.Message != "" {
		detail += fmt.Sprintf(", %s", health.Database.Message)
	}

	ok := health.Database.Status == "healthy" || health.Database.Status == "ok"
	warn := health.Database.Status == "degraded"

	return checkResult{
		name:   "Database",
		ok:     ok || warn,
		detail: detail,
		warn:   warn,
	}
}

func checkAuthentication(server string) (TokenData, checkResult) {
	tokenData, err := LoadToken()
	if err != nil {
		return TokenData{}, checkResult{
			name:   "Authentication",
			ok:     false,
			detail: "not logged in (run 'teamvault login')",
		}
	}

	// Verify the token is for the right server
	if strings.TrimRight(tokenData.Server, "/") != strings.TrimRight(server, "/") {
		return tokenData, checkResult{
			name:   "Authentication",
			ok:     false,
			warn:   true,
			detail: fmt.Sprintf("token is for %s, not %s", tokenData.Server, server),
		}
	}

	// Try to validate the token with the server
	client := &APIClient{
		BaseURL: strings.TrimRight(server, "/"),
		Token:   tokenData.Token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	me, err := client.WhoAmI()
	if err != nil {
		return tokenData, checkResult{
			name:   "Authentication",
			ok:     false,
			detail: fmt.Sprintf("token invalid: %v", err),
		}
	}

	email, _ := me["email"].(string)
	detail := "token valid"
	if email != "" {
		detail = fmt.Sprintf("authenticated as %s", email)
	}

	return tokenData, checkResult{
		name:   "Authentication",
		ok:     true,
		detail: detail,
	}
}

func checkTokenExpiry(token string) checkResult {
	// Attempt to decode JWT claims (second segment) without verifying signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return checkResult{
			name:   "Token Expiry",
			ok:     true,
			warn:   true,
			detail: "token is not JWT format (cannot check expiry)",
		}
	}

	// Decode the payload
	payload := parts[1]
	// Add padding if needed
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return checkResult{
			name:   "Token Expiry",
			ok:     true,
			warn:   true,
			detail: "could not decode token payload",
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return checkResult{
			name:   "Token Expiry",
			ok:     true,
			warn:   true,
			detail: "could not parse token claims",
		}
	}

	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return checkResult{
			name:   "Token Expiry",
			ok:     true,
			detail: "token has no expiry (non-expiring token)",
		}
	}

	expTime := time.Unix(int64(expFloat), 0)
	now := time.Now()

	if now.After(expTime) {
		return checkResult{
			name:   "Token Expiry",
			ok:     false,
			detail: fmt.Sprintf("token expired at %s (re-run 'teamvault login')", expTime.Format(time.RFC3339)),
		}
	}

	remaining := expTime.Sub(now)
	detail := fmt.Sprintf("expires %s (in %s)", expTime.Format(time.RFC3339), formatDuration(remaining))

	// Warn if less than 24 hours
	if remaining < 24*time.Hour {
		return checkResult{
			name:   "Token Expiry",
			ok:     true,
			warn:   true,
			detail: detail + " — consider refreshing",
		}
	}

	return checkResult{
		name:   "Token Expiry",
		ok:     true,
		detail: detail,
	}
}

// formatDuration renders a time.Duration in human-friendly form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) - hours*60
		if mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) - days*24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}
