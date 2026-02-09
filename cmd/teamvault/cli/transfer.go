package cli

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

// --- Export/Import types ---

// ExportEnvelope is the top-level structure of an exported secrets file.
type ExportEnvelope struct {
	Version    int              `json:"version"`
	Format     string           `json:"format"`
	Project    string           `json:"project"`
	ExportedAt string           `json:"exported_at"`
	ExportedBy string           `json:"exported_by,omitempty"`
	Encryption ExportEncryption `json:"encryption"`
	Secrets    []ExportedSecret `json:"secrets"`
}

// ExportEncryption describes how the secret values are encrypted in transit.
type ExportEncryption struct {
	Algorithm  string `json:"algorithm"`  // aes-256-gcm
	KDF        string `json:"kdf"`        // pbkdf2-sha256
	Iterations int    `json:"iterations"` // 600000
	SaltHex    string `json:"salt"`       // hex-encoded salt
}

// ExportedSecret is a single secret in the export file.
type ExportedSecret struct {
	Path           string `json:"path"`
	Version        int    `json:"version"`
	Description    string `json:"description,omitempty"`
	EncryptedValue string `json:"encrypted_value"` // base64(nonce + ciphertext)
}

// plainSecret is used internally during export/import.
type plainSecret struct {
	Path        string
	Value       string
	Version     int
	Description string
}

// --- Cobra commands ---

var (
	exportProject   string
	exportFormat    string
	exportOutput    string
	importProject   string
	importFile      string
	importOverwrite bool
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export secrets from a project (encrypted)",
	Long: `Export all secrets from a project as an encrypted JSON file.

Secret values are re-encrypted with a passphrase you provide. The exported
file can be safely transferred and imported into another TeamVault instance
using 'teamvault import'.

The encryption uses AES-256-GCM with PBKDF2-SHA256 key derivation (600k iterations).

Examples:
  teamvault export --project myproject > secrets.enc.json
  teamvault export --project myproject --format json -o secrets.enc.json
  teamvault export --project myproject --format json | gpg --encrypt > secrets.enc.json.gpg`,
	RunE: runExport,
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import secrets from an encrypted export file",
	Long: `Import secrets from a previously exported file. You will be prompted
for the passphrase used during export.

By default, existing secrets are NOT overwritten. Use --overwrite to replace them.

Examples:
  teamvault import --project myproject --file secrets.enc.json
  teamvault import --project myproject --file secrets.enc.json --overwrite`,
	RunE: runImport,
}

func init() {
	exportCmd.Flags().StringVar(&exportProject, "project", "", "Project to export secrets from")
	exportCmd.Flags().StringVar(&exportFormat, "format", "json", "Export format (json)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (defaults to stdout)")
	exportCmd.MarkFlagRequired("project")

	importCmd.Flags().StringVar(&importProject, "project", "", "Project to import secrets into (overrides file metadata)")
	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "Path to the encrypted export file")
	importCmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "Overwrite existing secrets")
	importCmd.MarkFlagRequired("file")
}

const (
	pbkdf2Iterations = 600000
	saltSize         = 32
	keySize          = 32 // AES-256
)

