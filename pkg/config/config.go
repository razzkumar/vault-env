package config

import (
	"os"
	"strconv"
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
	Transit struct {
		Mount string `yaml:"mount"`
		Key   string `yaml:"key"`
	} `yaml:"transit"`
	KV struct {
		Mount string `yaml:"mount"`
	} `yaml:"kv"`
	Secrets []SecretEntry `yaml:"secrets"`
}

// SecretEntry represents a secret configuration entry
type SecretEntry struct {
	Name     string `yaml:"name"`
	KVPath   string `yaml:"kv_path"`   // path under kv mount
	EnvVar   string `yaml:"env_var"`   // environment variable name
	Required bool   `yaml:"required"`  // fail if secret not found
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