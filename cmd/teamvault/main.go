// TeamVault CLI — manage secrets from the command line
//
// Usage:
//
//	teamvault login --server https://localhost:8443 --email user@example.com
//	teamvault kv get myproject/api-keys/stripe
//	teamvault kv put myproject/api-keys/stripe --value sk_live_xxx
//	teamvault kv list myproject
//	teamvault run --project myproject --map "STRIPE_KEY=api-keys/stripe" -- node server.js
//	teamvault token create --project myproject --scopes read --ttl 1h
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "login":
		cmdLogin()
	case "kv":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: teamvault kv <get|put|list> ...")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "get":
			cmdKVGet()
		case "put":
			cmdKVPut()
		case "list":
			cmdKVList()
		default:
			fmt.Fprintf(os.Stderr, "Unknown kv command: %s\n", os.Args[2])
			os.Exit(1)
		}
	case "run":
		cmdRun()
	case "token":
		if len(os.Args) < 3 || os.Args[2] != "create" {
			fmt.Fprintln(os.Stderr, "Usage: teamvault token create --project <id> --scopes read --ttl 1h")
			os.Exit(1)
		}
		cmdTokenCreate()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`TeamVault CLI

Usage:
  teamvault login --server <url> --email <email>
  teamvault kv get <project>/<path>
  teamvault kv put <project>/<path> --value <value>
  teamvault kv list <project>
  teamvault run --project <project> --map "ENV_VAR=secret/path,..." -- <command>
  teamvault token create --project <id> --scopes <scopes> --ttl <duration>`)
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".teamvault", "config.json")
}

func loadConfig() (*config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, fmt.Errorf("not logged in (run: teamvault login)")
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *config) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

func cmdLogin() {
	var server, email string

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--server":
			if i+1 < len(args) {
				server = args[i+1]
				i++
			}
		case "--email":
			if i+1 < len(args) {
				email = args[i+1]
				i++
			}
		}
	}

	if server == "" || email == "" {
		fmt.Fprintln(os.Stderr, "Usage: teamvault login --server <url> --email <email>")
		os.Exit(1)
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}
	password := string(passwordBytes)

	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	resp, err := http.Post(server+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", result["error"])
		os.Exit(1)
	}

	token, _ := result["token"].(string)
	if err := saveConfig(&config{Server: server, Token: token}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Login successful")
}

func cmdKVGet() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: teamvault kv get <project>/<path>")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ref := os.Args[3]
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Path must be <project>/<path>")
		os.Exit(1)
	}

	url := fmt.Sprintf("%s/api/v1/secrets/%s/%s", cfg.Server, parts[0], parts[1])
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", result["error"])
		os.Exit(1)
	}

	fmt.Print(result["value"])
}

func cmdKVPut() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: teamvault kv put <project>/<path> --value <value>")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ref := os.Args[3]
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Path must be <project>/<path>")
		os.Exit(1)
	}

	var value string
	args := os.Args[4:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--value" && i+1 < len(args) {
			value = args[i+1]
			i++
		}
	}

	if value == "" {
		// Read from stdin if no --value
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		value = string(data)
	}

	body, _ := json.Marshal(map[string]string{"value": value})
	url := fmt.Sprintf("%s/api/v1/secrets/%s/%s", cfg.Server, parts[0], parts[1])
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", result["error"])
		os.Exit(1)
	}

	fmt.Printf("Secret stored (version %v)\n", result["version"])
}

func cmdKVList() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: teamvault kv list <project>")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	project := os.Args[3]
	url := fmt.Sprintf("%s/api/v1/secrets/%s", cfg.Server, project)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var results []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&results)

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Error listing secrets\n")
		os.Exit(1)
	}

	for _, s := range results {
		fmt.Println(s["path"])
	}
}

func cmdRun() {
	var project string
	var mappings string
	var cmdArgs []string

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			cmdArgs = args[i+1:]
			break
		}
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--map":
			if i+1 < len(args) {
				mappings = args[i+1]
				i++
			}
		}
	}

	if project == "" || mappings == "" || len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: teamvault run --project <project> --map \"ENV=path,...\" -- <command>")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Parse mappings and fetch secrets
	env := os.Environ()
	pairs := strings.Split(mappings, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			fmt.Fprintf(os.Stderr, "Invalid mapping: %s\n", pair)
			os.Exit(1)
		}
		envVar := kv[0]
		secretPath := kv[1]

		url := fmt.Sprintf("%s/api/v1/secrets/%s/%s", cfg.Server, project, secretPath)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+cfg.Token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch %s: %v\n", secretPath, err)
			os.Exit(1)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "Failed to fetch %s: %v\n", secretPath, result["error"])
			os.Exit(1)
		}

		value, _ := result["value"].(string)
		env = append(env, envVar+"="+value)
	}

	// Execute command with secrets as env vars
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Failed to run command: %v\n", err)
		os.Exit(1)
	}
}

func cmdTokenCreate() {
	var projectID string
	var scopes string
	var ttl string

	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				projectID = args[i+1]
				i++
			}
		case "--scopes":
			if i+1 < len(args) {
				scopes = args[i+1]
				i++
			}
		case "--ttl":
			if i+1 < len(args) {
				ttl = args[i+1]
				i++
			}
		}
	}

	if projectID == "" {
		fmt.Fprintln(os.Stderr, "Usage: teamvault token create --project <id> [--scopes read,write] [--ttl 1h]")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	scopeList := []string{"read"}
	if scopes != "" {
		scopeList = strings.Split(scopes, ",")
	}

	reqBody := map[string]interface{}{
		"name":       "cli-token",
		"project_id": projectID,
		"scopes":     scopeList,
	}
	if ttl != "" {
		reqBody["ttl"] = ttl
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", cfg.Server+"/api/v1/service-accounts", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 201 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", result["error"])
		os.Exit(1)
	}

	fmt.Printf("Token: %s\n", result["token"])
	fmt.Println("Store this token securely. It cannot be retrieved again.")
}
