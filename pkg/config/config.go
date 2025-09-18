package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration
type Config struct {
	Version int `yaml:"version"`
	Vault   struct {
		Addr       string `yaml:"addr"`
		Namespace  string `yaml:"namespace"`
		SkipVerify bool   `yaml:"skip_verify"`
		CACert     string `yaml:"ca_cert"`
	} `yaml:"vault"`
	Transit *struct {
		Mount string `yaml:"mount"`
		Key   string `yaml:"key"`
	} `yaml:"transit,omitempty"`
	KV struct {
		Mount string `yaml:"mount"`
	} `yaml:"kv"`
	Secrets []SecretEntry `yaml:"secrets"`
}

// SecretEntry represents a secret configuration entry
// Supports multiple formats:
// 1. Old format: individual secret mapping (name, kv_path, env_var)
// 2. New format: all keys from path (path only)
// 3. Selective format: single key from path (path + key)
// 4. Mapped format: single key from path with custom env name (path + key + env_key)
type SecretEntry struct {
	// Old format - individual secret mapping
	Name     string `yaml:"name,omitempty"`
	KVPath   string `yaml:"kv_path,omitempty"` // path under kv mount
	EnvVar   string `yaml:"env_var,omitempty"` // environment variable name
	Required bool   `yaml:"required,omitempty"` // fail if secret not found
	
	// New formats - path-based
	Path   string `yaml:"path,omitempty"`    // vault path
	Key    string `yaml:"key,omitempty"`     // specific key to extract (optional)
	EnvKey string `yaml:"env_key,omitempty"` // custom env var name (optional, requires key)
}

// VaultConfig holds Vault client configuration
type VaultConfig struct {
	Addr       string
	Token      string
	Namespace  string
	CACert     string
	SkipVerify bool
	Timeout    int // seconds
}

// GetVaultConfigFromEnv creates VaultConfig from environment variables
func GetVaultConfigFromEnv() *VaultConfig {
	cfg := &VaultConfig{
		Addr:      os.Getenv("VAULT_ADDR"),
		Token:     os.Getenv("VAULT_TOKEN"),
		Namespace: os.Getenv("VAULT_NAMESPACE"),
		CACert:    os.Getenv("VAULT_CACERT"),
		Timeout:   15, // default timeout
	}

	if skip := os.Getenv("VAULT_SKIP_VERIFY"); skip == "1" || skip == "true" {
		cfg.SkipVerify = true
	}

	if timeout := os.Getenv("VAULT_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			cfg.Timeout = t
		}
	}

	return cfg
}

// Validate checks if the configuration is valid
func (c *VaultConfig) Validate() error {
	if c.Addr == "" {
		return ErrMissingVaultAddr
	}
	if c.Token == "" {
		return ErrMissingVaultToken
	}
	return nil
}

// GetEncryptionKey returns the encryption key from environment or parameter
func GetEncryptionKey(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("ENCRYPTION_KEY")
}

// NonEmpty returns the first non-empty string from the provided values
func NonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// IsPathBased returns true if this secret entry uses the new path-based format
func (s *SecretEntry) IsPathBased() bool {
	return s.Path != ""
}

// IsIndividual returns true if this secret entry uses the old individual format
func (s *SecretEntry) IsIndividual() bool {
	return s.KVPath != "" && s.EnvVar != ""
}

// IsPathAllKeys returns true if this loads all keys from the path
func (s *SecretEntry) IsPathAllKeys() bool {
	return s.Path != "" && s.Key == ""
}

// IsPathSingleKey returns true if this loads a single key from the path
func (s *SecretEntry) IsPathSingleKey() bool {
	return s.Path != "" && s.Key != ""
}

// GetEnvKeyName returns the environment variable name for this secret
func (s *SecretEntry) GetEnvKeyName() string {
	if s.EnvKey != "" {
		return s.EnvKey
	}
	if s.Key != "" {
		return strings.ToUpper(s.Key)
	}
	return ""
}

// GetTransitMount returns the transit mount path, with fallback
func (c *Config) GetTransitMount(defaultMount string) string {
	if c.Transit != nil && c.Transit.Mount != "" {
		return c.Transit.Mount
	}
	return defaultMount
}

// GetTransitKey returns the transit encryption key
func (c *Config) GetTransitKey() string {
	if c.Transit != nil {
		return c.Transit.Key
	}
	return ""
}
