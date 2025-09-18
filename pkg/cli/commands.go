package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/razzkumar/vault-env/internal/app"
)

// GetCommands returns all CLI commands
func GetCommands() []*cli.Command {
	return []*cli.Command{
		getPutCommand(),
		getGetCommand(),
		getEnvCommand(),
		getSyncCommand(),
		getCompletionCommand(),
	}
}

func getPutCommand() *cli.Command {
	return &cli.Command{
		Name:    "put",
		Usage:   "Store/update secrets in Vault (merges with existing data)",
		Aliases: []string{"p"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "path",
				Usage:   "KV path to store secret(s)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "encryption-key",
				Usage: "Transit encryption key name (optional)",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "Specific key to update in multi-value secret",
			},
			&cli.StringFlag{
				Name:  "value",
				Usage: "Secret value (or use stdin)",
			},
			&cli.StringFlag{
				Name:  "from-env",
				Usage: "Load multiple key-value pairs from .env file",
			},
			&cli.StringFlag{
				Name:  "from-file",
				Usage: "Load file content as base64 encoded value",
			},
			&cli.StringFlag{
				Name:  "kv-mount",
				Usage: "KV v2 mount path",
				Value: "kv",
			},
			&cli.StringFlag{
				Name:  "transit-mount",
				Usage: "Transit mount path",
				Value: "transit",
			},
		},
		Action: func(ctx *cli.Context) error {
			// Validate input options
			inputCount := 0
			if ctx.String("value") != "" {
				inputCount++
			}
			if ctx.String("from-env") != "" {
				inputCount++
			}
			if ctx.String("from-file") != "" {
				inputCount++
			}

			if inputCount > 1 {
				return fmt.Errorf("only one of --value, --from-env, or --from-file can be specified")
			}

			// Validate key update operation
			if ctx.String("key") != "" && (ctx.String("from-env") != "" || ctx.String("from-file") != "") {
				return fmt.Errorf("--key cannot be used with --from-env or --from-file")
			}

			appInstance, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			opts := &app.PutOptions{
				KVMount:      ctx.String("kv-mount"),
				KVPath:       ctx.String("path"),
				TransitMount: ctx.String("transit-mount"),
				EncryptionKey: ctx.String("encryption-key"),
				Key:          ctx.String("key"),
				Value:        ctx.String("value"),
				FromEnv:      ctx.String("from-env"),
				FromFile:     ctx.String("from-file"),
			}

			return appInstance.Put(opts)
		},
	}
}

func getGetCommand() *cli.Command {
	return &cli.Command{
		Name:    "get",
		Usage:   "Retrieve and optionally decrypt secrets from Vault",
		Aliases: []string{"g"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "path",
				Usage:    "KV path to retrieve secret",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "encryption-key",
				Usage: "Transit encryption key name (required for encrypted secrets)",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "Specific key to retrieve (for multi-value secrets)",
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output as JSON format",
			},
			&cli.StringFlag{
				Name:  "kv-mount",
				Usage: "KV v2 mount path",
				Value: "kv",
			},
			&cli.StringFlag{
				Name:  "transit-mount",
				Usage: "Transit mount path",
				Value: "transit",
			},
		},
		Action: func(ctx *cli.Context) error {
			appInstance, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			opts := &app.GetOptions{
				KVMount:       ctx.String("kv-mount"),
				KVPath:        ctx.String("path"),
				TransitMount:  ctx.String("transit-mount"),
				EncryptionKey: ctx.String("encryption-key"),
				Key:           ctx.String("key"),
				OutputJSON:    ctx.Bool("json"),
			}

			return appInstance.Get(opts)
		},
	}
}

func getEnvCommand() *cli.Command {
	return &cli.Command{
		Name:    "env",
		Usage:   "Generate .env file from multiple Vault secrets",
		Aliases: []string{"e"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Usage:    "YAML config file with secret definitions",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "encryption-key",
				Usage: "Transit encryption key name",
			},
			&cli.StringFlag{
				Name:  "output",
				Usage: "Output .env file",
				Value: ".env",
			},
		},
		Action: func(ctx *cli.Context) error {
			appInstance, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			return appInstance.GenerateEnvFile(
				ctx.String("config"),
				ctx.String("output"),
				ctx.String("encryption-key"),
			)
		},
	}
}

