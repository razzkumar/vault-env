package vault

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"

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

	client.SetToken(cfg.Token)

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