package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/spf13/cobra"
)

// --- Policy API types ---

// PolicyResponse represents a policy returned by the API.
type PolicyResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // rbac, abac, pbac
	Description string `json:"description"`
	HCLSource   string `json:"hcl_source"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	CreatedBy   string `json:"created_by"`
}

// --- API client methods ---

// ApplyPolicy sends an HCL policy to the server.
func (c *APIClient) ApplyPolicy(hclSource string) (*PolicyResponse, error) {
	var resp PolicyResponse
	err := c.do("POST", "/api/v1/iam-policies", map[string]string{
		"hcl_source": hclSource,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListPolicies returns all policies.
func (c *APIClient) ListPolicies() ([]PolicyResponse, error) {
	var resp struct {
		Policies []PolicyResponse `json:"policies"`
	}
	err := c.do("GET", "/api/v1/iam-policies", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Policies, nil
}

// InspectPolicy returns a single policy by name.
func (c *APIClient) InspectPolicy(name string) (*PolicyResponse, error) {
	var resp PolicyResponse
	err := c.do("GET", fmt.Sprintf("/api/v1/iam-policies?name=%s", name), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Cobra commands ---

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage IAM policies (policy-as-code)",
	Long: `Manage IAM policies using HCL policy-as-code.

Examples:
  teamvault policy apply -f policies/
  teamvault policy validate -f policy.hcl
  teamvault policy list
  teamvault policy inspect my-policy`,
}

var (
	policyApplyFile    string
	policyValidateFile string
)

var policyApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply HCL policy files to the server",
	Long: `Parse .hcl files and POST each policy to the TeamVault server.
Accepts a single file or a directory (all .hcl files will be applied).

Examples:
  teamvault policy apply -f policy.hcl
  teamvault policy apply -f policies/`,
	RunE: runPolicyApply,
}

var policyValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate HCL policy syntax locally",
	Long: `Parse an HCL file and check for syntax errors without sending to the server.

Examples:
  teamvault policy validate -f policy.hcl`,
	RunE: runPolicyValidate,
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all policies",
	Long: `Fetch and display all IAM policies from the server.

Examples:
  teamvault policy list`,
	RunE: runPolicyList,
}

var policyInspectCmd = &cobra.Command{
	Use:   "inspect NAME",
	Short: "Inspect a policy by name",
	Long: `Fetch full details of a policy by name and display as formatted JSON.

