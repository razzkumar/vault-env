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
	fmt.Print(`vault-env - Minimal secrets management with Vault Transit encryption

COMMANDS:
  put       Store a secret in Vault with Transit encryption
  get       Retrieve and decrypt a secret from Vault
  env       Generate .env file from multiple Vault secrets
  sync      Sync secrets from YAML config to .env file

ENVIRONMENT:
  VAULT_ADDR, VAULT_TOKEN (required)
  VAULT_NAMESPACE, VAULT_CACERT, VAULT_SKIP_VERIFY (optional)

EXAMPLES:
  # Store a secret
  vault-env put --key mykey --path secrets/db_password --value "supersecret"
  echo "supersecret" | vault-env put --key mykey --path secrets/db_password

  # Retrieve a secret
  vault-env get --key mykey --path secrets/db_password

  # Generate .env from multiple secrets
  vault-env env --key mykey --config secrets.yaml --output .env

  # Sync from config file
  vault-env sync --config secrets.yaml
`)
}

// --------------- Subcommand: put ---------------

func cmdPut(args []string) {
	fs := flag.NewFlagSet("put", flag.ExitOnError)
	kvMount := fs.String("kv-mount", "kv", "KV v2 mount path")
	kvPath := fs.String("path", "", "KV path to store secret")
	transitMount := fs.String("transit-mount", "transit", "Transit mount path")
	keyName := fs.String("key", "", "Transit key name")
	value := fs.String("value", "", "Secret value (or use stdin)")
	fs.Parse(args)

	if *kvPath == "" || *keyName == "" {
		fs.Usage()
		log.Fatal("--path and --key are required")
	}

	client := mustVaultClientFromEnv()

	// Read secret value
	var secretValue []byte
	var err error

	if *value != "" {
		secretValue = []byte(*value)
	} else {
		// Read from stdin
		secretValue, err = io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("read stdin: %v", err)
		}
	}

	if len(secretValue) == 0 {
		log.Fatal("no secret value provided")
	}

	// Remove trailing newline if reading from stdin
	if *value == "" && len(secretValue) > 0 && secretValue[len(secretValue)-1] == '\n' {
		secretValue = secretValue[:len(secretValue)-1]
	}

	// Encrypt with Transit
	ciphertext, err := transitEncrypt(client, *transitMount, *keyName, secretValue)
	if err != nil {
		log.Fatalf("transit encrypt: %v", err)
	}

	// Store in KV
	data := map[string]interface{}{
		"ciphertext": ciphertext,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	if err := kvv2Put(client, *kvMount, *kvPath, data); err != nil {
		log.Fatalf("kv put: %v", err)
	}

	fmt.Printf("Secret stored: %s/%s\n", *kvMount, *kvPath)
}

// --------------- Subcommand: get ---------------

func cmdGet(args []string) {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	kvMount := fs.String("kv-mount", "kv", "KV v2 mount path")
	kvPath := fs.String("path", "", "KV path to retrieve secret")
	transitMount := fs.String("transit-mount", "transit", "Transit mount path")
	keyName := fs.String("key", "", "Transit key name")
	fs.Parse(args)

	if *kvPath == "" || *keyName == "" {
		fs.Usage()
		log.Fatal("--path and --key are required")
	}

	client := mustVaultClientFromEnv()

	// Get from KV
	data, err := kvv2GetData(client, *kvMount, *kvPath)
	if err != nil {
		log.Fatalf("kv get: %v", err)
	}

	ciphertext, ok := data["ciphertext"].(string)
	if !ok || ciphertext == "" {
		log.Fatalf("no ciphertext found at %s/%s", *kvMount, *kvPath)
	}

	// Decrypt with Transit
	plaintext, err := transitDecrypt(client, *transitMount, *keyName, ciphertext)
	if err != nil {
		log.Fatalf("transit decrypt: %v", err)
	}

	fmt.Print(string(plaintext))
}

// --------------- Subcommand: env ---------------

func cmdEnv(args []string) {
	fs := flag.NewFlagSet("env", flag.ExitOnError)
	kvMount := fs.String("kv-mount", "kv", "KV v2 mount path")
	transitMount := fs.String("transit-mount", "transit", "Transit mount path")
	keyName := fs.String("key", "", "Transit key name")
	configFile := fs.String("config", "", "YAML config file with secret definitions")
	outputFile := fs.String("output", ".env", "Output .env file")
	fs.Parse(args)

	if *configFile == "" || *keyName == "" {
		fs.Usage()
		log.Fatal("--config and --key are required")
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
		plaintext, err := transitDecrypt(client, nonEmpty(*transitMount, config.Transit.Mount, "transit"), nonEmpty(*keyName, config.Transit.Key, ""), ciphertext)
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
