package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// --- Rotation API types ---

// RotationPolicy represents a secret rotation policy.
type RotationPolicy struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Path      string `json:"path"`
	Cron      string `json:"cron"`
	Connector string `json:"connector"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// RotationStatus represents the current rotation status for a secret.
type RotationStatus struct {
	Project       string `json:"project"`
	Path          string `json:"path"`
	Cron          string `json:"cron"`
	Connector     string `json:"connector"`
	Enabled       bool   `json:"enabled"`
	LastRotatedAt string `json:"last_rotated_at"`
	NextRotation  string `json:"next_rotation"`
	LastError     string `json:"last_error,omitempty"`
	Version       int    `json:"version"`
}

// RotationResult represents the result of a manual rotation trigger.
type RotationResult struct {
	Project    string `json:"project"`
	Path       string `json:"path"`
	NewVersion int    `json:"new_version"`
	RotatedAt  string `json:"rotated_at"`
	Connector  string `json:"connector"`
}

// --- API client methods ---

// SetRotationPolicy creates or updates a rotation policy for a secret.
func (c *APIClient) SetRotationPolicy(project, path, cron, connector string) (*RotationPolicy, error) {
	var resp RotationPolicy
	err := c.do("PUT", fmt.Sprintf("/api/v1/secrets/%s/%s/rotation", project, path), map[string]interface{}{
		"cron":      cron,
		"connector": connector,
		"enabled":   true,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRotationStatus retrieves the rotation status for a secret.
func (c *APIClient) GetRotationStatus(project, path string) (*RotationStatus, error) {
	var resp RotationStatus
	err := c.do("GET", fmt.Sprintf("/api/v1/secrets/%s/%s/rotation", project, path), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// TriggerRotation manually triggers rotation for a secret.
func (c *APIClient) TriggerRotation(project, path string) (*RotationResult, error) {
	var resp RotationResult
	err := c.do("POST", fmt.Sprintf("/api/v1/secrets/%s/%s/rotate", project, path), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListRotationPolicies lists all rotation policies, optionally filtered by project.
func (c *APIClient) ListRotationPolicies(project string) ([]RotationPolicy, error) {
	var resp struct {
		Policies []RotationPolicy `json:"policies"`
	}
	apiPath := "/api/v1/rotation-policies"
	if project != "" {
		apiPath = fmt.Sprintf("/api/v1/rotation-policies?project=%s", project)
	}
	err := c.do("GET", apiPath, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Policies, nil
}

// DeleteRotationPolicy removes a rotation policy for a secret.
func (c *APIClient) DeleteRotationPolicy(project, path string) error {
	return c.do("DELETE", fmt.Sprintf("/api/v1/secrets/%s/%s/rotation", project, path), nil, nil)
}

// --- Cobra commands ---

var rotationCmd = &cobra.Command{
	Use:   "rotation",
	Short: "Manage secret rotation policies",
	Long: `Configure and manage automatic secret rotation policies.

Rotation policies define a cron schedule and a connector that generates
the new secret value. Connectors include: random_password, aws_iam_key,
database_password, api_key, certificate, etc.

Examples:
  teamvault rotation set myproject/db/password --cron "0 0 * * MON" --connector random_password
  teamvault rotation status myproject/db/password
  teamvault rotation list --project myproject
  teamvault rotation delete myproject/db/password`,
}

var (
	rotationSetCron      string
	rotationSetConnector string
	rotationListProject  string
)

var rotationSetCmd = &cobra.Command{
	Use:   "set PROJECT/PATH",
	Short: "Create or update a rotation policy for a secret",
	Long: `Set a rotation policy for a secret. The secret will be automatically
rotated on the specified cron schedule using the given connector.

Connectors:
  random_password    Generate a random password (configurable length/charset)
  aws_iam_key        Rotate AWS IAM access keys
  database_password  Rotate database user passwords
  api_key            Regenerate API keys
  certificate        Renew TLS certificates

Examples:
  teamvault rotation set myproject/db/password --cron "0 0 * * MON" --connector random_password
  teamvault rotation set myproject/aws/key --cron "0 0 1 * *" --connector aws_iam_key`,
	Args: cobra.ExactArgs(1),
	RunE: runRotationSet,
}

var rotationStatusCmd = &cobra.Command{
	Use:   "status PROJECT/PATH",
	Short: "Show rotation status for a secret",
	Long: `Display the current rotation policy and status for a secret, including
the cron schedule, last rotation time, next scheduled rotation, and any errors.

Examples:
  teamvault rotation status myproject/db/password`,
	Args: cobra.ExactArgs(1),
	RunE: runRotationStatus,
}

var rotationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all rotation policies",
	Long: `List all rotation policies, optionally filtered by project.

Examples:
  teamvault rotation list
  teamvault rotation list --project myproject`,
	RunE: runRotationList,
}

var rotationDeleteCmd = &cobra.Command{
	Use:   "delete PROJECT/PATH",
	Short: "Delete a rotation policy",
	Long: `Remove a rotation policy from a secret. The secret itself is not deleted,
only the automatic rotation schedule is removed.

