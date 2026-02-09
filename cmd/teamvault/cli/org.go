package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// --- Org API types ---

// OrgResponse represents an organization returned by the API.
type OrgResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	MemberCount int    `json:"member_count"`
}

// --- API client methods ---

// CreateOrg creates a new organization.
func (c *APIClient) CreateOrg(name, displayName, description string) (*OrgResponse, error) {
	var resp OrgResponse
	body := map[string]string{
		"name": name,
	}
	if displayName != "" {
		body["display_name"] = displayName
	}
	if description != "" {
		body["description"] = description
	}
	err := c.do("POST", "/api/v1/orgs", body, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListOrgs returns all organizations the user has access to.
func (c *APIClient) ListOrgs() ([]OrgResponse, error) {
	var resp struct {
		Orgs []OrgResponse `json:"orgs"`
	}
	err := c.do("GET", "/api/v1/orgs", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Orgs, nil
}

// --- Cobra commands ---

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations",
	Long: `Create and manage organizations in TeamVault.

Examples:
  teamvault org create --name my-org --display-name "My Organization"
  teamvault org list`,
}

var (
	orgCreateName        string
	orgCreateDisplayName string
	orgCreateDescription string
)

var orgCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new organization",
	Long: `Create a new organization in TeamVault.

Examples:
  teamvault org create --name my-org
  teamvault org create --name my-org --display-name "My Organization" --description "Main org"`,
	RunE: runOrgCreate,
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations",
	Long: `List all organizations you have access to.

Examples:
  teamvault org list`,
	RunE: runOrgList,
}

func init() {
	orgCreateCmd.Flags().StringVar(&orgCreateName, "name", "", "Organization name (slug, required)")
	orgCreateCmd.Flags().StringVar(&orgCreateDisplayName, "display-name", "", "Display name")
	orgCreateCmd.Flags().StringVar(&orgCreateDescription, "description", "", "Organization description")
	orgCreateCmd.MarkFlagRequired("name")

	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgListCmd)
}

func runOrgCreate(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	org, err := client.CreateOrg(orgCreateName, orgCreateDisplayName, orgCreateDescription)
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Organization created\n")
	fmt.Fprintf(os.Stderr, "  ID:   %s\n", org.ID)
	fmt.Fprintf(os.Stderr, "  Name: %s\n", org.Name)
	if org.DisplayName != "" {
		fmt.Fprintf(os.Stderr, "  Display Name: %s\n", org.DisplayName)
	}

	// Also print as JSON to stdout for scripting
	out, _ := json.MarshalIndent(org, "", "  ")
	fmt.Println(string(out))

	return nil
}

func runOrgList(cmd *cobra.Command, args []string) error {
	client, err := NewClient()
	if err != nil {
		return err
	}

	orgs, err := client.ListOrgs()
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	if len(orgs) == 0 {
		fmt.Fprintf(os.Stderr, "No organizations found\n")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDISPLAY NAME\tMEMBERS\tCREATED")
	for _, o := range orgs {
		created := o.CreatedAt
		if len(created) > 19 {
			created = created[:19]
		}
		displayName := o.DisplayName
		if displayName == "" {
			displayName = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", o.Name, displayName, o.MemberCount, created)
	}
	w.Flush()

	return nil
}
