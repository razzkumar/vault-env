package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
	"github.com/joho/godotenv"

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

// RunOptions contains options for the Run operation
type RunOptions struct {
	KVMount       string
	TransitMount  string
	EncryptionKey string
	ConfigFile    string
	InjectSecrets []string // Format: "ENV_VAR=vault_path"
	EnvFile       string   // Additional .env file to load
	DryRun        bool     // Show env vars without running
	PreserveEnv   bool     // Preserve current environment
	Command       string   // Command to execute
	Args          []string // Arguments for the command
}

// Run executes a command with secrets injected as environment variables
func (a *App) Run(opts *RunOptions) error {
	effectiveEncryptionKey := config.GetEncryptionKey(opts.EncryptionKey)
	
	// Start with current environment if preserve-env is true
	envVars := make(map[string]string)
	if opts.PreserveEnv {
		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envVars[parts[0]] = parts[1]
			}
		}
	}
	
	// Load from .env file if specified
	if opts.EnvFile != "" {
		fileEnvVars, err := a.loadEnvFileForRun(opts.EnvFile)
		if err != nil {
			return fmt.Errorf("load env file %s: %w", opts.EnvFile, err)
		}
		for k, v := range fileEnvVars {
			envVars[k] = v
		}
	}
	
	// Load from config file if specified
	if opts.ConfigFile != "" {
		cfg, err := a.LoadConfig(opts.ConfigFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		
		configEnvVars, err := a.loadSecretsFromConfig(cfg, opts.KVMount, opts.TransitMount, effectiveEncryptionKey)
		if err != nil {
			return fmt.Errorf("load secrets from config: %w", err)
		}
		for k, v := range configEnvVars {
			envVars[k] = v
		}
	}
	
	// Load inline injected secrets
	if len(opts.InjectSecrets) > 0 {
		injectEnvVars, err := a.loadInlineSecrets(opts.InjectSecrets, opts.KVMount, opts.TransitMount, effectiveEncryptionKey)
		if err != nil {
			return fmt.Errorf("load inline secrets: %w", err)
		}
		for k, v := range injectEnvVars {
			envVars[k] = v
		}
	}
	
	// If dry-run, just print the environment variables
	if opts.DryRun {
		fmt.Println("Environment variables that would be set:")
		for k, v := range envVars {
			fmt.Printf("%s=%s\n", k, v)
		}
		fmt.Printf("\nCommand that would be executed: %s %s\n", opts.Command, strings.Join(opts.Args, " "))
		return nil
	}
	
	// Execute the command
	return a.executeCommand(opts.Command, opts.Args, envVars)
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

// Helper methods for Run command

// loadEnvFileForRun loads environment variables from a .env file
func (a *App) loadEnvFileForRun(path string) (map[string]string, error) {
	// Use godotenv to parse the .env file  
	envMap, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}
	return envMap, nil
}

// loadSecretsFromConfig loads secrets from YAML config and returns as env vars
func (a *App) loadSecretsFromConfig(cfg *config.Config, kvMount, transitMount, encryptionKey string) (map[string]string, error) {
	envVars := make(map[string]string)
	
	for _, secret := range cfg.Secrets {
		if secret.EnvVar == "" || secret.KVPath == "" {
			fmt.Printf("skipping invalid secret entry: %s\n", secret.Name)
			continue
		}

		// Get secret from KV
		data, err := a.vaultClient.KVGet(config.NonEmpty("", cfg.KV.Mount, kvMount), secret.KVPath)
		if err != nil {
			if secret.Required {
				return nil, fmt.Errorf("failed to get required secret %s: %w", secret.Name, err)
			}
			fmt.Printf("warning: failed to get secret %s: %v\n", secret.Name, err)
			continue
		}

		var secretValue string

		// Handle different secret types
		if ciphertext, ok := data["ciphertext"].(string); ok && strings.HasPrefix(ciphertext, "vault:v") {
			// Single encrypted value
			encKeyForDecrypt := config.NonEmpty(encryptionKey, cfg.Transit.Key, "")
			if encKeyForDecrypt == "" {
				if secret.Required {
					return nil, fmt.Errorf("encryption key required for encrypted secret %s", secret.Name)
				}
				fmt.Printf("warning: no encryption key available for secret %s\n", secret.Name)
				continue
			}
			plaintext, err := a.vaultClient.TransitDecrypt(config.NonEmpty("", cfg.Transit.Mount, transitMount), encKeyForDecrypt, ciphertext)
			if err != nil {
				if secret.Required {
					return nil, fmt.Errorf("failed to decrypt required secret %s: %w", secret.Name, err)
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
				return nil, fmt.Errorf("secret %s contains multiple values, cannot determine which to use for %s", secret.Name, secret.EnvVar)
			}
			fmt.Printf("warning: secret %s contains multiple values, skipping\n", secret.Name)
			continue
		} else {
			if secret.Required {
				return nil, fmt.Errorf("no valid data found for required secret %s", secret.Name)
			}
			fmt.Printf("warning: no valid data found for secret %s\n", secret.Name)
			continue
		}

		// Add to env vars
		envVars[secret.EnvVar] = secretValue
	}
	
	return envVars, nil
}

// loadInlineSecrets loads secrets specified via --inject flags
func (a *App) loadInlineSecrets(injectSecrets []string, kvMount, transitMount, encryptionKey string) (map[string]string, error) {
	envVars := make(map[string]string)
	
	for _, inject := range injectSecrets {
		// Parse ENV_VAR=vault_path format
		parts := strings.SplitN(inject, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid inject format: %s (expected ENV_VAR=vault_path)", inject)
		}
		
		envVar := strings.TrimSpace(parts[0])
		vaultPath := strings.TrimSpace(parts[1])
		
		if envVar == "" || vaultPath == "" {
			return nil, fmt.Errorf("invalid inject format: %s (empty env var or vault path)", inject)
		}
		
		// Get secret from Vault
		data, err := a.vaultClient.KVGet(kvMount, vaultPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret %s: %w", vaultPath, err)
		}
		
		var secretValue string
		
		// Handle different secret types
		if ciphertext, ok := data["ciphertext"].(string); ok && strings.HasPrefix(ciphertext, "vault:v") {
			// Single encrypted value
			if encryptionKey == "" {
				return nil, fmt.Errorf("encryption key required for encrypted secret %s", vaultPath)
			}
			plaintext, err := a.vaultClient.TransitDecrypt(transitMount, encryptionKey, ciphertext)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt secret %s: %w", vaultPath, err)
			}
			secretValue = string(plaintext)
		} else if value, ok := data["value"].(string); ok {
			// Single plaintext value
			secretValue = value
		} else if len(data) == 1 {
			// Single value with any key
			for _, v := range data {
				secretValue = fmt.Sprintf("%v", v)
				break
			}
		} else {
			return nil, fmt.Errorf("secret %s contains multiple values, cannot inject as single environment variable", vaultPath)
		}
		
		envVars[envVar] = secretValue
	}
	
	return envVars, nil
}

// executeCommand runs the specified command with the provided environment variables
func (a *App) executeCommand(command string, args []string, envVars map[string]string) error {
	// Convert environment variables to []string format
	envSlice := make([]string, 0, len(envVars))
	for k, v := range envVars {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}
	
	// Create the command
	cmd := exec.Command(command, args...)
	cmd.Env = envSlice
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	// Run the command and wait for it to complete
	err := cmd.Run()
	if err != nil {
		// Check if it's an exit error to preserve the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		return fmt.Errorf("command execution failed: %w", err)
	}
	
	return nil
}
