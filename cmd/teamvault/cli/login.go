package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	loginServer string
	loginEmail  string
	loginOIDC   bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a TeamVault server",
	Long: `Authenticate with a TeamVault server using email/password or OIDC.

For email/password login:
  teamvault login --server https://vault.example.com --email user@example.com

For OIDC (SSO) login:
  teamvault login --server https://vault.example.com --oidc
  Opens your browser for single sign-on authentication.`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&loginServer, "server", "", "TeamVault server URL (e.g. https://vault.example.com)")
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "Email address for authentication")
	loginCmd.Flags().BoolVar(&loginOIDC, "oidc", false, "Use OIDC (SSO) authentication flow")
	loginCmd.MarkFlagRequired("server")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Validate server URL
	server := strings.TrimRight(loginServer, "/")
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		return fmt.Errorf("server URL must start with http:// or https://")
	}

	if loginOIDC {
		return runLoginOIDC(server)
	}

	// Email is required for password login
	if loginEmail == "" {
		return fmt.Errorf("--email is required for password login (or use --oidc for SSO)")
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

// OIDCCallbackResponse is the response received on the local callback server.
type OIDCCallbackResponse struct {
	Token string `json:"token"`
	Email string `json:"email"`
	Error string `json:"error"`
}

// runLoginOIDC performs the OIDC browser-based authentication flow.
func runLoginOIDC(server string) error {
	// Generate a random state parameter for CSRF protection
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Start a local HTTP server on a random port for the callback
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start local callback server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Channel to receive the auth result
	resultCh := make(chan OIDCCallbackResponse, 1)
	errCh := make(chan error, 1)

	// Set up the callback handler
	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Check state parameter
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			errCh <- fmt.Errorf("OIDC callback state mismatch (possible CSRF)")
			return
		}

		// Check for error from the server
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body><h2>Authentication Failed</h2><p>%s: %s</p><p>You can close this window.</p></body></html>`, errMsg, desc)
			errCh <- fmt.Errorf("OIDC error: %s — %s", errMsg, desc)
			return
		}

		// Extract token from query params or POST body
		token := r.URL.Query().Get("token")
		email := r.URL.Query().Get("email")

		// If not in query params, try to read from POST JSON body
		if token == "" && r.Method == http.MethodPost {
			var body OIDCCallbackResponse
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				token = body.Token
				email = body.Email
			}
		}

		if token == "" {
			http.Error(w, "No token received", http.StatusBadRequest)
			errCh <- fmt.Errorf("OIDC callback did not include a token")
			return
		}

		// Return success page
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body>
			<h2>✓ Authentication Successful</h2>
			<p>You are logged in as <strong>%s</strong>.</p>
			<p>You can close this window and return to the terminal.</p>
			<script>window.close();</script>
		</body></html>`, email)

		resultCh <- OIDCCallbackResponse{
			Token: token,
			Email: email,
		}
	})

	httpServer := &http.Server{Handler: mux}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server error: %w", err)
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
	}()

	// Build the OIDC authorization URL
	authURL := fmt.Sprintf("%s/api/v1/auth/oidc/authorize?redirect_uri=%s&state=%s",
		server, callbackURL, state)

	fmt.Fprintf(os.Stderr, "Opening browser for OIDC authentication...\n")
	fmt.Fprintf(os.Stderr, "  Callback listening on http://127.0.0.1:%d\n\n", port)

	// Try to open the browser
	if err := openBrowser(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically.\n")
		fmt.Fprintf(os.Stderr, "Please open the following URL in your browser:\n\n")
		fmt.Fprintf(os.Stderr, "  %s\n\n", authURL)
	} else {
		fmt.Fprintf(os.Stderr, "If the browser did not open, visit:\n  %s\n\n", authURL)
	}

	fmt.Fprintf(os.Stderr, "Waiting for authentication (timeout: 5 minutes)...\n")

	// Wait for the callback or timeout
	select {
	case result := <-resultCh:
		// Save token
		if err := SaveToken(TokenData{
			Token:  result.Token,
			Server: server,
		}); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		email := result.Email
		if email == "" {
			email = "(OIDC user)"
		}

		// Save config
		if err := SaveConfig(Config{
			Server: server,
			Email:  email,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
		}

		fmt.Fprintf(os.Stderr, "\n✓ Logged in via OIDC as %s\n", email)
		fmt.Fprintf(os.Stderr, "  Token stored in ~/.teamvault/token\n")
		return nil

	case err := <-errCh:
		return err

	case <-time.After(5 * time.Minute):
		return fmt.Errorf("OIDC authentication timed out after 5 minutes")
	}
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		// Linux and others
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
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
