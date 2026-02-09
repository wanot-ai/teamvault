package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// --- Lease API types ---

// LeaseResponse represents a dynamic lease issued by TeamVault.
type LeaseResponse struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Project     string            `json:"project"`
	TTL         string            `json:"ttl"`
	ExpiresAt   string            `json:"expires_at"`
	CreatedAt   string            `json:"created_at"`
	CreatedBy   string            `json:"created_by"`
	Renewable   bool              `json:"renewable"`
	Credentials map[string]string `json:"credentials,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// LeaseListItem represents a lease in list output (no credentials exposed).
type LeaseListItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Project   string `json:"project"`
	TTL       string `json:"ttl"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	Renewable bool   `json:"renewable"`
	Status    string `json:"status"` // active, expired, revoked
}

// --- API client methods ---

// IssueLease requests a new dynamic lease from the server.
func (c *APIClient) IssueLease(leaseType, project, ttl, role string) (*LeaseResponse, error) {
	var resp LeaseResponse
	body := map[string]string{
		"type":    leaseType,
		"project": project,
		"ttl":     ttl,
	}
	if role != "" {
		body["role"] = role
	}
	err := c.do("POST", "/api/v1/leases", body, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListLeases lists all active leases, optionally filtered by project.
func (c *APIClient) ListLeases(project string) ([]LeaseListItem, error) {
	var resp struct {
		Leases []LeaseListItem `json:"leases"`
	}
	apiPath := "/api/v1/leases"
	if project != "" {
		apiPath = fmt.Sprintf("/api/v1/leases?project=%s", project)
	}
	err := c.do("GET", apiPath, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Leases, nil
}

// RevokeLease revokes an active lease by ID.
func (c *APIClient) RevokeLease(leaseID string) error {
	return c.do("DELETE", fmt.Sprintf("/api/v1/leases/%s", leaseID), nil, nil)
}

// RenewLease extends the TTL of an active lease.
func (c *APIClient) RenewLease(leaseID, ttl string) (*LeaseResponse, error) {
	var resp LeaseResponse
	body := map[string]string{}
	if ttl != "" {
		body["ttl"] = ttl
	}
	err := c.do("POST", fmt.Sprintf("/api/v1/leases/%s/renew", leaseID), body, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetLease retrieves details about a specific lease.
func (c *APIClient) GetLease(leaseID string) (*LeaseResponse, error) {
	var resp LeaseResponse
	err := c.do("GET", fmt.Sprintf("/api/v1/leases/%s", leaseID), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Cobra commands ---

var leaseCmd = &cobra.Command{
	Use:   "lease",
	Short: "Manage dynamic leases",
	Long: `Issue, list, renew, and revoke dynamic leases for short-lived credentials.

Leases provide temporary credentials (database users, cloud tokens, etc.)
that are automatically revoked when the TTL expires.

Supported lease types: database, aws, gcp, azure, redis, custom

Examples:
  teamvault lease issue --type database --ttl 15m --project myproject
  teamvault lease list
  teamvault lease list --project myproject
  teamvault lease inspect LEASE_ID
  teamvault lease renew LEASE_ID --ttl 30m
  teamvault lease revoke LEASE_ID`,
}

var (
	leaseIssueType    string
	leaseIssueTTL     string
	leaseIssueProject string
	leaseIssueRole    string
	leaseListProject  string
	leaseRenewTTL     string
)

var leaseIssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Issue a new dynamic lease",
	Long: `Issue a new dynamic lease. The server provisions short-lived credentials
based on the lease type and returns them. Credentials are displayed once
and cannot be retrieved again.

Types:
  database   Temporary database user with scoped permissions
  aws        Short-lived AWS STS credentials
  gcp        GCP service account key or OAuth token
  azure      Azure AD token
  redis      Temporary Redis AUTH credentials
  custom     Custom connector-defined credentials

Examples:
  teamvault lease issue --type database --ttl 15m --project myproject
  teamvault lease issue --type aws --ttl 1h --project myproject --role readonly
  teamvault lease issue --type database --ttl 30m --project myproject --role readwrite`,
	RunE: runLeaseIssue,
}

var leaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active leases",
	Long: `List all active leases. Expired and revoked leases are included in output
with their status.

Examples:
  teamvault lease list
  teamvault lease list --project myproject`,
	RunE: runLeaseList,
}

var leaseRevokeCmd = &cobra.Command{
	Use:   "revoke LEASE_ID",
	Short: "Revoke an active lease",
	Long: `Revoke a lease immediately. The associated credentials are invalidated
and the underlying resource (e.g. database user) is cleaned up.

Examples:
  teamvault lease revoke lease_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runLeaseRevoke,
}

var leaseInspectCmd = &cobra.Command{
	Use:   "inspect LEASE_ID",
	Short: "Show details of a lease",
	Long: `Display detailed information about a lease including metadata and status.
Credentials are not shown (they were displayed only at issuance time).

Examples:
  teamvault lease inspect lease_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runLeaseInspect,
}

var leaseRenewCmd = &cobra.Command{
	Use:   "renew LEASE_ID",
	Short: "Renew an active lease",
	Long: `Extend the TTL of an active, renewable lease. The new TTL is added
relative to the current time.

Examples:
  teamvault lease renew lease_abc123
  teamvault lease renew lease_abc123 --ttl 30m`,
	Args: cobra.ExactArgs(1),
	RunE: runLeaseRenew,
}

func init() {
	leaseIssueCmd.Flags().StringVar(&leaseIssueType, "type", "", "Lease type (database, aws, gcp, azure, redis, custom)")
	leaseIssueCmd.Flags().StringVar(&leaseIssueTTL, "ttl", "15m", "Lease time-to-live (e.g. 15m, 1h, 24h)")
	leaseIssueCmd.Flags().StringVar(&leaseIssueProject, "project", "", "Project scope for the lease")
	leaseIssueCmd.Flags().StringVar(&leaseIssueRole, "role", "", "Role or permission level (connector-specific)")
	leaseIssueCmd.MarkFlagRequired("type")
	leaseIssueCmd.MarkFlagRequired("project")

	leaseListCmd.Flags().StringVar(&leaseListProject, "project", "", "Filter by project name")

	leaseRenewCmd.Flags().StringVar(&leaseRenewTTL, "ttl", "", "New TTL to extend (e.g. 30m, 1h)")

	leaseCmd.AddCommand(leaseIssueCmd)
	leaseCmd.AddCommand(leaseListCmd)
	leaseCmd.AddCommand(leaseRevokeCmd)
	leaseCmd.AddCommand(leaseInspectCmd)
	leaseCmd.AddCommand(leaseRenewCmd)
}

func runLeaseIssue(cmd *cobra.Command, args []string) error {
	// Validate lease type
	validTypes := map[string]bool{
		"database": true,
		"aws":      true,
		"gcp":      true,
		"azure":    true,
		"redis":    true,
		"custom":   true,
	}
	if !validTypes[leaseIssueType] {
		return fmt.Errorf("unsupported lease type %q. Valid types: database, aws, gcp, azure, redis, custom", leaseIssueType)
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Issuing %s lease for project %s (ttl: %s)...\n", leaseIssueType, leaseIssueProject, leaseIssueTTL)

	lease, err := client.IssueLease(leaseIssueType, leaseIssueProject, leaseIssueTTL, leaseIssueRole)
	if err != nil {
		return fmt.Errorf("failed to issue lease: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Lease issued\n")
	fmt.Fprintf(os.Stderr, "  ID:        %s\n", lease.ID)
	fmt.Fprintf(os.Stderr, "  Type:      %s\n", lease.Type)
	fmt.Fprintf(os.Stderr, "  Project:   %s\n", lease.Project)
	fmt.Fprintf(os.Stderr, "  TTL:       %s\n", lease.TTL)
	fmt.Fprintf(os.Stderr, "  Expires:   %s\n", lease.ExpiresAt)
	fmt.Fprintf(os.Stderr, "  Renewable: %v\n", lease.Renewable)

	if len(lease.Credentials) > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠  Credentials (shown once only):\n")
		for k, v := range lease.Credentials {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", k, v)
		}
	}

	// Output full lease as JSON to stdout for scripting
	out, _ := json.MarshalIndent(lease, "", "  ")
	fmt.Println(string(out))

	return nil
}

func runLeaseList(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	leases, err := client.ListLeases(leaseListProject)
	if err != nil {
		return fmt.Errorf("failed to list leases: %w", err)
	}

	if len(leases) == 0 {
		fmt.Fprintf(os.Stderr, "No leases found\n")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tPROJECT\tTTL\tSTATUS\tEXPIRES")
	for _, l := range leases {
		expires := l.ExpiresAt
		if len(expires) > 19 {
			expires = expires[:19]
		}
		status := l.Status
		if status == "" {
			status = "active"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", l.ID, l.Type, l.Project, l.TTL, status, expires)
	}
	w.Flush()

	return nil
}

func runLeaseRevoke(cmd *cobra.Command, args []string) error {
	leaseID := args[0]

	client, err := NewClient()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Revoking lease %s...\n", leaseID)

	if err := client.RevokeLease(leaseID); err != nil {
		return fmt.Errorf("failed to revoke lease %s: %w", leaseID, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Lease %s revoked\n", leaseID)
	return nil
}

func runLeaseInspect(cmd *cobra.Command, args []string) error {
	leaseID := args[0]

	client, err := NewClient()
	if err != nil {
		return err
	}

	lease, err := client.GetLease(leaseID)
	if err != nil {
		return fmt.Errorf("failed to inspect lease %s: %w", leaseID, err)
	}

	fmt.Fprintf(os.Stdout, "ID:        %s\n", lease.ID)
	fmt.Fprintf(os.Stdout, "Type:      %s\n", lease.Type)
	fmt.Fprintf(os.Stdout, "Project:   %s\n", lease.Project)
	fmt.Fprintf(os.Stdout, "TTL:       %s\n", lease.TTL)
	fmt.Fprintf(os.Stdout, "Expires:   %s\n", lease.ExpiresAt)
	fmt.Fprintf(os.Stdout, "Created:   %s\n", lease.CreatedAt)
	fmt.Fprintf(os.Stdout, "CreatedBy: %s\n", lease.CreatedBy)
	fmt.Fprintf(os.Stdout, "Renewable: %v\n", lease.Renewable)

	if len(lease.Metadata) > 0 {
		fmt.Fprintf(os.Stdout, "\nMetadata:\n")
		for k, v := range lease.Metadata {
			fmt.Fprintf(os.Stdout, "  %s: %s\n", k, v)
		}
	}

	return nil
}

func runLeaseRenew(cmd *cobra.Command, args []string) error {
	leaseID := args[0]

	client, err := NewClient()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Renewing lease %s...\n", leaseID)

	lease, err := client.RenewLease(leaseID, leaseRenewTTL)
	if err != nil {
		return fmt.Errorf("failed to renew lease %s: %w", leaseID, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Lease renewed\n")
	fmt.Fprintf(os.Stderr, "  ID:      %s\n", lease.ID)
	fmt.Fprintf(os.Stderr, "  TTL:     %s\n", lease.TTL)
	fmt.Fprintf(os.Stderr, "  Expires: %s\n", lease.ExpiresAt)

	return nil
}
