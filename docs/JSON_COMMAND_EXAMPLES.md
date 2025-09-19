# JSON Command Examples

The new `vault-env json` command allows you to encrypt and convert .env files to JSON format.

## Usage

```bash
# Basic usage - output plaintext JSON from default .env file
vault-env json

# Output plaintext JSON from specific file
vault-env json example.env

# Output encrypted JSON (requires Vault connection and encryption key)
VAULT_ADDR=https://vault.example.com \
VAULT_TOKEN=hvs.xxx \
vault-env json --encryption-key mykey

# Output encrypted JSON from specific file
VAULT_ADDR=https://vault.example.com \
VAULT_TOKEN=hvs.xxx \
vault-env json --encryption-key mykey example.env

# Use alias
vault-env j example.env
```

## Examples

### Plaintext Output (No Vault Connection Needed)
```bash
$ cat .env
DATABASE_URL=postgresql://user:password@localhost:5432/mydb
API_KEY=sk-1234567890abcdef
DEBUG=true
PORT=3000

$ vault-env json
{
  "API_KEY": "sk-1234567890abcdef",
  "DATABASE_URL": "postgresql://user:password@localhost:5432/mydb",
  "DEBUG": "true",
  "PORT": "3000"
}
```

### Encrypted Output (Requires Vault Connection)
```bash
$ VAULT_ADDR=https://vault.example.com \
  VAULT_TOKEN=hvs.xxx \
  vault-env json --encryption-key mykey
{
  "API_KEY": "vault:v1:encrypted_api_key_data...",
  "DATABASE_URL": "vault:v1:encrypted_database_url...",
  "DEBUG": "vault:v1:encrypted_debug_flag...",
  "PORT": "vault:v1:encrypted_port_number..."
}
```

## Use Cases

1. **Development**: Convert .env files to JSON for configuration management
2. **Security**: Encrypt sensitive environment variables before storing in configuration systems
3. **CI/CD**: Transform environment configurations for different deployment systems
4. **Backup**: Create JSON snapshots of environment configurations

## Notes

- When no `--encryption-key` is provided, the command outputs plaintext JSON and doesn't require a Vault connection
- When `--encryption-key` is specified, the command requires Vault connection and encrypts all values using Transit encryption
- The default file is `.env` if no file argument is provided
- Supports all existing Vault authentication methods (token, AppRole, GitHub, Kubernetes)