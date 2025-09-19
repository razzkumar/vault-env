package config

import (
	"fmt"
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
	
	// Authentication methods
	AuthMethod string // auto-detected or explicitly set
	
	// AppRole auth
	RoleID   string
	SecretID string
	
	// GitHub auth
	GitHubToken string
	
	// Kubernetes auth
	K8sRole        string
	K8sJWTPath     string // defaults to /var/run/secrets/kubernetes.io/serviceaccount/token
	K8sAuthPath    string // defaults to kubernetes
}

// GetVaultConfigFromEnv creates VaultConfig from environment variables
func GetVaultConfigFromEnv() *VaultConfig {
	cfg := &VaultConfig{
		Addr:      os.Getenv("VAULT_ADDR"),
		Token:     os.Getenv("VAULT_TOKEN"),
		Namespace: os.Getenv("VAULT_NAMESPACE"),
		CACert:    os.Getenv("VAULT_CACERT"),
		Timeout:   15, // default timeout
		
		// Auth method (explicit or auto-detected)
		AuthMethod: strings.ToLower(os.Getenv("VAULT_AUTH_METHOD")),
		
		// AppRole auth
		RoleID:   os.Getenv("VAULT_ROLE_ID"),
		SecretID: os.Getenv("VAULT_SECRET_ID"),
		
		// GitHub auth
		GitHubToken: os.Getenv("VAULT_GITHUB_TOKEN"),
		
		// Kubernetes auth
		K8sRole:     os.Getenv("VAULT_K8S_ROLE"),
		K8sJWTPath:  os.Getenv("VAULT_K8S_JWT_PATH"),
		K8sAuthPath: os.Getenv("VAULT_K8S_AUTH_PATH"),
	}

	if skip := os.Getenv("VAULT_SKIP_VERIFY"); skip == "1" || skip == "true" {
		cfg.SkipVerify = true
	}

	if timeout := os.Getenv("VAULT_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			cfg.Timeout = t
		}
	}
	
	// Set defaults for Kubernetes auth
	if cfg.K8sJWTPath == "" {
		cfg.K8sJWTPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	}
	if cfg.K8sAuthPath == "" {
		cfg.K8sAuthPath = "kubernetes"
	}

	return cfg
}

// Validate checks if the configuration is valid
func (c *VaultConfig) Validate() error {
	if c.Addr == "" {
		return ErrMissingVaultAddr
	}
	
	// Auto-detect auth method if not explicitly set
	if c.AuthMethod == "" {
		c.AuthMethod = c.DetectAuthMethod()
	}
	
	// Validate based on auth method
	switch c.AuthMethod {
	case "token":
		if c.Token == "" {
			return ErrMissingVaultToken
		}
	case "approle":
		if c.RoleID == "" {
			return fmt.Errorf("VAULT_ROLE_ID is required for AppRole auth")
		}
		if c.SecretID == "" {
			return fmt.Errorf("VAULT_SECRET_ID is required for AppRole auth")
		}
	case "github":
		if c.GitHubToken == "" {
			return fmt.Errorf("VAULT_GITHUB_TOKEN is required for GitHub auth")
		}
	case "kubernetes":
		if c.K8sRole == "" {
			return fmt.Errorf("VAULT_K8S_ROLE is required for Kubernetes auth")
		}
	default:
		return fmt.Errorf("unsupported or auto-detected auth method: %s. Supported: token, approle, github, kubernetes", c.AuthMethod)
	}
	
	return nil
}

// DetectAuthMethod auto-detects the auth method based on available credentials
func (c *VaultConfig) DetectAuthMethod() string {
	// Priority order for auto-detection
	if c.Token != "" {
		return "token"
	}
	if c.RoleID != "" && c.SecretID != "" {
		return "approle"
	}
	if c.GitHubToken != "" {
		return "github"
	}
	if c.K8sRole != "" {
		return "kubernetes"
	}
	// Default to token if nothing else detected
	return "token"
}

// GetEncryptionKey returns the encryption key from environment or parameter
// If TRANSIT is enabled and no key is configured, returns default "app-secrets"
func GetEncryptionKey(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	
	envKey := os.Getenv("ENCRYPTION_KEY")
	if envKey != "" {
		return envKey
	}
	
	// If TRANSIT is enabled but no encryption key configured, use default
	if IsTransitEnabled() {
		return "app-secrets"
	}
	
	return ""
}

// IsTransitEnabled returns true if transit encryption should be enabled
// Checks TRANSIT environment variable for true/false or 1/0 values
func IsTransitEnabled() bool {
	transit := strings.ToLower(os.Getenv("TRANSIT"))
	switch transit {
	case "true", "1", "yes", "on", "enable", "enabled":
		return true
	case "false", "0", "no", "off", "disable", "disabled":
		return false
	default:
		// If TRANSIT is not set or invalid, don't enable by default
		return false
	}
}

// GetTransitMount returns the transit mount path with default fallback
// If TRANSIT is enabled and no mount is configured, returns default "transit"
func GetTransitMount(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	
	envMount := os.Getenv("TRANSIT_MOUNT")
	if envMount != "" {
		return envMount
	}
	
	// Default to "transit" (this is already the default in CLI flags, but good to be explicit)
	return "transit"
}

// ShouldUseEncryption determines if encryption should be used based on encryption key and TRANSIT env var
func ShouldUseEncryption(encryptionKey string) bool {
	// If TRANSIT is explicitly enabled, use encryption
	if IsTransitEnabled() {
		return true
	}
	
	// If encryption key is provided and TRANSIT is not explicitly disabled, use encryption
	if encryptionKey != "" {
		// Check if TRANSIT is explicitly disabled
		transit := strings.ToLower(os.Getenv("TRANSIT"))
		if transit == "false" || transit == "0" || transit == "no" || transit == "off" || transit == "disable" || transit == "disabled" {
			return false
		}
		return true
	}
	
	// Default: no encryption
	return false
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
