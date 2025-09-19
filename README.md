# vlt

A professional CLI tool for managing secrets with HashiCorp Vault using optional Transit encryption, inspired by [vaultx](https://github.com/hashicorp/vault) and [teller](https://github.com/tellerops/teller).

**Version 2.0** features a complete rewrite with modern Go practices, proper project structure, and enhanced libraries.

## Architecture

```
vlt/
├── cmd/vlt/          # Main application entry point
├── pkg/
│   ├── config/             # Configuration management
│   ├── vault/              # Vault client wrapper
│   └── cli/                # CLI command definitions
├── internal/
│   ├── app/                # Application business logic
│   └── utils/              # Utility functions
└── examples/               # Example configurations
```

## Features

- **put**: Store secrets in Vault (optionally with Transit encryption)
  - Single key-value pairs
  - Multiple values from .env files
  - File content as base64-encoded values
- **get**: Retrieve and optionally decrypt secrets from Vault
  - Single values or multi-value retrieval
  - JSON or .env format output
  - Specific subkey extraction
- **env**: Generate .env file from multiple Vault secrets
- **sync**: Sync secrets from YAML config to .env file

**Encryption Options:**
- **Transit encryption (default)**: Secrets encrypted using Vault's Transit engine before storage
- **Plaintext storage**: Option to store secrets without additional encryption
- **Flexible key requirement**: Transit key only required when encryption is enabled

## Built With

- **[urfave/cli v2](https://github.com/urfave/cli)** - Modern CLI framework with advanced features
- **[joho/godotenv](https://github.com/joho/godotenv)** - Professional .env file parsing
- **[HashiCorp Vault API](https://github.com/hashicorp/vault/api)** - Official Vault Go client

## Requirements

- HashiCorp Vault server with:
  - Transit secrets engine enabled (default mount: `transit`)
  - KV v2 secrets engine enabled (default mount: `kv`) 
  - A transit encryption key created
- Go 1.21+ (for building from source)

## Installation

### From Source

```bash
git clone https://github.com/razzkumar/vlt
cd vlt
go build -o vlt
sudo mv vlt /usr/local/bin/
```

### Environment Variables

Required:
- `VAULT_ADDR` - Vault server address (e.g., `https://vault.example.com:8200`)
- `VAULT_TOKEN` - Vault authentication token

Optional:
- `VAULT_NAMESPACE` - Vault namespace
- `VAULT_CACERT` - Path to CA certificate file
- `VAULT_SKIP_VERIFY` - Skip TLS verification (`1` or `true`)

## Vault Setup

Before using vlt, you need to set up Vault with the required engines and keys:

```bash
# Enable KV v2 secrets engine
vault secrets enable -path=kv kv-v2

# Enable Transit secrets engine  
vault secrets enable transit

# Create a transit encryption key
vault write -f transit/keys/app-secrets
```

## Usage

### Store Secrets

```bash
# Store single secret with encryption (default)
vlt put --key app-secrets --path myapp/db_password --value "supersecret"

# Store single secret without encryption
vlt put --path myapp/db_password --value "supersecret" --no-encrypt

# Store from stdin
echo "supersecret" | vlt put --key app-secrets --path myapp/db_password

# Store multiple secrets from .env file
vlt put --key app-secrets --path myapp/config --from-env production.env

# Store file content as base64 (useful for SSH keys, certificates)
vlt put --key app-secrets --path myapp/ssh_key --from-file ~/.ssh/id_rsa
```

### Retrieve Secrets

```bash
# Get single encrypted secret
vlt get --key app-secrets --path myapp/db_password

# Get multiple secrets as JSON
vlt get --key app-secrets --path myapp/config --json

# Get multiple secrets as .env format
vlt get --key app-secrets --path myapp/config

# Get specific value from multi-value secret
vlt get --key app-secrets --path myapp/config --subkey AWS_ACCESS_KEY_ID

# Get plaintext secret (no key needed)
vlt get --path myapp/plaintext_config --subkey EMAIL_FROM

# Use in environment variable
export DB_PASSWORD=$(vlt get --key app-secrets --path myapp/db_password)
```

### Generate .env File

Create a configuration file (see `example-config.yaml`):

```yaml
---
version: 1
vault:
  addr: "https://vault.example.com:8200"
transit:
  mount: "transit"
  key: "app-secrets"
kv:
  mount: "kv"
secrets:
  - name: "Database Password"
    kv_path: "myapp/prod/db_password"
    env_var: "DB_PASSWORD"
    required: true
  - name: "API Key" 
    kv_path: "myapp/prod/api_key"
    env_var: "API_KEY"
    required: true
```

Then generate the .env file:

```bash
# Generate .env from config
vlt sync --config secrets.yaml --output .env

# Or use the env command with CLI flags
vlt env --key app-secrets --config secrets.yaml --output .env
```

## Commands

### `put`

Store a secret in Vault with Transit encryption.

```bash
vlt put [flags]

Flags:
  --key string            Transit key name (required)
  --path string           KV path to store secret (required)  
  --value string          Secret value (or use stdin)
  --kv-mount string       KV v2 mount path (default "kv")
  --transit-mount string  Transit mount path (default "transit")
```

### `get`

Retrieve and decrypt a secret from Vault.

```bash
vlt get [flags]

Flags:
  --key string            Transit key name (required)
  --path string           KV path to retrieve secret (required)
  --kv-mount string       KV v2 mount path (default "kv") 
  --transit-mount string  Transit mount path (default "transit")
```

### `env` 

Generate .env file from multiple Vault secrets using a config file.

```bash
vlt env [flags]

Flags:
  --key string            Transit key name (required)
  --config string         YAML config file with secret definitions (required)
  --output string         Output .env file (default ".env")
  --kv-mount string       KV v2 mount path (default "kv")
  --transit-mount string  Transit mount path (default "transit")
```

### `sync`

Sync secrets from YAML config to .env file. Uses configuration from the YAML file for all settings.

```bash
vlt sync [flags]

Flags:
  --config string         YAML config file (default "vlt.yaml")
  --output string         Output .env file (default ".env")
```

## Configuration File

The YAML configuration file supports the following structure:

```yaml
version: 1
vault:
  addr: "https://vault.example.com:8200"  # optional; else VAULT_ADDR env
  namespace: ""                           # optional; else VAULT_NAMESPACE env  
  skip_verify: false                      # optional; else VAULT_SKIP_VERIFY env
  ca_cert: "/etc/ssl/certs/vault-ca.pem" # optional; else VAULT_CACERT env
transit:
  mount: "transit"                        # Transit secrets engine mount
  key: "app-secrets"                      # Transit encryption key name  
kv:
  mount: "kv"                            # KV v2 secrets engine mount
secrets:
  - name: "Description"                   # Human readable name
    kv_path: "path/to/secret"            # Path in KV store
    env_var: "ENV_VAR_NAME"              # Environment variable name
    required: true                       # Fail if secret missing (default: false)
```

## Security Notes

- All secrets are encrypted using Vault's Transit engine before storage
- The `.env` file is created with `0600` permissions (owner read/write only)
- Never commit `.env` files or configuration files containing secrets to version control
- Use Vault policies to restrict access to secrets and transit keys
- Consider using short-lived tokens and token renewal for production use

## Examples

### Complete Workflow

1. Set up environment:
```bash
export VAULT_ADDR="https://vault.example.com:8200"
export VAULT_TOKEN="your-vault-token"
```

2. Store secrets:
```bash
vlt put --key app-secrets --path myapp/db_password --value "db_secret_123"
vlt put --key app-secrets --path myapp/api_key --value "api_key_456"
```

3. Create config file (`secrets.yaml`):
```yaml
version: 1
transit:
  key: "app-secrets"
secrets:
  - name: "Database Password"
    kv_path: "myapp/db_password"
    env_var: "DB_PASSWORD"
    required: true
  - name: "API Key"
    kv_path: "myapp/api_key" 
    env_var: "API_KEY"
    required: true
```

4. Generate .env file:
```bash
vlt sync --config secrets.yaml
```

5. Use in your application:
```bash
source .env
echo "DB Password: $DB_PASSWORD"
echo "API Key: $API_KEY"
```

## Comparison with Teller

While inspired by Teller, vlt is focused specifically on HashiCorp Vault with Transit encryption:

| Feature | vlt | Teller |
|---------|-----------|---------|
| Vault Support | ✅ Full | ✅ Full |
| Transit Encryption | ✅ Built-in | ❌ Not supported |
| Multiple Providers | ❌ Vault only | ✅ Many providers |
| CLI Simplicity | ✅ Minimal | ⚖️ Feature-rich |
| Config Format | YAML | YAML/HCL |

## License

MIT License - see LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable  
5. Submit a pull request

## Support

For issues and questions:
- Create an issue in the GitHub repository
- Check Vault documentation for setup and configuration help