# Shell Completion Installation Guide

The `vault-env` CLI supports auto-completion for bash, zsh, fish, and PowerShell shells.

## Quick Installation

### Fish Shell (macOS/Linux)
```bash
# Generate and install completion
vault-env completion fish > ~/.config/fish/completions/vault-env.fish

# Restart your fish shell or run:
source ~/.config/fish/completions/vault-env.fish
```

### Bash (macOS/Linux)
```bash
# System-wide (requires sudo)
vault-env completion bash | sudo tee /etc/bash_completion.d/vault-env

# User-only
mkdir -p ~/.bash_completion.d
vault-env completion bash > ~/.bash_completion.d/vault-env

# Add to your ~/.bashrc or ~/.bash_profile:
echo 'source ~/.bash_completion.d/vault-env' >> ~/.bashrc
```

### Zsh (macOS/Linux)
```bash
# System-wide (requires sudo)
vault-env completion zsh | sudo tee /usr/local/share/zsh/site-functions/_vault-env

# User-only
mkdir -p ~/.zsh/completions
vault-env completion zsh > ~/.zsh/completions/_vault-env

# Add to your ~/.zshrc:
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -U compinit && compinit' >> ~/.zshrc
```

### PowerShell (Windows/macOS/Linux)
```powershell
# Generate completion script
vault-env completion powershell > vault-env-completion.ps1

# Add to your PowerShell profile:
# Get profile location: $PROFILE
Add-Content $PROFILE ". path\to\vault-env-completion.ps1"
```

## What You Get

Once installed, you'll have intelligent completion for:

### Commands and Aliases
- `vault-env <TAB>` → `put`, `get`, `env`, `sync`, `completion`, `help`
- `vault-env p<TAB>` → `put`
- `vault-env comp<TAB>` → `completion`

### Command Options
- `vault-env put --<TAB>` → `--path`, `--encryption-key`, `--key`, `--value`, etc.
- `vault-env get --<TAB>` → `--path`, `--encryption-key`, `--key`, `--json`, etc.

### Shell-Specific Arguments
- `vault-env completion <TAB>` → `bash`, `zsh`, `fish`, `powershell`

### File Path Completion
- `vault-env put --from-env <TAB>` → file path completion
- `vault-env put --from-file <TAB>` → file path completion
- `vault-env env --config <TAB>` → file path completion

## Testing Completion

After installation, test your completion:

### Fish
```bash
# Type this and press TAB
vault-env <TAB>
vault-env put --<TAB>
vault-env completion <TAB>
```

### Bash/Zsh
```bash
# Type this and press TAB twice
vault-env [TAB][TAB]
vault-env put --[TAB][TAB]
vault-env completion [TAB][TAB]
```

### PowerShell
```powershell
# Type this and press TAB
vault-env [TAB]
vault-env put --[TAB]
vault-env completion [TAB]
```

## Troubleshooting

### Fish
- Ensure Fish version ≥ 2.3
- Check completion file exists: `ls ~/.config/fish/completions/vault-env.fish`
- Reload completions: `source ~/.config/fish/completions/vault-env.fish`

### Bash
- Ensure bash-completion package is installed
- Check completion is sourced in your shell profile
- Verify with: `complete -p vault-env`

### Zsh
- Ensure `compinit` is called in your `.zshrc`
- Check function path includes completion directory
- Rebuild completion cache: `rm -f ~/.zcompdump; compinit`

### PowerShell
- Ensure execution policy allows script execution
- Verify profile location: `echo $PROFILE`
- Test loading: `. path\to\vault-env-completion.ps1`

## Advanced Usage

### Custom Installation Locations

You can install completions to custom locations by redirecting the output:

```bash
# Custom location
vault-env completion fish > /path/to/custom/completions/vault-env.fish

# Then source it in your shell's configuration
```

### Integration with Package Managers

If you're distributing `vault-env` via package managers, include completion files:

- **Homebrew**: Place in `share/zsh/site-functions/`, `etc/bash_completion.d/`, etc.
- **APT/DEB**: Place in `/usr/share/bash-completion/completions/`, `/usr/share/zsh/vendor-completions/`
- **RPM**: Place in `/usr/share/bash-completion/completions/`, `/usr/share/zsh/site-functions/`

---

**Enjoy enhanced productivity with intelligent command completion!** ⚡