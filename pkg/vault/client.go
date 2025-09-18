package vault

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	// Auth methods implemented directly

	"github.com/razzkumar/vault-env/pkg/config"
)

// Client wraps the Vault API client with our specific functionality
type Client struct {
	client *vaultapi.Client
	config *config.VaultConfig
}

// NewClient creates a new Vault client
func NewClient(cfg *config.VaultConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	vaultConfig := vaultapi.DefaultConfig()
	vaultConfig.Address = cfg.Addr
	vaultConfig.Timeout = time.Duration(cfg.Timeout) * time.Second

	if cfg.CACert != "" || cfg.SkipVerify {
		err := vaultConfig.ConfigureTLS(&vaultapi.TLSConfig{
			CACert:   cfg.CACert,
			Insecure: cfg.SkipVerify,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}
	}

	client, err := vaultapi.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	// Authenticate and get token
	token, err := authenticateVault(client, cfg)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	
	client.SetToken(token)

	// Configure TLS properly
	if tr, ok := vaultConfig.HttpClient.Transport.(*http.Transport); ok && tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// TransitEncrypt encrypts plaintext using Vault's Transit secrets engine
func (c *Client) TransitEncrypt(transitMount, keyName string, plaintext []byte) (string, error) {
	if keyName == "" {
		return "", errors.New("transit key name required")
	}

	b64 := base64.StdEncoding.EncodeToString(plaintext)
	path := fmt.Sprintf("%s/encrypt/%s", strings.TrimSuffix(transitMount, "/"), keyName)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	secret, err := c.client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"plaintext": b64,
	})
	if err != nil {
		return "", fmt.Errorf("transit encrypt failed: %w", err)
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok || ciphertext == "" {
		return "", errors.New("ciphertext missing in transit response")
	}

	return ciphertext, nil
}

// TransitDecrypt decrypts ciphertext using Vault's Transit secrets engine
func (c *Client) TransitDecrypt(transitMount, keyName, ciphertext string) ([]byte, error) {
	if keyName == "" {
		return nil, errors.New("transit key name required")
	}

	path := fmt.Sprintf("%s/decrypt/%s", strings.TrimSuffix(transitMount, "/"), keyName)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	secret, err := c.client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"ciphertext": ciphertext,
	})
	if err != nil {
		return nil, fmt.Errorf("transit decrypt failed: %w", err)
	}

	b64, ok := secret.Data["plaintext"].(string)
	if !ok || b64 == "" {
		return nil, errors.New("plaintext missing in transit response")
	}

	dec, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode plaintext: %w", err)
	}

	return dec, nil
}

// KVPut stores data in Vault's KV v2 secrets engine
func (c *Client) KVPut(mount, path string, data map[string]interface{}) error {
	apiPath := fmt.Sprintf("%s/data/%s", strings.TrimSuffix(mount, "/"), strings.TrimPrefix(path, "/"))
	payload := map[string]interface{}{"data": data}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	_, err := c.client.Logical().WriteWithContext(ctx, apiPath, payload)
	if err != nil {
		return fmt.Errorf("kv put failed: %w", err)
	}

	return nil
}

// KVGet retrieves data from Vault's KV v2 secrets engine
func (c *Client) KVGet(mount, path string) (map[string]interface{}, error) {
	apiPath := fmt.Sprintf("%s/data/%s", strings.TrimSuffix(mount, "/"), strings.TrimPrefix(path, "/"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	secret, err := c.client.Logical().ReadWithContext(ctx, apiPath)
	if err != nil {
		return nil, fmt.Errorf("kv get failed: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, errors.New("no data returned from vault")
	}

	inner, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("unexpected kv v2 format: missing 'data' field")
	}

	return inner, nil
}

// authenticateVault performs authentication based on the configured method
func authenticateVault(client *vaultapi.Client, cfg *config.VaultConfig) (string, error) {
	switch cfg.AuthMethod {
	case "token":
		if cfg.Token == "" {
			return "", fmt.Errorf("token is required for token auth")
		}
		return cfg.Token, nil
		
	case "approle":
		return authenticateAppRole(client, cfg)
		
	case "github":
		return authenticateGitHub(client, cfg)
		
	case "kubernetes":
		return authenticateKubernetes(client, cfg)
		
	default:
		return "", fmt.Errorf("unsupported auth method: %s", cfg.AuthMethod)
	}
}

// authenticateAppRole performs AppRole authentication
func authenticateAppRole(client *vaultapi.Client, cfg *config.VaultConfig) (string, error) {
	data := map[string]interface{}{
		"role_id":   cfg.RoleID,
		"secret_id": cfg.SecretID,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()
	
	secret, err := client.Logical().WriteWithContext(ctx, "auth/approle/login", data)
	if err != nil {
		return "", fmt.Errorf("unable to login to AppRole auth method: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return "", fmt.Errorf("no auth info was returned after login")
	}

	return secret.Auth.ClientToken, nil
}

// authenticateGitHub performs GitHub personal access token authentication
func authenticateGitHub(client *vaultapi.Client, cfg *config.VaultConfig) (string, error) {
	data := map[string]interface{}{
		"token": cfg.GitHubToken,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()
	
	secret, err := client.Logical().WriteWithContext(ctx, "auth/github/login", data)
	if err != nil {
		return "", fmt.Errorf("unable to login to GitHub auth method: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return "", fmt.Errorf("no auth info was returned after login")
	}

	return secret.Auth.ClientToken, nil
}

// authenticateKubernetes performs Kubernetes service account authentication
func authenticateKubernetes(client *vaultapi.Client, cfg *config.VaultConfig) (string, error) {
	// Read the service account token
	jwtBytes, err := os.ReadFile(cfg.K8sJWTPath)
	if err != nil {
		return "", fmt.Errorf("unable to read Kubernetes JWT token from %s: %w", cfg.K8sJWTPath, err)
	}
	jwt := strings.TrimSpace(string(jwtBytes))

	data := map[string]interface{}{
		"role": cfg.K8sRole,
		"jwt":  jwt,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()
	
	path := fmt.Sprintf("auth/%s/login", cfg.K8sAuthPath)
	secret, err := client.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		return "", fmt.Errorf("unable to login to Kubernetes auth method: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return "", fmt.Errorf("no auth info was returned after login")
	}

	return secret.Auth.ClientToken, nil
}
