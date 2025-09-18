package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/razzkumar/vault-env/internal/utils"
	"github.com/razzkumar/vault-env/pkg/config"
	"github.com/razzkumar/vault-env/pkg/vault"
)

// App represents the main application
type App struct {
	vaultClient *vault.Client
}

// New creates a new application instance
func New() (*App, error) {
	vaultConfig := config.GetVaultConfigFromEnv()
	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	return &App{
		vaultClient: client,
	}, nil
}

// PutOptions contains options for the Put operation
type PutOptions struct {
	KVMount      string
	KVPath       string
	TransitMount string
	EncryptionKey string
	Key          string
	Value        string
	FromEnv      string
	FromFile     string
}

// Put stores secrets in Vault with optional encryption
func (a *App) Put(opts *PutOptions) error {
	effectiveEncryptionKey := config.GetEncryptionKey(opts.EncryptionKey)
	useEncryption := effectiveEncryptionKey != ""

	// Get existing data to merge with
	existingData, err := a.vaultClient.KVGet(opts.KVMount, opts.KVPath)
	if err != nil {
		// If secret doesn't exist, start with empty data
		existingData = make(map[string]interface{})
	}

	var finalData map[string]interface{}

	// Handle different data structures in existing data
	if utils.IsEncryptedSingleValue(existingData) || utils.IsPlaintextSingleValue(existingData) {
		finalData = make(map[string]interface{})
	} else {
		finalData = utils.MergeData(make(map[string]interface{}), existingData)
	}

	var newData map[string]interface{}

	if opts.FromEnv != "" {
		// Load from .env file
		newData, err = utils.LoadEnvFile(opts.FromEnv, a.vaultClient, opts.TransitMount, effectiveEncryptionKey, useEncryption)
		if err != nil {
			return fmt.Errorf("load env file: %w", err)
		}
		// Merge with existing data
		finalData = utils.MergeData(finalData, newData)
	} else if opts.FromFile != "" {
		// Load file as base64
		newData, err = utils.LoadFileAsBase64(opts.FromFile, a.vaultClient, opts.TransitMount, effectiveEncryptionKey, useEncryption)
		if err != nil {
			return fmt.Errorf("load file: %w", err)
		}
		finalData = newData
	} else {
		// Single value (from --value, stdin, or key update)
		var secretValue []byte

		if opts.Value != "" {
			secretValue = []byte(opts.Value)
		} else {
			// Read from stdin
			secretValue, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			// Remove trailing newline if reading from stdin
			if len(secretValue) > 0 && secretValue[len(secretValue)-1] == '\n' {
				secretValue = secretValue[:len(secretValue)-1]
			}
		}

		if len(secretValue) == 0 {
			return fmt.Errorf("no secret value provided")
		}

		// Handle key-specific update or single value storage
		if opts.Key != "" {
			// Update specific key in multi-value secret
			if useEncryption {
				ciphertext, err := a.vaultClient.TransitEncrypt(opts.TransitMount, effectiveEncryptionKey, secretValue)
				if err != nil {
					return fmt.Errorf("transit encrypt: %w", err)
				}
				finalData[opts.Key] = ciphertext
			} else {
				finalData[opts.Key] = string(secretValue)
			}
		} else {
			// Single value storage (backward compatibility)
			if useEncryption {
				ciphertext, err := a.vaultClient.TransitEncrypt(opts.TransitMount, effectiveEncryptionKey, secretValue)
				if err != nil {
					return fmt.Errorf("transit encrypt: %w", err)
				}
				finalData = map[string]interface{}{"ciphertext": ciphertext}
			} else {
				finalData = map[string]interface{}{"value": string(secretValue)}
			}
		}
	}

	if err := a.vaultClient.KVPut(opts.KVMount, opts.KVPath, finalData); err != nil {
		return fmt.Errorf("kv put: %w", err)
	}

	encryptionStatus := "plaintext"
	if useEncryption {
		encryptionStatus = "encrypted"
	}

	if opts.Key != "" {
		fmt.Printf("Updated key '%s' as %s: %s/%s\n", opts.Key, encryptionStatus, opts.KVMount, opts.KVPath)
	} else {
		secretsCount := len(finalData)
		fmt.Printf("Stored/updated %d secret(s) as %s: %s/%s\n", secretsCount, encryptionStatus, opts.KVMount, opts.KVPath)
	}

	return nil
}

// GetOptions contains options for the Get operation
type GetOptions struct {
	KVMount       string
	KVPath        string
	TransitMount  string
	EncryptionKey string
	Key           string
	OutputJSON    bool
}

