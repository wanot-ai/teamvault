// Package rotation implements the secret rotation framework.
package rotation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// Connector defines the interface for rotation connectors.
// Each connector knows how to generate a new secret value for a specific type.
type Connector interface {
	// Type returns the connector type name.
	Type() string

	// Rotate generates a new secret value based on the connector config.
	// Returns the new plaintext value.
	Rotate(ctx context.Context, config json.RawMessage) (string, error)

	// Validate checks if the config is valid for this connector.
	Validate(config json.RawMessage) error
}

// RandomPasswordConfig is the configuration for the random_password connector.
type RandomPasswordConfig struct {
	Length      int    `json:"length"`
	Charset     string `json:"charset"`
	IncludeUpper bool  `json:"include_upper"`
	IncludeLower bool  `json:"include_lower"`
	IncludeDigits bool  `json:"include_digits"`
	IncludeSpecial bool `json:"include_special"`
}

// RandomPasswordConnector generates random passwords on rotation.
type RandomPasswordConnector struct{}

const (
	lowerChars   = "abcdefghijklmnopqrstuvwxyz"
	upperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars   = "0123456789"
	specialChars = "!@#$%^&*()-_=+[]{}|;:',.<>?/"
)

// Type returns "random_password".
func (c *RandomPasswordConnector) Type() string {
	return "random_password"
}

// Rotate generates a new random password.
func (c *RandomPasswordConnector) Rotate(ctx context.Context, config json.RawMessage) (string, error) {
	cfg := RandomPasswordConfig{
		Length:         32,
		IncludeUpper:  true,
		IncludeLower:  true,
		IncludeDigits: true,
		IncludeSpecial: true,
	}

	if config != nil && len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return "", fmt.Errorf("invalid random_password config: %w", err)
		}
	}

	if cfg.Length <= 0 {
		cfg.Length = 32
	}
	if cfg.Length > 256 {
		cfg.Length = 256
	}

	var charset string
	if cfg.Charset != "" {
		charset = cfg.Charset
	} else {
		var sb strings.Builder
		if cfg.IncludeLower {
			sb.WriteString(lowerChars)
		}
		if cfg.IncludeUpper {
			sb.WriteString(upperChars)
		}
		if cfg.IncludeDigits {
			sb.WriteString(digitChars)
		}
		if cfg.IncludeSpecial {
			sb.WriteString(specialChars)
		}
		charset = sb.String()
	}

	if charset == "" {
		charset = lowerChars + upperChars + digitChars
	}

	password := make([]byte, cfg.Length)
	for i := range password {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("generating random index: %w", err)
		}
		password[i] = charset[idx.Int64()]
	}

	return string(password), nil
}

// Validate checks if the config is valid.
func (c *RandomPasswordConnector) Validate(config json.RawMessage) error {
	if config == nil || len(config) == 0 {
		return nil // Use defaults
	}
	var cfg RandomPasswordConfig
	return json.Unmarshal(config, &cfg)
}

// Registry holds all registered rotation connectors.
type Registry struct {
	connectors map[string]Connector
}

// NewRegistry creates a new connector registry with built-in connectors.
func NewRegistry() *Registry {
	r := &Registry{
		connectors: make(map[string]Connector),
	}
	// Register built-in connectors
	r.Register(&RandomPasswordConnector{})
	return r
}

// Register adds a connector to the registry.
func (r *Registry) Register(c Connector) {
	r.connectors[c.Type()] = c
}

// Get retrieves a connector by type.
func (r *Registry) Get(connectorType string) (Connector, error) {
	c, ok := r.connectors[connectorType]
	if !ok {
		return nil, fmt.Errorf("unknown connector type: %s", connectorType)
	}
	return c, nil
}
