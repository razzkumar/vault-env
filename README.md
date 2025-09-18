# vault-env

A minimal CLI tool for managing secrets with HashiCorp Vault using Transit encryption, inspired by [vaultx](https://github.com/hashicorp/vault) and [teller](https://github.com/tellerops/teller).

## Features

- **put**: Store secrets in Vault with Transit encryption
- **get**: Retrieve and decrypt secrets from Vault  
- **env**: Generate .env file from multiple Vault secrets
- **sync**: Sync secrets from YAML config to .env file

All secrets are encrypted using Vault's Transit secrets engine before being stored in the KV v2 engine, providing an extra layer of security.

## Requirements

- HashiCorp Vault server with:
  - Transit secrets engine enabled (default mount: `transit`)
  - KV v2 secrets engine enabled (default mount: `kv`) 
  - A transit encryption key created
- Go 1.20+ (for building from source)

## Installation

### From Source

```bash
git clone https://github.com/razzkumar/vault-env
cd vault-env
go build -o vault-env
sudo mv vault-env /usr/local/bin/
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

Before using vault-env, you need to set up Vault with the required engines and keys:

```bash
# Enable KV v2 secrets engine
vault secrets enable -path=kv kv-v2

# Enable Transit secrets engine  
vault secrets enable transit

# Create a transit encryption key
vault write -f transit/keys/app-secrets
```

## Usage

### Store a Secret

```bash
# Store from command line
vault-env put --key app-secrets --path myapp/db_password --value "supersecret"

# Store from stdin
echo "supersecret" | vault-env put --key app-secrets --path myapp/db_password

# Store from file  
cat secret.txt | vault-env put --key app-secrets --path myapp/api_key
```

### Retrieve a Secret

```bash
# Get secret to stdout
vault-env get --key app-secrets --path myapp/db_password

# Get secret and use in another command
export DB_PASSWORD=$(vault-env get --key app-secrets --path myapp/db_password)
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
vault-env sync --config secrets.yaml --output .env

# Or use the env command with CLI flags
vault-env env --key app-secrets --config secrets.yaml --output .env
```

## Commands

### `put`

Store a secret in Vault with Transit encryption.

```bash
vault-env put [flags]

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
vault-env get [flags]

Flags:
  --key string            Transit key name (required)
  --path string           KV path to retrieve secret (required)
  --kv-mount string       KV v2 mount path (default "kv") 
  --transit-mount string  Transit mount path (default "transit")
```

### `env` 

Generate .env file from multiple Vault secrets using a config file.

```bash
vault-env env [flags]

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
vault-env sync [flags]

Flags:
  --config string         YAML config file (default "vault-env.yaml")
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
vault-env put --key app-secrets --path myapp/db_password --value "db_secret_123"
vault-env put --key app-secrets --path myapp/api_key --value "api_key_456"
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
vault-env sync --config secrets.yaml
```

5. Use in your application:
```bash
source .env
echo "DB Password: $DB_PASSWORD"
echo "API Key: $API_KEY"
```

## Comparison with Teller

While inspired by Teller, vault-env is focused specifically on HashiCorp Vault with Transit encryption:

| Feature | vault-env | Teller |
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