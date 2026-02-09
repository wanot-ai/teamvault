package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Manage key-value secrets",
	Long: `Manage key-value secrets in TeamVault projects.

Examples:
  teamvault kv get myproject/api-keys/stripe
  teamvault kv put myproject/api-keys/stripe --value sk_live_xxx
  teamvault kv list myproject`,
}

var kvGetCmd = &cobra.Command{
	Use:   "get PROJECT/PATH",
	Short: "Fetch and print a secret value",
	Long: `Fetch a secret from TeamVault and print its value to stdout.

The argument should be in the format PROJECT/PATH where PATH can contain
multiple segments separated by slashes.

Examples:
  teamvault kv get myproject/api-keys/stripe
  teamvault kv get myproject/db/postgres-url`,
	Args: cobra.ExactArgs(1),
	RunE: runKVGet,
}

var (
	kvPutValue string
)

var kvPutCmd = &cobra.Command{
	Use:   "put PROJECT/PATH --value VALUE",
	Short: "Create or update a secret",
	Long: `Create or update a secret in TeamVault. If the secret already exists,
a new version is created.

Examples:
  teamvault kv put myproject/api-keys/stripe --value sk_live_xxx
  teamvault kv put myproject/db/postgres-url --value "postgres://user:pass@host/db"`,
	Args: cobra.ExactArgs(1),
	RunE: runKVPut,
}

var kvListCmd = &cobra.Command{
	Use:   "list PROJECT",
	Short: "List secrets in a project",
	Long: `List all secrets in a TeamVault project. Only paths are shown,
not secret values.

Examples:
  teamvault kv list myproject`,
	Args: cobra.ExactArgs(1),
	RunE: runKVList,
}

func init() {
	kvPutCmd.Flags().StringVar(&kvPutValue, "value", "", "Secret value to store")
	kvPutCmd.MarkFlagRequired("value")

	kvCmd.AddCommand(kvGetCmd)
	kvCmd.AddCommand(kvPutCmd)
	kvCmd.AddCommand(kvListCmd)
}

// parseProjectPath splits "project/path/to/secret" into project and path.
func parseProjectPath(arg string) (string, string, error) {
	idx := strings.Index(arg, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid format: expected PROJECT/PATH (e.g. myproject/api-keys/stripe)")
	}

	project := arg[:idx]
	path := arg[idx+1:]

	if project == "" {
		return "", "", fmt.Errorf("project name cannot be empty")
	}
	if path == "" {
		return "", "", fmt.Errorf("secret path cannot be empty")
	}

	return project, path, nil
}

func runKVGet(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	secret, err := client.GetSecret(project, path)
	if err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", project, path, err)
	}

	// Print only the value to stdout (no trailing newline for piping)
	fmt.Print(secret.Value)
	return nil
}

func runKVPut(cmd *cobra.Command, args []string) error {
	project, path, err := parseProjectPath(args[0])
	if err != nil {
		return err
	}

	if kvPutValue == "" {
		return fmt.Errorf("--value cannot be empty")
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	if err := client.PutSecret(project, path, kvPutValue); err != nil {
		return fmt.Errorf("failed to put secret %s/%s: %w", project, path, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Secret %s/%s saved\n", project, path)
	return nil
}

func runKVList(cmd *cobra.Command, args []string) error {
	project := args[0]
	if project == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	secrets, err := client.ListSecrets(project)
	if err != nil {
		return fmt.Errorf("failed to list secrets in %s: %w", project, err)
	}

	if len(secrets) == 0 {
		fmt.Fprintf(os.Stderr, "No secrets found in project %q\n", project)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PATH\tVERSION\tCREATED")
	for _, s := range secrets {
		created := s.CreatedAt
		if len(created) > 19 {
			created = created[:19] // Trim to readable datetime
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n", s.Path, s.Version, created)
	}
	w.Flush()

	return nil
}
