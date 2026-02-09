package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	runProject string
	runMap     string
)

var runCmd = &cobra.Command{
	Use:   "run --project PROJECT --map \"ENV=path,...\" -- CMD [ARGS...]",
	Short: "Run a command with secrets injected as environment variables",
	Long: `Fetch secrets from TeamVault and inject them as environment variables
into a child process. Secrets are NEVER printed to stdout or stderr.

The --map flag specifies mappings from environment variable names to secret
paths within the project. Multiple mappings are comma-separated.

The child process inherits the current environment plus the injected secrets.
When the child exits, the secrets are gone (they only exist in the child's env).

Examples:
  teamvault run --project myproject --map "STRIPE_KEY=api-keys/stripe,DB_URL=db/postgres-url" -- node server.js
  teamvault run --project myproject --map "API_KEY=keys/main" -- ./my-app --port 8080`,
	DisableFlagParsing: false,
	RunE:               runRun,
}

func init() {
	runCmd.Flags().StringVar(&runProject, "project", "", "Project containing the secrets")
	runCmd.Flags().StringVar(&runMap, "map", "", "Secret mappings as ENV=path,ENV2=path2")
	runCmd.MarkFlagRequired("project")
	runCmd.MarkFlagRequired("map")
}

// parseSecretMappings parses "ENV=path,ENV2=path2" into a map.
func parseSecretMappings(mapStr string) (map[string]string, error) {
	mappings := make(map[string]string)

	for _, entry := range strings.Split(mapStr, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid mapping %q: expected ENV=path format", entry)
		}

		envVar := parts[0]
		secretPath := parts[1]

		// Validate env var name (basic check)
		if strings.ContainsAny(envVar, " \t\n=") {
			return nil, fmt.Errorf("invalid environment variable name %q", envVar)
		}

		mappings[envVar] = secretPath
	}

	if len(mappings) == 0 {
		return nil, fmt.Errorf("no valid secret mappings provided")
	}

	return mappings, nil
}

func runRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified. Usage: teamvault run --project PROJECT --map \"ENV=path\" -- CMD [ARGS...]")
	}

	// Parse secret mappings
	mappings, err := parseSecretMappings(runMap)
	if err != nil {
		return err
	}

	// Create API client
	client, err := NewClient()
	if err != nil {
		return err
	}

	// Fetch all secrets
	// SECURITY: Secret values are NEVER printed to stdout/stderr
	secretEnv := make(map[string]string, len(mappings))
	for envVar, secretPath := range mappings {
		secret, err := client.GetSecret(runProject, secretPath)
		if err != nil {
			return fmt.Errorf("failed to fetch secret %s/%s for %s: %w", runProject, secretPath, envVar, err)
		}
		secretEnv[envVar] = secret.Value
	}

	// Log that we fetched secrets (but NEVER the values themselves)
	fmt.Fprintf(os.Stderr, "✓ Fetched %d secret(s), launching command...\n", len(secretEnv))

	// Build environment: current env + injected secrets
	env := os.Environ()
	for envVar, val := range secretEnv {
		env = append(env, envVar+"="+val)
	}

	// Find the command binary
	binary, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", args[0])
	}

	// Use syscall.Exec to replace the current process with the child.
	// This ensures:
	// 1. Signals are properly forwarded
	// 2. Exit code is preserved
	// 3. No Go runtime remains in memory with secrets
	return syscall.Exec(binary, args, env)
}
