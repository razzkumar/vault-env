# vlt Authentication Methods

vlt supports multiple authentication methods with Vault, with automatic detection based on available credentials.

## Supported Authentication Methods

### 1. Token Authentication (Default)
The traditional token-based authentication.

```bash
export VAULT_ADDR="https://vault.example.com:8200"
export VAULT_TOKEN="hvs.xxxxxxxxxxxxx"

vlt get --path secrets/app
```

### 2. AppRole Authentication  
Uses role-based authentication with role_id and secret_id.

```bash
export VAULT_ADDR="https://vault.example.com:8200"
export VAULT_ROLE_ID="00275ac3-734f-49fc-0f46-5e9a76fbf304"
export VAULT_SECRET_ID="282cb405-42e9-c709-bd9f-030998e3f8e8"

# Auth method is auto-detected
vlt get --path secrets/app

# Or explicitly specify
export VAULT_AUTH_METHOD="approle"
vlt get --path secrets/app
```

### 3. GitHub Authentication
Uses GitHub personal access token for authentication.

```bash
export VAULT_ADDR="https://vault.example.com:8200"  
export VAULT_GITHUB_TOKEN="ghp_xxxxxxxxxxxxx"

vlt get --path secrets/app
```

### 4. Kubernetes Authentication
Uses Kubernetes service account for authentication (typically in pods).

```bash
export VAULT_ADDR="https://vault.example.com:8200"
export VAULT_K8S_ROLE="my-app-role"

# Optional - defaults shown
export VAULT_K8S_JWT_PATH="/var/run/secrets/kubernetes.io/serviceaccount/token"
export VAULT_K8S_AUTH_PATH="kubernetes"

vlt get --path secrets/app
```

## Authentication Priority

vlt automatically detects the authentication method in this priority order:

1. **Token** - if `VAULT_TOKEN` is provided
2. **AppRole** - if both `VAULT_ROLE_ID` and `VAULT_SECRET_ID` are provided
3. **GitHub** - if `VAULT_GITHUB_TOKEN` is provided
4. **Kubernetes** - if `VAULT_K8S_ROLE` is provided

You can override auto-detection by setting `VAULT_AUTH_METHOD` explicitly.

## CLI Flags

All environment variables can also be set via CLI flags:

```bash
vlt --vault-addr https://vault.example.com:8200 \
          --vault-role-id 00275ac3-734f-49fc-0f46-5e9a76fbf304 \
          --vault-secret-id 282cb405-42e9-c709-bd9f-030998e3f8e8 \
          get --path secrets/app
```

## Setting up AppRole in Vault

```bash
# Enable AppRole auth method
vault auth enable approle

# Create a role  
vault write auth/approle/role/vlt-app \
    token_policies="vlt-policy" \
    token_ttl=1h \
    token_max_ttl=4h

# Get role ID
vault read auth/approle/role/vlt-app/role-id

# Generate secret ID  
vault write -f auth/approle/role/vlt-app/secret-id
```

## Setting up GitHub Auth in Vault

```bash
# Enable GitHub auth method
vault auth enable github

# Configure GitHub organization
vault write auth/github/config organization=myorg

# Map GitHub team to policies
vault write auth/github/map/teams/developers value=vlt-policy
```

## Setting up Kubernetes Auth in Vault

```bash
# Enable Kubernetes auth method
vault auth enable kubernetes

# Configure Kubernetes auth
vault write auth/kubernetes/config \
    token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
    kubernetes_host="https://kubernetes.default.svc:443" \
    kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt

# Create a role
vault write auth/kubernetes/role/vlt-app \
    bound_service_account_names=vlt \
    bound_service_account_namespaces=default \
    policies=vlt-policy \
    ttl=24h
```