func getSyncCommand() *cli.Command {
	return &cli.Command{
		Name:    "sync",
		Usage:   "Sync secrets from YAML config to .env file",
		Aliases: []string{"s"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "YAML config file",
				Value: "vault-env.yaml",
			},
			&cli.StringFlag{
				Name:  "output",
				Usage: "Output .env file",
				Value: ".env",
			},
		},
		Action: func(ctx *cli.Context) error {
			appInstance, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			return appInstance.GenerateEnvFile(
				ctx.String("config"),
				ctx.String("output"),
				"", // encryption key will be taken from config or environment
			)
		},
	}
}

func getCompletionCommand() *cli.Command {
	return &cli.Command{
		Name:  "completion",
		Usage: "Generate shell completion scripts",
		Description: `Generate shell completion scripts for various shells.

Supported shells: bash, zsh, fish, powershell

To install completions:

Bash:
  vault-env completion bash > /etc/bash_completion.d/vault-env
  # Or for user-only:
  vault-env completion bash > ~/.bash_completion.d/vault-env

Zsh:
  vault-env completion zsh > /usr/local/share/zsh/site-functions/_vault-env
  # Or for user-only:
  vault-env completion zsh > ~/.zsh/completions/_vault-env

Fish:
  vault-env completion fish > ~/.config/fish/completions/vault-env.fish

PowerShell:
  vault-env completion powershell > vault-env.ps1
  # Then source it in your PowerShell profile`,
		Aliases: []string{"comp"},
		ArgsUsage: "[shell]",
		Action: func(ctx *cli.Context) error {
			shell := ctx.Args().First()
			if shell == "" {
				return fmt.Errorf("shell argument required. Supported: bash, zsh, fish, powershell")
			}

			// Generate completion script for the specified shell
			switch shell {
			case "bash":
				return generateBashCompletion(ctx)
			case "zsh":
				return generateZshCompletion(ctx)
			case "fish":
				return generateFishCompletion(ctx)
			case "powershell":
				return generatePowerShellCompletion(ctx)
			default:
				return fmt.Errorf("unsupported shell: %s. Supported: bash, zsh, fish, powershell", shell)
			}
		},
	}
}

// Completion generation functions
func generateBashCompletion(ctx *cli.Context) error {
	_, err := fmt.Print(`# vault-env bash completion
_vault_env_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    
    # Complete commands
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        opts="put get env sync completion help"
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi
    
    # Complete flags based on command
    case "${COMP_WORDS[1]}" in
        put|p)
            opts="--path --encryption-key --key --value --from-env --from-file --kv-mount --transit-mount --help"
            ;;
        get|g)
            opts="--path --encryption-key --key --json --kv-mount --transit-mount --help"
            ;;
        env|e)
            opts="--config --encryption-key --output --help"
            ;;
        sync|s)
            opts="--config --output --help"
            ;;
        completion|comp)
            if [[ ${COMP_CWORD} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- ${cur}) )
                return 0
            fi
            ;;
        *)
            opts="--help"
            ;;
    esac
    
    # Complete file paths for certain flags
    if [[ "$prev" == "--from-env" || "$prev" == "--from-file" || "$prev" == "--config" ]]; then
        COMPREPLY=( $(compgen -f -- ${cur}) )
        return 0
    fi
    
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
}

complete -F _vault_env_completion vault-env
`)
	return err
}