// Get retrieves and optionally decrypts secrets from Vault
func (a *App) Get(opts *GetOptions) error {
	effectiveEncryptionKey := config.GetEncryptionKey(opts.EncryptionKey)

	// Get from KV
	data, err := a.vaultClient.KVGet(opts.KVMount, opts.KVPath)
	if err != nil {
		return fmt.Errorf("kv get: %w", err)
	}

	// Try to get single encrypted data first
	ciphertext, hasCiphertext := data["ciphertext"].(string)
	if hasCiphertext && ciphertext != "" {
		// Single encrypted data - requires key
		if effectiveEncryptionKey == "" {
			return fmt.Errorf("--encryption-key is required for encrypted secrets")
		}
		plaintext, err := a.vaultClient.TransitDecrypt(opts.TransitMount, effectiveEncryptionKey, ciphertext)
		if err != nil {
			return fmt.Errorf("transit decrypt: %w", err)
		}
		fmt.Print(string(plaintext))
		return nil
	}

	// Handle encrypted multi-value data
	if utils.IsEncryptedMultiValue(data) {
		if effectiveEncryptionKey == "" {
			return fmt.Errorf("--encryption-key is required for encrypted secrets")
		}

		decryptedData, err := utils.DecryptMultiValueData(data, a.vaultClient, opts.TransitMount, effectiveEncryptionKey)
		if err != nil {
			return fmt.Errorf("decrypt multi-value data: %w", err)
		}

		// Handle output for decrypted multi-value data
		if opts.Key != "" {
			value, ok := decryptedData[opts.Key]
			if !ok {
				return fmt.Errorf("key %q not found", opts.Key)
			}
			fmt.Print(value)
		} else if opts.OutputJSON {
			if err := utils.OutputJSON(decryptedData); err != nil {
				return fmt.Errorf("output json: %w", err)
			}
		} else {
			utils.OutputEnvFormat(decryptedData)
		}
		return nil
	}

	// Handle plaintext data (single value or multiple values)
	if opts.Key != "" {
		// Get specific key
		value, ok := data[opts.Key]
		if !ok {
			return fmt.Errorf("key %q not found", opts.Key)
		}
		fmt.Print(value)
	} else if len(data) == 1 {
		// Single value - print it directly
		for _, v := range data {
			fmt.Print(v)
			break
		}
	} else {
		// Multiple values - output based on format
		if opts.OutputJSON {
			if err := utils.OutputJSON(data); err != nil {
				return fmt.Errorf("output json: %w", err)
			}
		} else {
			utils.OutputEnvFormat(data)
		}
	}

	return nil
}

// LoadConfig loads configuration from a YAML file
func (a *App) LoadConfig(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml config: %w", err)
	}

	return &cfg, nil
}

// GenerateEnvFile generates a .env file from multiple vault secrets
func (a *App) GenerateEnvFile(configPath, outputPath string, encryptionKey string) error {
	cfg, err := a.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	effectiveEncryptionKey := config.GetEncryptionKey(encryptionKey)
	
	var envLines []string

	for _, secret := range cfg.Secrets {
		if secret.EnvVar == "" || secret.KVPath == "" {
			fmt.Printf("skipping invalid secret entry: %s\n", secret.Name)
			continue
		}

		// Get secret from KV
		data, err := a.vaultClient.KVGet(config.NonEmpty("", cfg.KV.Mount, "kv"), secret.KVPath)
		if err != nil {
			if secret.Required {
				return fmt.Errorf("failed to get required secret %s: %w", secret.Name, err)
			}
			fmt.Printf("warning: failed to get secret %s: %v\n", secret.Name, err)
			continue
		}

		var secretValue string

		// Handle different secret types
		if ciphertext, ok := data["ciphertext"].(string); ok && strings.HasPrefix(ciphertext, "vault:v") {
			// Single encrypted value
			encKeyForDecrypt := config.NonEmpty(effectiveEncryptionKey, cfg.Transit.Key, "")
			if encKeyForDecrypt == "" {
				if secret.Required {
					return fmt.Errorf("encryption key required for encrypted secret %s", secret.Name)
				}
				fmt.Printf("warning: no encryption key available for secret %s\n", secret.Name)
				continue
			}
			plaintext, err := a.vaultClient.TransitDecrypt(config.NonEmpty("", cfg.Transit.Mount, "transit"), encKeyForDecrypt, ciphertext)
			if err != nil {
				if secret.Required {
					return fmt.Errorf("failed to decrypt required secret %s: %w", secret.Name, err)
				}
				fmt.Printf("warning: failed to decrypt secret %s: %v\n", secret.Name, err)
				continue
			}
			secretValue = string(plaintext)
		} else if value, ok := data["value"].(string); ok {
			// Single plaintext value
			secretValue = value
		} else if len(data) > 1 {
			// Multi-value secret - this shouldn't be used in env generation typically
			if secret.Required {
				return fmt.Errorf("secret %s contains multiple values, cannot determine which to use for %s", secret.Name, secret.EnvVar)
			}
			fmt.Printf("warning: secret %s contains multiple values, skipping\n", secret.Name)
			continue
		} else {
			if secret.Required {
				return fmt.Errorf("no valid data found for required secret %s", secret.Name)
			}
			fmt.Printf("warning: no valid data found for secret %s\n", secret.Name)
			continue
		}

		// Add to env format
		envLines = append(envLines, fmt.Sprintf("%s=%s", secret.EnvVar, secretValue))
	}

	// Write to file
	content := strings.Join(envLines, "\n")
	if len(envLines) > 0 {
		content += "\n" // Add final newline
	}

	if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	fmt.Printf("Generated %s with %d secrets\n", outputPath, len(envLines))
	return nil
}