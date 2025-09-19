package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"

	vaultcli "github.com/razzkumar/vault-env/pkg/cli"
)

func main() {
	app := &cli.App{
		Name:  "vault-env",
		Usage: "Minimal secrets management with Vault (optionally with Transit encryption)",
		Description: `vault-env is a CLI tool for managing secrets with HashiCorp Vault using optional Transit encryption.
It supports storing and retrieving single values or multiple key-value pairs, with smart merging capabilities.`,
		Version: "2.0.0",
		Authors: []*cli.Author{
			{
				Name: "vault-env contributors",
			},
		},
		Commands: vaultcli.GetCommands(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "vault-addr",
				Usage:   "Vault server address",
				EnvVars: []string{"VAULT_ADDR"},
			},
			&cli.StringFlag{
				Name:    "vault-token",
				Usage:   "Vault authentication token",
				EnvVars: []string{"VAULT_TOKEN"},
			},
			&cli.StringFlag{
				Name:    "vault-namespace",
				Usage:   "Vault namespace",
				EnvVars: []string{"VAULT_NAMESPACE"},
			},
			&cli.StringFlag{
				Name:    "encryption-key",
				Usage:   "Default transit encryption key",
				EnvVars: []string{"ENCRYPTION_KEY"},
			},
			// Auth method flags
			&cli.StringFlag{
				Name:    "vault-auth-method",
				Usage:   "Vault auth method (token, approle, github, kubernetes)",
				EnvVars: []string{"VAULT_AUTH_METHOD"},
			},
			&cli.StringFlag{
				Name:    "vault-role-id",
				Usage:   "Vault AppRole role ID",
				EnvVars: []string{"VAULT_ROLE_ID"},
			},
			&cli.StringFlag{
				Name:    "vault-secret-id",
				Usage:   "Vault AppRole secret ID",
				EnvVars: []string{"VAULT_SECRET_ID"},
			},
			&cli.StringFlag{
				Name:    "vault-github-token",
				Usage:   "GitHub personal access token for auth",
				EnvVars: []string{"VAULT_GITHUB_TOKEN"},
			},
			&cli.StringFlag{
				Name:    "vault-k8s-role",
				Usage:   "Vault Kubernetes auth role",
				EnvVars: []string{"VAULT_K8S_ROLE"},
			},
		},
		Before: func(ctx *cli.Context) error {
			// Set environment variables from flags if provided
			if addr := ctx.String("vault-addr"); addr != "" {
				os.Setenv("VAULT_ADDR", addr)
			}
			if token := ctx.String("vault-token"); token != "" {
				os.Setenv("VAULT_TOKEN", token)
			}
			if namespace := ctx.String("vault-namespace"); namespace != "" {
				os.Setenv("VAULT_NAMESPACE", namespace)
			}
			if encKey := ctx.String("encryption-key"); encKey != "" {
				os.Setenv("ENCRYPTION_KEY", encKey)
			}
			// Auth method environment variables
			if authMethod := ctx.String("vault-auth-method"); authMethod != "" {
				os.Setenv("VAULT_AUTH_METHOD", authMethod)
			}
			if roleID := ctx.String("vault-role-id"); roleID != "" {
				os.Setenv("VAULT_ROLE_ID", roleID)
			}
			if secretID := ctx.String("vault-secret-id"); secretID != "" {
				os.Setenv("VAULT_SECRET_ID", secretID)
			}
			if githubToken := ctx.String("vault-github-token"); githubToken != "" {
				os.Setenv("VAULT_GITHUB_TOKEN", githubToken)
			}
			if k8sRole := ctx.String("vault-k8s-role"); k8sRole != "" {
				os.Setenv("VAULT_K8S_ROLE", k8sRole)
			}
			return nil
		},
		UsageText: `vault-env [global options] command [command options] [arguments...]

ENVIRONMENT VARIABLES:
  VAULT_ADDR         Vault server address (required)
  VAULT_TOKEN        Vault authentication token (required for token auth)
  VAULT_NAMESPACE    Vault namespace (optional)
  VAULT_CACERT       CA certificate path (optional)
  VAULT_SKIP_VERIFY  Skip TLS verification (optional)
  ENCRYPTION_KEY     Default transit encryption key (defaults to "app-secrets" when TRANSIT=true)
  TRANSIT            Enable/disable transit encryption: true/false, 1/0, yes/no, on/off (optional)
  TRANSIT_MOUNT      Transit mount path (defaults to "transit" when TRANSIT=true)
  
  Authentication (auto-detected or explicit):
  VAULT_AUTH_METHOD  Auth method: token, approle, github, kubernetes (optional)
  
  AppRole authentication:
  VAULT_ROLE_ID      AppRole role ID (required for approle auth)
  VAULT_SECRET_ID    AppRole secret ID (required for approle auth)
  
  GitHub authentication:
  VAULT_GITHUB_TOKEN GitHub personal access token (required for github auth)
  
  Kubernetes authentication:
  VAULT_K8S_ROLE     Kubernetes auth role (required for kubernetes auth)
  VAULT_K8S_JWT_PATH Kubernetes service account token path (default: /var/run/secrets/kubernetes.io/serviceaccount/token)
  VAULT_K8S_AUTH_PATH Kubernetes auth mount path (default: kubernetes)

EXAMPLES:
  # Token authentication (default)
  VAULT_ADDR=https://vault.example.com VAULT_TOKEN=hvs.xxx vault-env get --path secrets/app
  
  # AppRole authentication (auto-detected)
  VAULT_ADDR=https://vault.example.com VAULT_ROLE_ID=xxx VAULT_SECRET_ID=yyy vault-env get --path secrets/app
  
  # GitHub authentication
  VAULT_ADDR=https://vault.example.com VAULT_GITHUB_TOKEN=ghp_xxx vault-env get --path secrets/app
  
  # Kubernetes authentication
  VAULT_ADDR=https://vault.example.com VAULT_K8S_ROLE=my-role vault-env get --path secrets/app
  
  # Store a single secret with transit encryption
  vault-env put --encryption-key mykey --path secrets/db_password --value "supersecret"
  
  # Store using environment variable for encryption key
  ENCRYPTION_KEY=mykey vault-env put --path secrets/db_password --value "supersecret"
  
  # Enable transit encryption with defaults (key="app-secrets", mount="transit")
  TRANSIT=true vault-env put --path secrets/db_password --value "supersecret"
  
  # Enable transit encryption with custom key
  TRANSIT=true ENCRYPTION_KEY=mykey vault-env put --path secrets/db_password --value "supersecret"
  
  # Store without encryption (disable transit even if key is set)
  TRANSIT=false vault-env put --path secrets/db_password --value "supersecret"
  
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
  
  # Get all secrets from config file (.env format)
  vault-env get --config secrets.yaml
  
  # Get all secrets from config file (JSON format)
  vault-env get --config secrets.yaml --json
  
  # Sync from config file
  vault-env sync --config secrets.yaml
  
  # Run command with secrets injected
  vault-env run --config secrets.yaml -- go run main.go
  
  # Convert .env file to JSON with encryption (uses defaults)
  TRANSIT=true vault-env json
  
  # Convert .env file to plaintext JSON
  vault-env json example.env
  
  # Generate shell completion
  vault-env completion fish > ~/.config/fish/completions/vault-env.fish`,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}