func generateZshCompletion(ctx *cli.Context) error {
	_, err := fmt.Print(`#compdef vault-env

_vault_env() {
    local context curcontext state line
    typeset -A opt_args
    
    _arguments -C \
        '1: :_vault_env_commands' \
        '*:: :->args'
    
    case $state in
        args)
            case $words[1] in
                put|p)
                    _arguments \
                        '--path=[KV path to store secret(s)]:path:' \
                        '--encryption-key=[Transit encryption key name]:key:' \
                        '--key=[Specific key to update]:key:' \
                        '--value=[Secret value]:value:' \
                        '--from-env=[Load from .env file]:file:_files' \
                        '--from-file=[Load file as base64]:file:_files' \
                        '--kv-mount=[KV v2 mount path]:mount:' \
                        '--transit-mount=[Transit mount path]:mount:' \
                        '--help[Show help]'
                    ;;
                get|g)
                    _arguments \
                        '--path=[KV path to retrieve secret]:path:' \
                        '--encryption-key=[Transit encryption key name]:key:' \
                        '--key=[Specific key to retrieve]:key:' \
                        '--json[Output as JSON format]' \
                        '--kv-mount=[KV v2 mount path]:mount:' \
                        '--transit-mount=[Transit mount path]:mount:' \
                        '--help[Show help]'
                    ;;
                env|e)
                    _arguments \
                        '--config=[YAML config file]:file:_files' \
                        '--encryption-key=[Transit encryption key name]:key:' \
                        '--output=[Output .env file]:file:_files' \
                        '--help[Show help]'
                    ;;
                sync|s)
                    _arguments \
                        '--config=[YAML config file]:file:_files' \
                        '--output=[Output .env file]:file:_files' \
                        '--help[Show help]'
                    ;;
                completion|comp)
                    _arguments '1: :(bash zsh fish powershell)'
                    ;;
            esac
            ;;
    esac
}

_vault_env_commands() {
    local -a commands
    commands=(
        'put:Store/update secrets in Vault'
        'get:Retrieve and decrypt secrets from Vault'
        'env:Generate .env file from multiple Vault secrets'
        'sync:Sync secrets from YAML config to .env file'
        'completion:Generate shell completion scripts'
        'help:Show help'
    )
    _describe 'commands' commands
}

_vault_env
`)
	return err
}

