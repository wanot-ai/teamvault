package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configDir  = ".teamvault"
	tokenFile  = "token"
	configFile = "config.json"
)

// Config holds CLI configuration persisted to disk.
type Config struct {
	Server string `json:"server"`
	Email  string `json:"email"`
}

// TokenData holds the authentication token and server info.
type TokenData struct {
	Token  string `json:"token"`
	Server string `json:"server"`
}

func configDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, configDir), nil
}

func ensureConfigDir() (string, error) {
	dir, err := configDirPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}
	return dir, nil
}

// SaveToken persists the token to ~/.teamvault/token with 0600 permissions.
func SaveToken(data TokenData) error {
	dir, err := ensureConfigDir()
	if err != nil {
		return err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("cannot marshal token data: %w", err)
	}

	path := filepath.Join(dir, tokenFile)
	if err := os.WriteFile(path, b, 0600); err != nil {
		return fmt.Errorf("cannot write token file %s: %w", path, err)
	}
	return nil
}

// LoadToken reads the stored token from ~/.teamvault/token.
func LoadToken() (TokenData, error) {
	dir, err := configDirPath()
	if err != nil {
		return TokenData{}, err
	}

	path := filepath.Join(dir, tokenFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TokenData{}, fmt.Errorf("not logged in. Run 'teamvault login' first")
		}
		return TokenData{}, fmt.Errorf("cannot read token file: %w", err)
	}

	var data TokenData
	if err := json.Unmarshal(b, &data); err != nil {
		return TokenData{}, fmt.Errorf("corrupt token file: %w", err)
	}

	if data.Token == "" {
		return TokenData{}, fmt.Errorf("empty token. Run 'teamvault login' to re-authenticate")
	}

	return data, nil
}

// SaveConfig persists CLI config (server URL, email).
func SaveConfig(cfg Config) error {
	dir, err := ensureConfigDir()
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	path := filepath.Join(dir, configFile)
	if err := os.WriteFile(path, b, 0600); err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}
	return nil
}

// LoadConfig reads CLI config from ~/.teamvault/config.json.
func LoadConfig() (Config, error) {
	dir, err := configDirPath()
	if err != nil {
		return Config{}, err
	}

	path := filepath.Join(dir, configFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil // no config yet, that's fine
		}
		return Config{}, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("corrupt config file: %w", err)
	}
	return cfg, nil
}