Examples:
  teamvault rotation delete myproject/db/password`,
	Args: cobra.ExactArgs(1),
	RunE: runRotationDelete,
}

// rotateCmd is a top-level convenience command for manual rotation.
var rotateCmd = &cobra.Command{
	Use:   "rotate PROJECT/PATH",
	Short: "Manually trigger secret rotation",
	Long: `Manually trigger an immediate rotation for a secret. The secret must have
a rotation policy configured (use 'teamvault rotation set' first).

The connector specified in the rotation policy will be used to generate
the new secret value.

Examples:
  teamvault rotate myproject/db/password
  teamvault rotate myproject/aws/access-key`,
	Args: cobra.ExactArgs(1),
	RunE: runRotate,
}

func init() {
	rotationSetCmd.Flags().StringVar(&rotationSetCron, "cron", "", "Cron schedule expression (e.g. \"0 0 * * MON\")")
	rotationSetCmd.Flags().StringVar(&rotationSetConnector, "connector", "", "Rotation connector (e.g. random_password, aws_iam_key)")
	rotationSetCmd.MarkFlagRequired("cron")
	rotationSetCmd.MarkFlagRequired("connector")

	rotationListCmd.Flags().StringVar(&rotationListProject, "project", "", "Filter by project name")

	rotationCmd.AddCommand(rotationSetCmd)
	rotationCmd.AddCommand(rotationStatusCmd)
	rotationCmd.AddCommand(rotationListCmd)
	rotationCmd.AddCommand(rotationDeleteCmd)
}

func runRotationSet(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	// Validate connector name
	validConnectors := map[string]bool{
		"random_password":   true,
		"aws_iam_key":       true,
		"database_password": true,
		"api_key":           true,
		"certificate":       true,
	}
	if !validConnectors[rotationSetConnector] {
		fmt.Fprintf(os.Stderr, "Warning: %q is not a built-in connector. Proceeding anyway.\n", rotationSetConnector)
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	policy, err := client.SetRotationPolicy(project, path, rotationSetCron, rotationSetConnector)
	if err != nil {
		return fmt.Errorf("failed to set rotation policy for %s/%s: %w", project, path, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Rotation policy set for %s/%s\n", project, path)
	fmt.Fprintf(os.Stderr, "  Cron:      %s\n", policy.Cron)
	fmt.Fprintf(os.Stderr, "  Connector: %s\n", policy.Connector)
	fmt.Fprintf(os.Stderr, "  Enabled:   %v\n", policy.Enabled)

	return nil
}

func runRotationStatus(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	status, err := client.GetRotationStatus(project, path)
	if err != nil {
		return fmt.Errorf("failed to get rotation status for %s/%s: %w", project, path, err)
	}

	fmt.Fprintf(os.Stdout, "Secret:          %s/%s\n", status.Project, status.Path)
	fmt.Fprintf(os.Stdout, "Cron:            %s\n", status.Cron)
	fmt.Fprintf(os.Stdout, "Connector:       %s\n", status.Connector)
	fmt.Fprintf(os.Stdout, "Enabled:         %v\n", status.Enabled)
	fmt.Fprintf(os.Stdout, "Current Version: %d\n", status.Version)

	if status.LastRotatedAt != "" {
		fmt.Fprintf(os.Stdout, "Last Rotated:    %s\n", status.LastRotatedAt)
	} else {
		fmt.Fprintf(os.Stdout, "Last Rotated:    never\n")
	}

	if status.NextRotation != "" {
		fmt.Fprintf(os.Stdout, "Next Rotation:   %s\n", status.NextRotation)
	}

	if status.LastError != "" {
		fmt.Fprintf(os.Stdout, "Last Error:      %s\n", status.LastError)
	}

	return nil
}

func runRotationList(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	policies, err := client.ListRotationPolicies(rotationListProject)
	if err != nil {
		return fmt.Errorf("failed to list rotation policies: %w", err)
	}

	if len(policies) == 0 {
		fmt.Fprintf(os.Stderr, "No rotation policies found\n")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tPATH\tCRON\tCONNECTOR\tENABLED\tUPDATED")
	for _, p := range policies {
		updated := p.UpdatedAt
		if len(updated) > 19 {
			updated = updated[:19]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\t%s\n", p.Project, p.Path, p.Cron, p.Connector, p.Enabled, updated)
	}
	w.Flush()

	return nil
}

func runRotationDelete(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	if err := client.DeleteRotationPolicy(project, path); err != nil {
		return fmt.Errorf("failed to delete rotation policy for %s/%s: %w", project, path, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Rotation policy deleted for %s/%s\n", project, path)
	return nil
}

func runRotate(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Triggering rotation for %s/%s...\n", project, path)

	result, err := client.TriggerRotation(project, path)
	if err != nil {
		return fmt.Errorf("failed to rotate %s/%s: %w", project, path, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Secret rotated successfully\n")
	fmt.Fprintf(os.Stderr, "  New Version: %d\n", result.NewVersion)
	fmt.Fprintf(os.Stderr, "  Connector:   %s\n", result.Connector)
	fmt.Fprintf(os.Stderr, "  Rotated At:  %s\n", result.RotatedAt)

	// Output JSON for scripting
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))

	return nil
}
