// vault-env - A minimal CLI tool for managing secrets with HashiCorp Vault
//
// Features:
// - put:  Store secrets in Vault with Transit encryption
// - get:  Retrieve and decrypt secrets from Vault
// - env:  Generate .env file from Vault secrets
// - sync: Sync secrets from YAML config to .env file
//
// Environment variables:
// - VAULT_ADDR: Vault server address
// - VAULT_TOKEN: Vault authentication token
// - VAULT_NAMESPACE: Vault namespace (optional)
// - VAULT_CACERT: CA certificate path (optional)
// - VAULT_SKIP_VERIFY: Skip TLS verification (optional)

package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v3"
)

// --------------- Config types ---------------

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

type SecretEntry struct {
	Name     string `yaml:"name"`
	KVPath   string `yaml:"kv_path"`  // path under kv mount
	EnvVar   string `yaml:"env_var"`  // environment variable name
	Required bool   `yaml:"required"` // fail if secret not found
}

// --------------- CLI ---------------

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	sub := os.Args[1]
	switch sub {
	case "put":
		cmdPut(os.Args[2:])
	case "get":
		cmdGet(os.Args[2:])
	case "env":
		cmdEnv(os.Args[2:])
	case "sync":
		cmdSync(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", sub)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`vault-env - Minimal secrets management with Vault (optionally with Transit encryption)

COMMANDS:
  put       Store/update secrets in Vault (merges with existing data)
  get       Retrieve and optionally decrypt secrets from Vault
  env       Generate .env file from multiple Vault secrets
  sync      Sync secrets from YAML config to .env file

ENVIRONMENT:
  VAULT_ADDR, VAULT_TOKEN (required)
  VAULT_NAMESPACE, VAULT_CACERT, VAULT_SKIP_VERIFY (optional)
  ENCRYPTION_KEY - Default transit encryption key (optional)

EXAMPLES:
  # Store a single secret with transit encryption
  vault-env put --encryption-key mykey --path secrets/db_password --value "supersecret"
  
  # Store using environment variable for encryption key
  ENCRYPTION_KEY=mykey vault-env put --path secrets/db_password --value "supersecret"
  
  # Store without encryption
  vault-env put --path secrets/db_password --value "supersecret"
  
  # Store multiple secrets from .env file (merges with existing)
  vault-env put --encryption-key mykey --path secrets/myapp --from-env .env
  
  # Store file as base64 encoded value
  vault-env put --encryption-key mykey --path secrets/ssh_key --from-file ~/.ssh/id_rsa
  
  # Update specific key in existing multi-value secret
  vault-env put --encryption-key mykey --path secrets/myapp --key API_KEY --value "new-api-key"
  
  # Retrieve a secret
  vault-env get --encryption-key mykey --path secrets/db_password
  
  # Retrieve specific key from multi-value secret
  vault-env get --encryption-key mykey --path secrets/myapp --key API_KEY
  
  # Generate .env from multiple secrets
  vault-env env --encryption-key mykey --config secrets.yaml --output .env
  
  # Sync from config file
  vault-env sync --config secrets.yaml
`)
}

// --------------- Subcommand: put ---------------

func cmdPut(args []string) {
	fs := flag.NewFlagSet("put", flag.ExitOnError)
	kvMount := fs.String("kv-mount", "kv", "KV v2 mount path")
	kvPath := fs.String("path", "", "KV path to store secret(s)")
	transitMount := fs.String("transit-mount", "transit", "Transit mount path")
	encryptionKey := fs.String("encryption-key", "", "Transit encryption key name (optional)")
	key := fs.String("key", "", "Specific key to update in multi-value secret")
	value := fs.String("value", "", "Secret value (or use stdin)")
	fromEnv := fs.String("from-env", "", "Load multiple key-value pairs from .env file")
	fromFile := fs.String("from-file", "", "Load file content as base64 encoded value")
	fs.Parse(args)

	if *kvPath == "" {
		fs.Usage()
		log.Fatal("--path is required")
	}

	// Get encryption key from flag or environment
	effectiveEncryptionKey := *encryptionKey
	if effectiveEncryptionKey == "" {
		effectiveEncryptionKey = os.Getenv("ENCRYPTION_KEY")
	}

	// Validate input options
	inputCount := 0
	if *value != "" {
		inputCount++
	}
	if *fromEnv != "" {
		inputCount++
	}
	if *fromFile != "" {
		inputCount++
	}

	// If no input specified and no key for update, read from stdin
	if inputCount == 0 && *key == "" {
		// Will read from stdin later
	} else if inputCount > 1 {
		log.Fatal("only one of --value, --from-env, or --from-file can be specified")
	}

	// Validate key update operation
	if *key != "" && (*fromEnv != "" || *fromFile != "") {
		log.Fatal("--key cannot be used with --from-env or --from-file")
	}

	client := mustVaultClientFromEnv()

	// Determine if we should use encryption
	useEncryption := effectiveEncryptionKey != ""

	// Get existing data to merge with
	existingData, err := kvv2GetData(client, *kvMount, *kvPath)
	if err != nil {
		// If secret doesn't exist, start with empty data
		existingData = make(map[string]interface{})
	}

	// Prepare the final data map starting with existing data
	var finalData map[string]interface{}

	// Handle different data structures in existing data
	if isEncryptedSingleValue(existingData) || isPlaintextSingleValue(existingData) {
		// Convert single value to multi-value format for merging
		finalData = make(map[string]interface{})
		// Keep existing single value structure if we're not adding multiple values
	} else {
		// Start with existing multi-value data
		finalData = make(map[string]interface{})
		for k, v := range existingData {
			finalData[k] = v
		}
	}

	var newData map[string]interface{}

	if *fromEnv != "" {
		// Load from .env file
		newData, err = loadEnvFile(*fromEnv, client, *transitMount, effectiveEncryptionKey, useEncryption)
		if err != nil {
			log.Fatalf("load env file: %v", err)
		}
		// Merge with existing data
		for k, v := range newData {
			finalData[k] = v
		}
	} else if *fromFile != "" {
		// Load file as base64
		fileContent, err := os.ReadFile(*fromFile)
		if err != nil {
			log.Fatalf("read file: %v", err)
		}
		base64Content := base64.StdEncoding.EncodeToString(fileContent)
		
		if useEncryption {
			ciphertext, err := transitEncrypt(client, *transitMount, effectiveEncryptionKey, []byte(base64Content))
			if err != nil {
				log.Fatalf("transit encrypt: %v", err)
			}
			finalData = map[string]interface{}{"ciphertext": ciphertext}
		} else {
			finalData = map[string]interface{}{"value": base64Content}
		}
	} else {
		// Single value (from --value, stdin, or key update)
		var secretValue []byte
		
		if *value != "" {
			secretValue = []byte(*value)
		} else {
			// Read from stdin
			secretValue, err = io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("read stdin: %v", err)
			}
			// Remove trailing newline if reading from stdin
			if len(secretValue) > 0 && secretValue[len(secretValue)-1] == '\n' {
				secretValue = secretValue[:len(secretValue)-1]
			}
		}

		if len(secretValue) == 0 {
			log.Fatal("no secret value provided")
		}

		// Handle key-specific update or single value storage
		if *key != "" {
			// Update specific key in multi-value secret
			if useEncryption {
				ciphertext, err := transitEncrypt(client, *transitMount, effectiveEncryptionKey, secretValue)
				if err != nil {
					log.Fatalf("transit encrypt: %v", err)
				}
				finalData[*key] = ciphertext
			} else {
				finalData[*key] = string(secretValue)
			}
		} else {
			// Single value storage (backward compatibility)
			if useEncryption {
				ciphertext, err := transitEncrypt(client, *transitMount, effectiveEncryptionKey, secretValue)
				if err != nil {
					log.Fatalf("transit encrypt: %v", err)
				}
				finalData = map[string]interface{}{"ciphertext": ciphertext}
			} else {
				finalData = map[string]interface{}{"value": string(secretValue)}
			}
		}
	}

	if err := kvv2Put(client, *kvMount, *kvPath, finalData); err != nil {
		log.Fatalf("kv put: %v", err)
	}

	encryptionStatus := "plaintext"
	if useEncryption {
		encryptionStatus = "encrypted"
	}
	secretsCount := len(finalData)
	if *key != "" {
		fmt.Printf("Updated key '%s' as %s: %s/%s\n", *key, encryptionStatus, *kvMount, *kvPath)
	} else {
		fmt.Printf("Stored/updated %d secret(s) as %s: %s/%s\n", secretsCount, encryptionStatus, *kvMount, *kvPath)
	}
}

// --------------- Subcommand: get ---------------

func cmdGet(args []string) {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	kvMount := fs.String("kv-mount", "kv", "KV v2 mount path")
	kvPath := fs.String("path", "", "KV path to retrieve secret")
	transitMount := fs.String("transit-mount", "transit", "Transit mount path")
	encryptionKey := fs.String("encryption-key", "", "Transit encryption key name (required for encrypted secrets)")
	key := fs.String("key", "", "Specific key to retrieve (for multi-value secrets)")
	outputJson := fs.Bool("json", false, "Output as JSON format")
	fs.Parse(args)

	if *kvPath == "" {
		fs.Usage()
		log.Fatal("--path is required")
	}

	// Get encryption key from flag or environment
	effectiveEncryptionKey := *encryptionKey
	if effectiveEncryptionKey == "" {
		effectiveEncryptionKey = os.Getenv("ENCRYPTION_KEY")
	}

	client := mustVaultClientFromEnv()

	// Get from KV
	data, err := kvv2GetData(client, *kvMount, *kvPath)
	if err != nil {
		log.Fatalf("kv get: %v", err)
	}

	// Check if this is encrypted multi-value data (all values start with "vault:v")
	isEncryptedMultiValue := false
	for _, v := range data {
		if str, ok := v.(string); ok && strings.HasPrefix(str, "vault:v") {
			isEncryptedMultiValue = true
			break
		}
	}

	// Try to get single encrypted data first
	ciphertext, hasCiphertext := data["ciphertext"].(string)
	if hasCiphertext && ciphertext != "" {
		// Single encrypted data - requires key
		if effectiveEncryptionKey == "" {
			log.Fatal("--encryption-key is required for encrypted secrets")
		}
		plaintext, err := transitDecrypt(client, *transitMount, effectiveEncryptionKey, ciphertext)
		if err != nil {
			log.Fatalf("transit decrypt: %v", err)
		}
		fmt.Print(string(plaintext))
		return
	}

	// Handle encrypted multi-value data
	if isEncryptedMultiValue {
		if effectiveEncryptionKey == "" {
			log.Fatal("--encryption-key is required for encrypted secrets")
		}
		
		decryptedData := make(map[string]interface{})
		for k, v := range data {
			if ciphertext, ok := v.(string); ok && strings.HasPrefix(ciphertext, "vault:v") {
				plaintext, err := transitDecrypt(client, *transitMount, effectiveEncryptionKey, ciphertext)
				if err != nil {
					log.Fatalf("decrypt %s: %v", k, err)
				}
				decryptedData[k] = string(plaintext)
			} else {
				decryptedData[k] = v
			}
		}
		
		// Handle output for decrypted multi-value data
		if *key != "" {
			value, ok := decryptedData[*key]
			if !ok {
				log.Fatalf("key %q not found", *key)
			}
			fmt.Print(value)
		} else if *outputJson {
			outputJSON(decryptedData)
		} else {
			// Output as env format
			for k, v := range decryptedData {
				fmt.Printf("%s=%v\n", k, v)
			}
		}
		return
	}

	// Handle plaintext data (single value or multiple values)
	if *key != "" {
		// Get specific key
		value, ok := data[*key]
		if !ok {
			log.Fatalf("key %q not found", *key)
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
		if *outputJson {
			outputJSON(data)
		} else {
			// Output as env format
			for k, v := range data {
				fmt.Printf("%s=%v\n", k, v)
			}
		}
	}
}

// --------------- Subcommand: env ---------------

func cmdEnv(args []string) {
	fs := flag.NewFlagSet("env", flag.ExitOnError)
	kvMount := fs.String("kv-mount", "kv", "KV v2 mount path")
	transitMount := fs.String("transit-mount", "transit", "Transit mount path")
	encryptionKey := fs.String("encryption-key", "", "Transit encryption key name")
	configFile := fs.String("config", "", "YAML config file with secret definitions")
	outputFile := fs.String("output", ".env", "Output .env file")
	fs.Parse(args)

	if *configFile == "" {
		fs.Usage()
		log.Fatal("--config is required")
	}

	// Get encryption key from flag or environment
	effectiveEncryptionKey := *encryptionKey
	if effectiveEncryptionKey == "" {
		effectiveEncryptionKey = os.Getenv("ENCRYPTION_KEY")
	}

	// Load config
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	client := mustVaultClientFromEnv()

	var envLines []string

	for _, secret := range config.Secrets {
		if secret.EnvVar == "" || secret.KVPath == "" {
			log.Printf("skipping invalid secret entry: %s", secret.Name)
			continue
		}

		// Get secret from KV
		data, err := kvv2GetData(client, nonEmpty(*kvMount, config.KV.Mount, "kv"), secret.KVPath)
		if err != nil {
			if secret.Required {
				log.Fatalf("failed to get required secret %s: %v", secret.Name, err)
			}
			log.Printf("warning: failed to get secret %s: %v", secret.Name, err)
			continue
		}

		ciphertext, ok := data["ciphertext"].(string)
		if !ok || ciphertext == "" {
			if secret.Required {
				log.Fatalf("no ciphertext found for required secret %s", secret.Name)
			}
			log.Printf("warning: no ciphertext found for secret %s", secret.Name)
			continue
		}

		// Decrypt
		encKeyForDecrypt := nonEmpty(effectiveEncryptionKey, config.Transit.Key, "")
		if encKeyForDecrypt == "" {
			if secret.Required {
				log.Fatalf("encryption key required for encrypted secret %s", secret.Name)
			}
			log.Printf("warning: no encryption key available for secret %s", secret.Name)
			continue
		}
		plaintext, err := transitDecrypt(client, nonEmpty(*transitMount, config.Transit.Mount, "transit"), encKeyForDecrypt, ciphertext)
		if err != nil {
			if secret.Required {
				log.Fatalf("failed to decrypt required secret %s: %v", secret.Name, err)
			}
			log.Printf("warning: failed to decrypt secret %s: %v", secret.Name, err)
			continue
		}

		// Add to env format
		envLines = append(envLines, fmt.Sprintf("%s=%s", secret.EnvVar, string(plaintext)))
	}

	// Write to file
	content := strings.Join(envLines, "\n")
	if len(envLines) > 0 {
		content += "\n" // Add final newline
	}

	if err := os.WriteFile(*outputFile, []byte(content), 0600); err != nil {
		log.Fatalf("write output file: %v", err)
	}

	fmt.Printf("Generated %s with %d secrets\n", *outputFile, len(envLines))
}

// --------------- Subcommand: sync ---------------

func cmdSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	configFile := fs.String("config", "vault-env.yaml", "YAML config file")
	outputFile := fs.String("output", ".env", "Output .env file")
	fs.Parse(args)

	// Load config
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if config.Transit.Key == "" {
		log.Fatal("config.transit.key is required")
	}

	client := mustVaultClientWithOverrides(
		config.Vault.Addr,
		config.Vault.Namespace,
		config.Vault.CACert,
		config.Vault.SkipVerify,
	)

	kvMount := nonEmpty("", config.KV.Mount, "kv")
	transitMount := nonEmpty("", config.Transit.Mount, "transit")

	var envLines []string

	for _, secret := range config.Secrets {
		if secret.EnvVar == "" || secret.KVPath == "" {
			log.Printf("skipping invalid secret entry: %s", secret.Name)
			continue
		}

		// Get secret from KV
		data, err := kvv2GetData(client, kvMount, secret.KVPath)
		if err != nil {
			if secret.Required {
				log.Fatalf("failed to get required secret %s: %v", secret.Name, err)
			}
			log.Printf("warning: failed to get secret %s: %v", secret.Name, err)
			continue
		}

		ciphertext, ok := data["ciphertext"].(string)
		if !ok || ciphertext == "" {
			if secret.Required {
				log.Fatalf("no ciphertext found for required secret %s", secret.Name)
			}
			log.Printf("warning: no ciphertext found for secret %s", secret.Name)
			continue
		}

		// Decrypt
		plaintext, err := transitDecrypt(client, transitMount, config.Transit.Key, ciphertext)
		if err != nil {
			if secret.Required {
				log.Fatalf("failed to decrypt required secret %s: %v", secret.Name, err)
			}
			log.Printf("warning: failed to decrypt secret %s: %v", secret.Name, err)
			continue
		}

		// Add to env format
		envLines = append(envLines, fmt.Sprintf("%s=%s", secret.EnvVar, string(plaintext)))
	}

	// Write to file
	content := strings.Join(envLines, "\n")
	if len(envLines) > 0 {
		content += "\n" // Add final newline
	}

	if err := os.WriteFile(*outputFile, []byte(content), 0600); err != nil {
		log.Fatalf("write output file: %v", err)
	}

	fmt.Printf("Synced %s with %d secrets\n", *outputFile, len(envLines))
}

// --------------- Vault helpers ---------------

func mustVaultClientFromEnv() *vaultapi.Client {
	addr := os.Getenv("VAULT_ADDR")
	ns := os.Getenv("VAULT_NAMESPACE")
	cacert := os.Getenv("VAULT_CACERT")
	skip := os.Getenv("VAULT_SKIP_VERIFY") == "1" || strings.EqualFold(os.Getenv("VAULT_SKIP_VERIFY"), "true")
	return mustVaultClientWithOverrides(addr, ns, cacert, skip)
}

func mustVaultClientWithOverrides(addr, ns, cacert string, skipVerify bool) *vaultapi.Client {
	conf := vaultapi.DefaultConfig()
	if addr != "" {
		conf.Address = addr
	}
	if cacert != "" || skipVerify {
		_ = conf.ConfigureTLS(&vaultapi.TLSConfig{CACert: cacert, Insecure: skipVerify})
	}

	// Set reasonable timeout
	conf.Timeout = 15 * time.Second

	client, err := vaultapi.NewClient(conf)
	if err != nil {
		log.Fatalf("vault client: %v", err)
	}

	if ns != "" {
		client.SetNamespace(ns)
	}

	tok := os.Getenv("VAULT_TOKEN")
	if tok == "" {
		log.Fatal("VAULT_TOKEN is required in environment")
	}
	client.SetToken(tok)

	// Configure TLS properly
	if tr, ok := conf.HttpClient.Transport.(*http.Transport); ok && tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return client
}

func transitEncrypt(client *vaultapi.Client, transitMount, keyName string, plaintext []byte) (string, error) {
	if keyName == "" {
		return "", errors.New("transit key name required")
	}

	b64 := base64.StdEncoding.EncodeToString(plaintext)
	path := fmt.Sprintf("%s/encrypt/%s", strings.TrimSuffix(transitMount, "/"), keyName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	secret, err := client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"plaintext": b64,
	})
	if err != nil {
		return "", err
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok || ciphertext == "" {
		return "", errors.New("ciphertext missing in transit response")
	}

	return ciphertext, nil
}

func transitDecrypt(client *vaultapi.Client, transitMount, keyName, ciphertext string) ([]byte, error) {
	if keyName == "" {
		return nil, errors.New("transit key name required")
	}

	path := fmt.Sprintf("%s/decrypt/%s", strings.TrimSuffix(transitMount, "/"), keyName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	secret, err := client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"ciphertext": ciphertext,
	})
	if err != nil {
		return nil, err
	}

	b64, ok := secret.Data["plaintext"].(string)
	if !ok || b64 == "" {
		return nil, errors.New("plaintext missing in transit response")
	}

	dec, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// KV v2 helpers
func kvv2Put(client *vaultapi.Client, mount, path string, data map[string]interface{}) error {
	apiPath := fmt.Sprintf("%s/data/%s", strings.TrimSuffix(mount, "/"), strings.TrimPrefix(path, "/"))
	payload := map[string]interface{}{"data": data}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := client.Logical().WriteWithContext(ctx, apiPath, payload)
	return err
}

func kvv2GetData(client *vaultapi.Client, mount, path string) (map[string]interface{}, error) {
	apiPath := fmt.Sprintf("%s/data/%s", strings.TrimSuffix(mount, "/"), strings.TrimPrefix(path, "/"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	secret, err := client.Logical().ReadWithContext(ctx, apiPath)
	if err != nil {
		return nil, err
	}

	if secret == nil || secret.Data == nil {
		return nil, errors.New("no data returned")
	}

	inner, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("unexpected kv v2 format: missing 'data' field")
	}

	return inner, nil
}

// --------------- Utils ---------------

func loadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func nonEmpty(override, configVal, defaultVal string) string {
	if override != "" {
		return override
	}
	if configVal != "" {
		return configVal
	}
	return defaultVal
}

// loadEnvFile loads a .env file and returns encrypted/plaintext data map
func loadEnvFile(path string, client *vaultapi.Client, transitMount, keyName string, useEncryption bool) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s", i+1, line)
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		
		if useEncryption {
			ciphertext, err := transitEncrypt(client, transitMount, keyName, []byte(value))
			if err != nil {
				return nil, fmt.Errorf("encrypt %s: %w", key, err)
			}
			data[key] = ciphertext
		} else {
			data[key] = value
		}
	}
	
	return data, nil
}

// outputJSON outputs data as JSON format
func outputJSON(data map[string]interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("marshal json: %v", err)
	}
	fmt.Println(string(jsonData))
}

// isEncryptedSingleValue checks if data contains a single encrypted value
func isEncryptedSingleValue(data map[string]interface{}) bool {
	if len(data) != 1 {
		return false
	}
	ciphertext, ok := data["ciphertext"].(string)
	return ok && strings.HasPrefix(ciphertext, "vault:v")
}

// isPlaintextSingleValue checks if data contains a single plaintext value
func isPlaintextSingleValue(data map[string]interface{}) bool {
	if len(data) != 1 {
		return false
	}
	_, hasValue := data["value"]
	return hasValue
}
