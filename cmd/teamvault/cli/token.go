package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage service account tokens",
	Long:  `Create and manage service account tokens for programmatic access to TeamVault.`,
}

var (
	tokenProject string
	tokenScopes  string
	tokenTTL     string
)

var tokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new service account token",
	Long: `Create a new service account token scoped to a specific project.
The token is displayed once and cannot be retrieved again.

Examples:
  teamvault token create --project myproject --scopes read --ttl 1h
  teamvault token create --project myproject --scopes read,write --ttl 24h`,
	RunE: runTokenCreate,
}

func init() {
	tokenCreateCmd.Flags().StringVar(&tokenProject, "project", "", "Project to scope the token to")
	tokenCreateCmd.Flags().StringVar(&tokenScopes, "scopes", "read", "Comma-separated scopes (e.g. read,write)")
	tokenCreateCmd.Flags().StringVar(&tokenTTL, "ttl", "1h", "Token time-to-live (e.g. 1h, 24h, 7d)")
	tokenCreateCmd.MarkFlagRequired("project")

	tokenCmd.AddCommand(tokenCreateCmd)
}

func runTokenCreate(cmd *cobra.Command, args []string) error {
	if tokenProject == "" {
		return fmt.Errorf("--project is required")
	}

	// Parse scopes
	scopes := []string{}
	for _, s := range strings.Split(tokenScopes, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			scopes = append(scopes, s)
		}
	}
	if len(scopes) == 0 {
		return fmt.Errorf("at least one scope is required")
	}

	// Validate TTL format (basic check)
	if tokenTTL == "" {
		return fmt.Errorf("--ttl cannot be empty")
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	result, err := client.CreateServiceAccountToken(tokenProject, scopes, tokenTTL)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Service account token created\n")
	fmt.Fprintf(os.Stderr, "  ID:         %s\n", result.ID)
	if result.Name != "" {
		fmt.Fprintf(os.Stderr, "  Name:       %s\n", result.Name)
	}
	fmt.Fprintf(os.Stderr, "  Project:    %s\n", tokenProject)
	fmt.Fprintf(os.Stderr, "  Scopes:     %s\n", strings.Join(scopes, ", "))
	if result.ExpiresAt != "" {
		fmt.Fprintf(os.Stderr, "  Expires:    %s\n", result.ExpiresAt)
	}
	fmt.Fprintf(os.Stderr, "\n")

	// Print token to stdout so it can be captured by scripts
	fmt.Print(result.Token)

	fmt.Fprintf(os.Stderr, "\n\n⚠  Save this token now — it will not be shown again.\n")
	fmt.Fprintf(os.Stderr, "   Use as: Authorization: Bearer sa.%s\n", result.Token)

	return nil
}
