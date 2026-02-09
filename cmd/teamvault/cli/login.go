package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	loginServer string
	loginEmail  string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a TeamVault server",
	Long: `Authenticate with a TeamVault server using email and password.
The authentication token is stored in ~/.teamvault/token with 0600 permissions.

Example:
  teamvault login --server https://vault.example.com --email user@example.com`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&loginServer, "server", "", "TeamVault server URL (e.g. https://vault.example.com)")
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "Email address for authentication")
	loginCmd.MarkFlagRequired("server")
	loginCmd.MarkFlagRequired("email")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Validate server URL
	server := strings.TrimRight(loginServer, "/")
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		return fmt.Errorf("server URL must start with http:// or https://")
	}

	// Prompt for password interactively
	password, err := readPassword("Password: ")
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Authenticate with the server
	client := NewClientWithURL(server)
	fmt.Fprintf(os.Stderr, "Authenticating with %s...\n", server)

	token, err := client.Login(loginEmail, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Save token securely
	if err := SaveToken(TokenData{
		Token:  token,
		Server: server,
	}); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	// Save config for convenience
	if err := SaveConfig(Config{
		Server: server,
		Email:  loginEmail,
	}); err != nil {
		// Non-fatal: token is already saved
		fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Logged in as %s\n", loginEmail)
	fmt.Fprintf(os.Stderr, "  Token stored in ~/.teamvault/token\n")
	return nil
}

// readPassword prompts for a password without echoing input.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	// Check if stdin is a terminal
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr) // newline after password input
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	// Non-interactive: read from stdin (piped input)
	var password string
	_, err := fmt.Fscanln(os.Stdin, &password)
	if err != nil {
		return "", err
	}
	return password, nil
}