func runExport(cmd *cobra.Command, args []string) error {
	if exportFormat != "json" {
		return fmt.Errorf("unsupported format %q (only json is supported)", exportFormat)
	}

	client, err := NewClient()
	if err != nil {
		return err
	}

	// Fetch all secrets in the project
	fmt.Fprintf(os.Stderr, "Fetching secrets from project %q...\n", exportProject)

	secrets, err := client.ListSecrets(exportProject)
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	if len(secrets) == 0 {
		return fmt.Errorf("no secrets found in project %q", exportProject)
	}

	fmt.Fprintf(os.Stderr, "Found %d secret(s). Fetching values...\n", len(secrets))

	// Fetch full secret values
	var plainSecrets []plainSecret
	for _, s := range secrets {
		full, err := client.GetSecret(exportProject, s.Path)
		if err != nil {
			return fmt.Errorf("failed to fetch secret %s: %w", s.Path, err)
		}
		plainSecrets = append(plainSecrets, plainSecret{
			Path:        s.Path,
			Value:       full.Value,
			Version:     s.Version,
			Description: s.Description,
		})
	}

	// Prompt for passphrase
	passphrase, err := readExportPassphrase()
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}

	// Generate salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, keySize, sha256.New)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt each secret value
	var exported []ExportedSecret
	for _, s := range plainSecrets {
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return fmt.Errorf("failed to generate nonce: %w", err)
		}

		ciphertext := gcm.Seal(nonce, nonce, []byte(s.Value), nil)

		exported = append(exported, ExportedSecret{
			Path:           s.Path,
			Version:        s.Version,
			Description:    s.Description,
			EncryptedValue: base64.StdEncoding.EncodeToString(ciphertext),
		})
	}

	// Build envelope
	envelope := ExportEnvelope{
		Version:    1,
		Format:     "json",
		Project:    exportProject,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Encryption: ExportEncryption{
			Algorithm:  "aes-256-gcm",
			KDF:        "pbkdf2-sha256",
			Iterations: pbkdf2Iterations,
			SaltHex:    hex.EncodeToString(salt),
		},
		Secrets: exported,
	}

	// Marshal to JSON
	out, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal export: %w", err)
	}

	// Write output
	if exportOutput != "" {
		if err := os.WriteFile(exportOutput, out, 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "✓ Exported %d secret(s) to %s\n", len(exported), exportOutput)
	} else {
		// Write to stdout
		fmt.Println(string(out))
		fmt.Fprintf(os.Stderr, "✓ Exported %d secret(s)\n", len(exported))
	}

	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	// Read the import file
	data, err := os.ReadFile(importFile)
	if err != nil {
		return fmt.Errorf("failed to read import file: %w", err)
	}

	// Parse envelope
	var envelope ExportEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("failed to parse import file: %w", err)
	}

	if envelope.Version != 1 {
		return fmt.Errorf("unsupported export version %d (expected 1)", envelope.Version)
	}

	if envelope.Encryption.Algorithm != "aes-256-gcm" {
		return fmt.Errorf("unsupported encryption algorithm %q", envelope.Encryption.Algorithm)
	}

	project := importProject
	if project == "" {
		project = envelope.Project
	}
	if project == "" {
		return fmt.Errorf("no project specified (use --project or ensure the export file has project metadata)")
	}

	fmt.Fprintf(os.Stderr, "Importing %d secret(s) into project %q...\n", len(envelope.Secrets), project)
	fmt.Fprintf(os.Stderr, "  Source: exported at %s from project %q\n", envelope.ExportedAt, envelope.Project)

	// Prompt for passphrase
	passphrase, err := readImportPassphrase()
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}

	// Decode salt
	salt, err := hex.DecodeString(envelope.Encryption.SaltHex)
	if err != nil {
		return fmt.Errorf("invalid salt in export file: %w", err)
	}

	iterations := envelope.Encryption.Iterations
	if iterations <= 0 {
		iterations = pbkdf2Iterations
	}

	// Derive key
	key := pbkdf2.Key([]byte(passphrase), salt, iterations, keySize, sha256.New)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt and import each secret
	client, err := NewClient()
	if err != nil {
		return err
	}

	var imported, skipped, errored int
	for _, s := range envelope.Secrets {
		// Decode ciphertext
		ciphertext, err := base64.StdEncoding.DecodeString(s.EncryptedValue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s — failed to decode: %v\n", s.Path, err)
			errored++
			continue
		}

		if len(ciphertext) < gcm.NonceSize() {
			fmt.Fprintf(os.Stderr, "  ✗ %s — ciphertext too short\n", s.Path)
			errored++
			continue
		}

		nonce := ciphertext[:gcm.NonceSize()]
		ct := ciphertext[gcm.NonceSize():]

		plaintext, err := gcm.Open(nil, nonce, ct, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s — decryption failed (wrong passphrase?): %v\n", s.Path, err)
			errored++
			continue
		}

		// Check if secret already exists (unless overwrite)
		if !importOverwrite {
			existing, err := client.GetSecret(project, s.Path)
			if err == nil && existing != nil {
				fmt.Fprintf(os.Stderr, "  ⏭ %s — already exists (v%d), skipping\n", s.Path, existing.Version)
				skipped++
				continue
			}
		}

		// Put the secret
		if err := client.PutSecret(project, s.Path, string(plaintext)); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s — failed to import: %v\n", s.Path, err)
			errored++
			continue
		}

		fmt.Fprintf(os.Stderr, "  ✓ %s\n", s.Path)
		imported++
	}

	fmt.Fprintf(os.Stderr, "\nImport complete: %d imported, %d skipped, %d errors\n", imported, skipped, errored)

	if errored > 0 {
		return fmt.Errorf("%d secret(s) failed to import", errored)
	}

	return nil
}

// --- passphrase helpers ---

func readExportPassphrase() (string, error) {
	fmt.Fprint(os.Stderr, "Enter export passphrase: ")
	pass1, err := readTermPassword()
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr)

	fmt.Fprint(os.Stderr, "Confirm passphrase: ")
	pass2, err := readTermPassword()
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr)

	if pass1 != pass2 {
		return "", fmt.Errorf("passphrases do not match")
	}

	if len(pass1) < 8 {
		return "", fmt.Errorf("passphrase must be at least 8 characters")
	}

	return pass1, nil
}

func readImportPassphrase() (string, error) {
	fmt.Fprint(os.Stderr, "Enter import passphrase: ")
	pass, err := readTermPassword()
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr)

	if pass == "" {
		return "", fmt.Errorf("passphrase cannot be empty")
	}

	return pass, nil
}

func readTermPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	// Non-interactive: read from stdin
	var pass string
	_, err := fmt.Fscanln(os.Stdin, &pass)
	if err != nil {
		return "", err
	}
	return pass, nil
}
