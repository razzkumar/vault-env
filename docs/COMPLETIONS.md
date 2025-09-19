# Shell Completion Installation Guide

The `vlt` CLI supports auto-completion for bash, zsh, fish, and PowerShell shells.

## Quick Installation

### Fish Shell (macOS/Linux)
```bash
# Generate and install completion
vlt completion fish > ~/.config/fish/completions/vlt.fish

# Restart your fish shell or run:
source ~/.config/fish/completions/vlt.fish
```

### Bash (macOS/Linux)
```bash
# System-wide (requires sudo)
vlt completion bash | sudo tee /etc/bash_completion.d/vlt

# User-only
mkdir -p ~/.bash_completion.d
vlt completion bash > ~/.bash_completion.d/vlt

# Add to your ~/.bashrc or ~/.bash_profile:
echo 'source ~/.bash_completion.d/vlt' >> ~/.bashrc
```

### Zsh (macOS/Linux)
```bash
# System-wide (requires sudo)
vlt completion zsh | sudo tee /usr/local/share/zsh/site-functions/_vlt

# User-only
mkdir -p ~/.zsh/completions
vlt completion zsh > ~/.zsh/completions/_vlt

# Add to your ~/.zshrc:
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -U compinit && compinit' >> ~/.zshrc
```

### PowerShell (Windows/macOS/Linux)
```powershell
# Generate completion script
vlt completion powershell > vlt-completion.ps1

# Add to your PowerShell profile:
# Get profile location: $PROFILE
Add-Content $PROFILE ". path\to\vlt-completion.ps1"
```

## What You Get

Once installed, you'll have intelligent completion for:

### Commands and Aliases
- `vlt <TAB>` → `put`, `get`, `env`, `sync`, `completion`, `help`
- `vlt p<TAB>` → `put`
- `vlt comp<TAB>` → `completion`

### Command Options
- `vlt put --<TAB>` → `--path`, `--encryption-key`, `--key`, `--value`, etc.
- `vlt get --<TAB>` → `--path`, `--encryption-key`, `--key`, `--json`, etc.

### Shell-Specific Arguments
- `vlt completion <TAB>` → `bash`, `zsh`, `fish`, `powershell`

### File Path Completion
- `vlt put --from-env <TAB>` → file path completion
- `vlt put --from-file <TAB>` → file path completion
- `vlt env --config <TAB>` → file path completion

## Testing Completion

After installation, test your completion:

### Fish
```bash
# Type this and press TAB
vlt <TAB>
vlt put --<TAB>
vlt completion <TAB>
```

### Bash/Zsh
```bash
# Type this and press TAB twice
vlt [TAB][TAB]
vlt put --[TAB][TAB]
vlt completion [TAB][TAB]
```

### PowerShell
```powershell
# Type this and press TAB
vlt [TAB]
vlt put --[TAB]
vlt completion [TAB]
```

## Troubleshooting

### Fish
- Ensure Fish version ≥ 2.3
- Check completion file exists: `ls ~/.config/fish/completions/vlt.fish`
- Reload completions: `source ~/.config/fish/completions/vlt.fish`

### Bash
- Ensure bash-completion package is installed
- Check completion is sourced in your shell profile
- Verify with: `complete -p vlt`

### Zsh
- Ensure `compinit` is called in your `.zshrc`
- Check function path includes completion directory
- Rebuild completion cache: `rm -f ~/.zcompdump; compinit`

### PowerShell
- Ensure execution policy allows script execution
- Verify profile location: `echo $PROFILE`
- Test loading: `. path\to\vlt-completion.ps1`

## Advanced Usage

### Custom Installation Locations

You can install completions to custom locations by redirecting the output:

```bash
# Custom location
vlt completion fish > /path/to/custom/completions/vlt.fish

# Then source it in your shell's configuration
```

### Integration with Package Managers

If you're distributing `vlt` via package managers, include completion files:

- **Homebrew**: Place in `share/zsh/site-functions/`, `etc/bash_completion.d/`, etc.
- **APT/DEB**: Place in `/usr/share/bash-completion/completions/`, `/usr/share/zsh/vendor-completions/`
- **RPM**: Place in `/usr/share/bash-completion/completions/`, `/usr/share/zsh/site-functions/`

---

**Enjoy enhanced productivity with intelligent command completion!** ⚡