Examples:
  teamvault policy inspect my-rbac-policy`,
	Args: cobra.ExactArgs(1),
	RunE: runPolicyInspect,
}

func init() {
	policyApplyCmd.Flags().StringVarP(&policyApplyFile, "file", "f", "", "HCL file or directory to apply")
	policyApplyCmd.MarkFlagRequired("file")

	policyValidateCmd.Flags().StringVarP(&policyValidateFile, "file", "f", "", "HCL file to validate")
	policyValidateCmd.MarkFlagRequired("file")

	policyCmd.AddCommand(policyApplyCmd)
	policyCmd.AddCommand(policyValidateCmd)
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyInspectCmd)
}

// collectHCLFiles returns a list of .hcl file paths from a file or directory.
func collectHCLFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", path, err)
	}

	if !info.IsDir() {
		if !strings.HasSuffix(path, ".hcl") {
			return nil, fmt.Errorf("%s is not an .hcl file", path)
		}
		return []string{path}, nil
	}

	var files []string
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %s: %w", path, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".hcl") {
			files = append(files, filepath.Join(path, entry.Name()))
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .hcl files found in %s", path)
	}

	return files, nil
}

// validateHCLFile parses an HCL file and returns diagnostics.
func validateHCLFile(filePath string) ([]byte, hcl.Diagnostics) {
	parser := hclparse.NewParser()

	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   err.Error(),
			},
		}
	}

	_, diags := parser.ParseHCL(src, filePath)
	return src, diags
}

func runPolicyApply(cmd *cobra.Command, args []string) error {
	files, err := collectHCLFiles(policyApplyFile)
	if err != nil {
		return err
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	var errors int
	for _, file := range files {
		basename := filepath.Base(file)

		// Validate locally first
		src, diags := validateHCLFile(file)
		if diags.HasErrors() {
			fmt.Fprintf(os.Stderr, "✗ %s — syntax error:\n", basename)
			for _, d := range diags {
				if d.Severity == hcl.DiagError {
					fmt.Fprintf(os.Stderr, "    %s: %s\n", d.Summary, d.Detail)
				}
			}
			errors++
			continue
		}

		// POST to server
		resp, err := client.ApplyPolicy(string(src))
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s — %v\n", basename, err)
			errors++
			continue
		}

		fmt.Fprintf(os.Stderr, "✓ %s — applied policy %q (type: %s, id: %s)\n",
			basename, resp.Name, resp.Type, resp.ID)
	}

	if errors > 0 {
		return fmt.Errorf("%d of %d policies failed to apply", errors, len(files))
	}

	fmt.Fprintf(os.Stderr, "\n✓ All %d policies applied successfully\n", len(files))
	return nil
}

func runPolicyValidate(cmd *cobra.Command, args []string) error {
	info, err := os.Stat(policyValidateFile)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", policyValidateFile, err)
	}
	if info.IsDir() {
		return fmt.Errorf("validate expects a single file, not a directory (use apply for directories)")
	}

	_, diags := validateHCLFile(policyValidateFile)

	if diags.HasErrors() {
		fmt.Fprintf(os.Stderr, "✗ %s — validation failed:\n", policyValidateFile)
		for _, d := range diags {
			severity := "error"
			if d.Severity == hcl.DiagWarning {
				severity = "warning"
			}
			loc := ""
			if d.Subject != nil {
				loc = fmt.Sprintf(" (line %d, col %d)", d.Subject.Start.Line, d.Subject.Start.Column)
			}
			fmt.Fprintf(os.Stderr, "  [%s]%s %s", severity, loc, d.Summary)
			if d.Detail != "" {
				fmt.Fprintf(os.Stderr, ": %s", d.Detail)
			}
			fmt.Fprintln(os.Stderr)
		}
		return fmt.Errorf("validation failed")
	}

	// Print warnings if any
	for _, d := range diags {
		if d.Severity == hcl.DiagWarning {
			loc := ""
			if d.Subject != nil {
				loc = fmt.Sprintf(" (line %d, col %d)", d.Subject.Start.Line, d.Subject.Start.Column)
			}
			fmt.Fprintf(os.Stderr, "  [warning]%s %s: %s\n", loc, d.Summary, d.Detail)
		}
	}

	fmt.Fprintf(os.Stderr, "✓ %s — valid HCL syntax\n", policyValidateFile)
	return nil
}

func runPolicyList(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	policies, err := client.ListPolicies()
	if err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}

	if len(policies) == 0 {
		fmt.Fprintf(os.Stderr, "No policies found\n")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tDESCRIPTION\tCREATED")
	for _, p := range policies {
		created := p.CreatedAt
		if len(created) > 19 {
			created = created[:19]
		}
		desc := p.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Type, desc, created)
	}
	w.Flush()

	return nil
}

func runPolicyInspect(cmd *cobra.Command, args []string) error {
	name := args[0]

	client, err := NewClient()
	if err != nil {
		return err
	}

	policy, err := client.InspectPolicy(name)
	if err != nil {
		return fmt.Errorf("failed to inspect policy %q: %w", name, err)
	}

	// If HCL source is available, print it
	if policy.HCLSource != "" {
		fmt.Fprintf(os.Stderr, "# Policy: %s (type: %s)\n", policy.Name, policy.Type)
		fmt.Fprintf(os.Stderr, "# ID: %s\n", policy.ID)
		fmt.Fprintf(os.Stderr, "# Created: %s by %s\n\n", policy.CreatedAt, policy.CreatedBy)
		fmt.Println(policy.HCLSource)
		return nil
	}

	// Fallback to formatted JSON
	out, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format policy: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