func generateFishCompletion(ctx *cli.Context) error {
	_, err := fmt.Print(`# vault-env fish completion

# Commands
complete -c vault-env -f -n '__fish_use_subcommand' -a 'put' -d 'Store/update secrets in Vault'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'get' -d 'Retrieve and decrypt secrets from Vault'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'env' -d 'Generate .env file from multiple Vault secrets'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'sync' -d 'Sync secrets from YAML config to .env file'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'completion' -d 'Generate shell completion scripts'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'help' -d 'Show help'

# Aliases
complete -c vault-env -f -n '__fish_use_subcommand' -a 'p' -d 'Store/update secrets in Vault (alias)'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'g' -d 'Retrieve and decrypt secrets from Vault (alias)'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'e' -d 'Generate .env file from multiple Vault secrets (alias)'
complete -c vault-env -f -n '__fish_use_subcommand' -a 's' -d 'Sync secrets from YAML config to .env file (alias)'
complete -c vault-env -f -n '__fish_use_subcommand' -a 'comp' -d 'Generate shell completion scripts (alias)'

# Put command options
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'path' -d 'KV path to store secret(s)'
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'encryption-key' -d 'Transit encryption key name'
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'key' -d 'Specific key to update in multi-value secret'
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'value' -d 'Secret value'
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'from-env' -d 'Load multiple key-value pairs from .env file'
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'from-file' -d 'Load file content as base64 encoded value'
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'kv-mount' -d 'KV v2 mount path'
complete -c vault-env -f -n '__fish_seen_subcommand_from put p' -l 'transit-mount' -d 'Transit mount path'

# Get command options
complete -c vault-env -f -n '__fish_seen_subcommand_from get g' -l 'path' -d 'KV path to retrieve secret'
complete -c vault-env -f -n '__fish_seen_subcommand_from get g' -l 'encryption-key' -d 'Transit encryption key name'
complete -c vault-env -f -n '__fish_seen_subcommand_from get g' -l 'key' -d 'Specific key to retrieve'
complete -c vault-env -f -n '__fish_seen_subcommand_from get g' -l 'json' -d 'Output as JSON format'
complete -c vault-env -f -n '__fish_seen_subcommand_from get g' -l 'kv-mount' -d 'KV v2 mount path'
complete -c vault-env -f -n '__fish_seen_subcommand_from get g' -l 'transit-mount' -d 'Transit mount path'

# Env command options
complete -c vault-env -f -n '__fish_seen_subcommand_from env e' -l 'config' -d 'YAML config file with secret definitions'
complete -c vault-env -f -n '__fish_seen_subcommand_from env e' -l 'encryption-key' -d 'Transit encryption key name'
complete -c vault-env -f -n '__fish_seen_subcommand_from env e' -l 'output' -d 'Output .env file'

# Sync command options
complete -c vault-env -f -n '__fish_seen_subcommand_from sync s' -l 'config' -d 'YAML config file'
complete -c vault-env -f -n '__fish_seen_subcommand_from sync s' -l 'output' -d 'Output .env file'

# Completion command options
complete -c vault-env -f -n '__fish_seen_subcommand_from completion comp' -a 'bash' -d 'Generate bash completion'
complete -c vault-env -f -n '__fish_seen_subcommand_from completion comp' -a 'zsh' -d 'Generate zsh completion'
complete -c vault-env -f -n '__fish_seen_subcommand_from completion comp' -a 'fish' -d 'Generate fish completion'
complete -c vault-env -f -n '__fish_seen_subcommand_from completion comp' -a 'powershell' -d 'Generate PowerShell completion'

# Global options
complete -c vault-env -f -l 'vault-addr' -d 'Vault server address'
complete -c vault-env -f -l 'vault-token' -d 'Vault authentication token'
complete -c vault-env -f -l 'vault-namespace' -d 'Vault namespace'
complete -c vault-env -f -l 'encryption-key' -d 'Default transit encryption key'
complete -c vault-env -f -l 'help' -d 'Show help'
complete -c vault-env -f -l 'version' -d 'Print version'
`)
	return err
}

func generatePowerShellCompletion(ctx *cli.Context) error {
	_, err := fmt.Print(`# vault-env PowerShell completion

Register-ArgumentCompleter -Native -CommandName vault-env -ScriptBlock {
    param($commandName, $wordToComplete, $cursorPosition)
    
    $commands = @('put', 'get', 'env', 'sync', 'completion', 'help')
    $aliases = @('p', 'g', 'e', 's', 'comp', 'h')
    
    # Split the command line
    $commandElements = $wordToComplete.Split(' ', [System.StringSplitOptions]::RemoveEmptyEntries)
    
    # Complete main commands
    if ($commandElements.Count -le 1) {
        return ($commands + $aliases) | Where-Object { $_ -like "$wordToComplete*" }
    }
    
    # Complete based on subcommand
    switch ($commandElements[0]) {
        { $_ -in @('put', 'p') } {
            return @('--path', '--encryption-key', '--key', '--value', '--from-env', '--from-file', '--kv-mount', '--transit-mount', '--help') | Where-Object { $_ -like "$wordToComplete*" }
        }
        { $_ -in @('get', 'g') } {
            return @('--path', '--encryption-key', '--key', '--json', '--kv-mount', '--transit-mount', '--help') | Where-Object { $_ -like "$wordToComplete*" }
        }
        { $_ -in @('env', 'e') } {
            return @('--config', '--encryption-key', '--output', '--help') | Where-Object { $_ -like "$wordToComplete*" }
        }
        { $_ -in @('sync', 's') } {
            return @('--config', '--output', '--help') | Where-Object { $_ -like "$wordToComplete*" }
        }
        { $_ -in @('completion', 'comp') } {
            return @('bash', 'zsh', 'fish', 'powershell') | Where-Object { $_ -like "$wordToComplete*" }
        }
    }
    
    return @()
}
`)
	return err
}
