# Dependency Management

## Current Status (Updated for Go 1.25.1)

### Major Dependencies
- **HashiCorp Vault API**: `v1.21.0` (latest)
- **urfave/cli**: `v2.27.7` (latest) 
- **joho/godotenv**: `v1.5.1` (latest)
- **Go Version**: `1.25.1`

### Dependency Conflict Resolution

When updating to the latest versions, you may encounter this error:
```
go: github.com/hashicorp/vault/api@v1.21.0 requires github.com/hashicorp/hcl@v1.0.1-vault-7, 
not github.com/hashicorp/hcl@v1.0.0
```

#### Root Cause
The HashiCorp Vault API requires a specific fork of the HCL library (`v1.0.1-vault-7`) that conflicts with the upstream HCL library.

#### Resolution Strategy

**❌ Don't do this:**
```bash
go get -u all  # This tries to update everything to latest, causing conflicts
```

**✅ Do this instead:**
```bash
# Update specific packages first
go get -u github.com/urfave/cli/v2 github.com/joho/godotenv

# Then update Vault API (let it manage its own dependencies)
go get -u github.com/hashicorp/vault/api@latest

# Clean up
go mod tidy
```

#### Why This Works
- Vault API is allowed to select its required HCL version (`v1.0.1-vault-7`)
- Other dependencies are updated to their latest compatible versions
- Go's module system resolves the dependency graph correctly

### Verification Commands

```bash
# Check current versions
go list -m all | grep -E "(vault|hcl|cli|godotenv)"

# Verify build works
make build

# Test functionality
./vlt --version
./vlt --help
```

### Security Considerations

- All dependencies are at their latest compatible versions
- The Vault API fork of HCL is actively maintained by HashiCorp
- Regular dependency updates should follow the resolution strategy above

### Future Updates

When updating dependencies in the future:

1. **For non-Vault dependencies**: Update freely with `go get -u <package>`
2. **For Vault API**: Let it manage its own dependency versions
3. **Always run**: `go mod tidy` after updates
4. **Always test**: Build and functionality after updates

### Automated Dependency Management

Consider using tools like:
- `go mod tidy` - Clean up dependencies
- `go list -m -u all` - Check for available updates  
- `go mod graph` - View dependency graph
- IDE plugins for dependency management

---

**Last Updated**: January 2025 for Go 1.25.1