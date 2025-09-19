# JSON Command Examples

The new `vlt json` command allows you to encrypt and convert .env files to JSON format.

## TRANSIT Environment Variable

Transit encryption is controlled by the `TRANSIT` environment variable:
- **Supported values**: `true/false`, `1/0`, `yes/no`, `on/off`, `enable/disable`, `enabled/disabled`
- **Defaults when TRANSIT=true**:
  - `ENCRYPTION_KEY` defaults to `"app-secrets"`
  - `TRANSIT_MOUNT` defaults to `"transit"`

## Usage

```bash
# Basic usage - output plaintext JSON from default .env file
vlt json

# Output plaintext JSON from specific file
vlt json example.env

# Output encrypted JSON with defaults (key="app-secrets", mount="transit")
TRANSIT=true vlt json

# Output encrypted JSON with custom key
TRANSIT=true ENCRYPTION_KEY=mykey vlt json

# Output encrypted JSON from specific file
TRANSIT=true vlt json example.env

# Disable encryption even if encryption key is set
TRANSIT=false ENCRYPTION_KEY=mykey vlt json

# Use custom transit mount
TRANSIT=true TRANSIT_MOUNT=custom-transit vlt json

# Use alias with encryption
TRANSIT=1 vlt j example.env
```

## Examples

### Plaintext Output (Default Behavior)
```bash
$ cat .env
DATABASE_URL=postgresql://user:password@localhost:5432/mydb
API_KEY=sk-1234567890abcdef
DEBUG=true
PORT=3000

$ vlt json
{
  "API_KEY": "sk-1234567890abcdef",
  "DATABASE_URL": "postgresql://user:password@localhost:5432/mydb",
  "DEBUG": "true",
  "PORT": "3000"
}
```

### Encrypted Output with Defaults (TRANSIT=true)
```bash
$ TRANSIT=true vlt json
{
  "API_KEY": "vault:v2:mHLNtaMpF6JPtUt2wkaCewy6ZRY3GRqOu/uHcqf0Dqs/h5RXQ9MzOv2nQbhPUII=",
  "DATABASE_URL": "vault:v2:sqrGPvB2NJtXWa+ZR/9rUxqQnIMH+KC5ZY7qK42I+7qDK9FSIoztgNI99h9RYbsn...",
  "DEBUG": "vault:v2:kW8ku9wS1UaxbbWcreUbR/UNxrjSbJSl1OJBAg8TuyM=",
  "PORT": "vault:v2:sA0UXkwEwcwQN1uoFJck94QUT/9E/DycbpVErOX8V/M="
}
```

### Encrypted Output with Custom Key
```bash
$ TRANSIT=true ENCRYPTION_KEY=my-custom-key vlt json
{
  "API_KEY": "vault:v1:B+lngV/IT0yvpR/Fvfy9/Bs9h2bONvHQL7yLocN3yBNZOd/B9hqs2O/Ggpa3FHE=",
  "DATABASE_URL": "vault:v1:ajfYWx8Cr+rrVtJ5ZovE4SSH4MgP28zqMj6Z7c03dGm+MESW9mW7QXxisoNH3b1M...",
  "DEBUG": "vault:v1:ZiB/SZ2FA8j5MKyHCAy7B6TFIhfwC/1Xd9JuHtx3cvU=",
  "PORT": "vault:v1:NrU/WKiJVze9EINPsUEiJQYxD85nR4GfSOplVh4EVu8="
}
```

### Overriding Encryption (TRANSIT=false)
```bash
$ TRANSIT=false ENCRYPTION_KEY=mykey vlt json
{
  "API_KEY": "sk-1234567890abcdef",
  "DATABASE_URL": "postgresql://user:password@localhost:5432/mydb",
  "DEBUG": "true",
  "PORT": "3000"
}
```

## Use Cases

1. **Development**: Convert .env files to JSON for configuration management
2. **Security**: Encrypt sensitive environment variables before storing in configuration systems
3. **CI/CD**: Transform environment configurations for different deployment systems
4. **Backup**: Create JSON snapshots of environment configurations

## Notes

- **Default behavior**: Outputs plaintext JSON without Vault connection
- **TRANSIT=true**: Enables encryption with defaults (`ENCRYPTION_KEY="app-secrets"`, `TRANSIT_MOUNT="transit"`)
- **TRANSIT=false**: Forces plaintext output even if encryption key is configured
- **Encryption key sources**: `--encryption-key` flag > `ENCRYPTION_KEY` environment variable > default (`"app-secrets"` when `TRANSIT=true`)
- **Transit mount sources**: `--transit-mount` flag > `TRANSIT_MOUNT` environment variable > default (`"transit"`)
- **File argument**: Defaults to `.env` if no file is specified
- **Vault connection**: Required only when encryption is enabled
- **Authentication**: Supports all Vault auth methods (token, AppRole, GitHub, Kubernetes)
- **Supported TRANSIT values**: `true/false`, `1/0`, `yes/no`, `on/off`, `enable/disable`, `enabled/disabled` (case-insensitive)
