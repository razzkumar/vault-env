package cli

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/razzkumar/vault-env/internal/app"
	"github.com/razzkumar/vault-env/internal/utils"
	"github.com/razzkumar/vault-env/pkg/config"
)

// GetCommands returns all CLI commands
func GetCommands() []*cli.Command {
	return []*cli.Command{
		getPutCommand(),
		getGetCommand(),
		getSyncCommand(),
		getRunCommand(),
		getJSONCommand(),
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
				Name:     "path",
				Usage:    "KV path to store secret(s)",
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
				KVMount:       ctx.String("kv-mount"),
				KVPath:        ctx.String("path"),
				TransitMount:  ctx.String("transit-mount"),
				EncryptionKey: ctx.String("encryption-key"),
				Key:           ctx.String("key"),
				Value:         ctx.String("value"),
				FromEnv:       ctx.String("from-env"),
				FromFile:      ctx.String("from-file"),
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
		Description: `Retrieve secrets from Vault either by direct path or from a config file.

Examples:
  # Get secrets from specific path
  vault-env get --path secrets/prod
  
  # Get all secrets from config file
  vault-env get --config secrets.yaml
  
  # Get secrets from default config file (vault-env.yaml)
  vault-env get
  
  # Output as JSON
  vault-env get --config secrets.yaml --json`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "KV path to retrieve secret",
			},
			&cli.StringFlag{
				Name:  "config",
				Usage: "YAML config file with secret definitions (defaults to vault-env.yaml if exists)",
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
			// Check for default config file if neither path nor config specified
			configFile := ctx.String("config")
			kvPath := ctx.String("path")

			if configFile == "" && kvPath == "" {
				// Check if vault-env.yaml exists in current directory
				if _, err := os.Stat("vault-env.yaml"); err == nil {
					configFile = "vault-env.yaml"
				}
			}

			// Validate that we have either path or config
			if kvPath == "" && configFile == "" {
				return fmt.Errorf("either --path, --config, or vault-env.yaml file must be specified")
			}

			appInstance, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			if configFile != "" {
				// Use config file to get all secrets
				return appInstance.GetFromConfig(configFile, ctx.String("encryption-key"), ctx.Bool("json"))
			} else {
				// Use direct path
				opts := &app.GetOptions{
					KVMount:       ctx.String("kv-mount"),
					KVPath:        kvPath,
					TransitMount:  ctx.String("transit-mount"),
					EncryptionKey: ctx.String("encryption-key"),
					Key:           ctx.String("key"),
					OutputJSON:    ctx.Bool("json"),
				}
				return appInstance.Get(opts)
			}
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

func getRunCommand() *cli.Command {
	return &cli.Command{
		Name:    "run",
		Usage:   "Run command with secrets injected as environment variables",
		Aliases: []string{"r"},
		Description: `Run a command with secrets from Vault injected as environment variables.

This command fetches secrets from Vault (using a config file or individual paths),
decrypts them if needed, and injects them into the environment of the specified command.

The command inherits your current environment and adds/overrides with Vault secrets.

Examples:
  # Run with config file (most common)
  vault-env run --config secrets.yaml -- go run main.go
  
  # Run with default config file (vault-env.yaml)
  vault-env run -- go run main.go
  
  # Run with inline secret injection
  vault-env run --inject DB_PASSWORD=secrets/db_password -- ./myapp
  
  # Run with multiple secret injections
  vault-env run --inject DB_PASSWORD=secrets/db_password --inject API_KEY=secrets/api_key -- npm start
  
  # Run with existing .env file plus Vault secrets
  vault-env run --config secrets.yaml --env-file .env.local -- python app.py

Note: Use -- to separate vault-env flags from the command to run.
If vault-env.yaml exists in the current directory, it will be used automatically if no --config is specified.`,
		ArgsUsage: "[-- command args...]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "YAML config file with secret definitions (defaults to vault-env.yaml if exists)",
			},
			&cli.StringFlag{
				Name:  "encryption-key",
				Usage: "Transit encryption key name",
			},
			&cli.StringSliceFlag{
				Name:  "inject",
				Usage: "Inject specific secret as ENV_VAR=vault_path (can be used multiple times)",
			},
			&cli.StringFlag{
				Name:  "env-file",
				Usage: "Load additional environment variables from .env file",
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
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Show environment variables that would be set without running the command",
			},
			&cli.BoolFlag{
				Name:  "preserve-env",
				Usage: "Preserve all current environment variables (default: true)",
				Value: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			// Check for default config file if none specified and no inject flags provided
			configFile := ctx.String("config")
			injectSecrets := ctx.StringSlice("inject")

			if configFile == "" && len(injectSecrets) == 0 {
				// Check if vault-env.yaml exists in current directory only if no inject flags
				if _, err := os.Stat("vault-env.yaml"); err == nil {
					configFile = "vault-env.yaml"
				}
			}

			// Validate that we have either config or inject flags
			if configFile == "" && len(injectSecrets) == 0 {
				return fmt.Errorf("either --config, vault-env.yaml file, or --inject must be specified")
			}

			// Get the command to run (everything after --)
			args := ctx.Args().Slice()
			if len(args) == 0 {
				return fmt.Errorf("command to run is required. Use -- to separate vault-env options from the command")
			}

			appInstance, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			opts := &app.RunOptions{
				KVMount:       ctx.String("kv-mount"),
				TransitMount:  ctx.String("transit-mount"),
				EncryptionKey: ctx.String("encryption-key"),
				ConfigFile:    configFile,
				InjectSecrets: injectSecrets,
				EnvFile:       ctx.String("env-file"),
				DryRun:        ctx.Bool("dry-run"),
				PreserveEnv:   ctx.Bool("preserve-env"),
				Command:       args[0],
				Args:          args[1:],
			}

			return appInstance.Run(opts)
		},
	}
}

func getJSONCommand() *cli.Command {
	return &cli.Command{
		Name:    "json",
		Usage:   "Encrypt .env file content and output as JSON",
		Aliases: []string{"j"},
		Description: `Encrypts environment variables from a .env file using Vault Transit encryption and outputs the result as JSON.

This command is useful for converting .env files to encrypted JSON format that can be stored in Vault or other secure storage systems.

Encryption is controlled by:
  1. TRANSIT environment variable (true/false, 1/0, yes/no, on/off, enable/disable)
  2. --encryption-key flag or ENCRYPTION_KEY environment variable

Defaults when TRANSIT=true:
  - ENCRYPTION_KEY defaults to "app-secrets"
  - TRANSIT_MOUNT defaults to "transit"

Examples:
  # Output plaintext JSON (default behavior)
  vlt json
  
  # Output plaintext JSON from specific file
  vlt json example.env
  
  # Enable encryption with defaults (key="app-secrets", mount="transit")
  TRANSIT=true vlt json
  
  # Enable encryption with custom key
  TRANSIT=true ENCRYPTION_KEY=mykey vlt json
  
  # Enable encryption with command flag
  vlt json --encryption-key mykey
  
  # Disable encryption even if encryption key is set
  TRANSIT=false ENCRYPTION_KEY=mykey vlt json
  
  # Use custom transit mount
  TRANSIT=true TRANSIT_MOUNT=custom-transit vlt json example.env`,
		ArgsUsage: "[env-file]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "encryption-key",
				Usage: "Transit encryption key name (optional - if not provided, outputs plaintext)",
			},
			&cli.StringFlag{
				Name:  "transit-mount",
				Usage: "Transit mount path",
				Value: "transit",
			},
		},
		Action: func(ctx *cli.Context) error {
			// Get env file from args or default to .env
			envFile := ctx.Args().First()
			if envFile == "" {
				envFile = ".env"
			}

			// Check if encryption is needed based on encryption key and TRANSIT env var
			encryptionKey := config.GetEncryptionKey(ctx.String("encryption-key"))
			useEncryption := config.ShouldUseEncryption(encryptionKey)

			if !useEncryption {
				// For plaintext output, don't need vault client
				return handlePlaintextJSON(envFile)
			}

			// For encryption, create app with vault client
			appInstance, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to create app: %w", err)
			}

			opts := &app.JSONOptions{
				TransitMount:  config.GetTransitMount(ctx.String("transit-mount")),
				EncryptionKey: ctx.String("encryption-key"),
				EnvFile:       envFile,
			}

			return appInstance.JSON(opts)
		},
	}
}

// handlePlaintextJSON handles JSON output without encryption (no vault client needed)
func handlePlaintextJSON(envFile string) error {
	// Check if file exists
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		return fmt.Errorf("env file not found: %s", envFile)
	}

	// Load as plaintext
	data, err := utils.LoadEnvFileAsPlaintext(envFile)
	if err != nil {
		return fmt.Errorf("load env file: %w", err)
	}

	// Output as JSON
	if err := utils.OutputJSON(data); err != nil {
		return fmt.Errorf("output json: %w", err)
	}

	return nil
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
		Aliases:   []string{"comp"},
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
	_, err := fmt.Print(`# vlt bash completion
_vlt_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    
    # Complete commands
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        opts="put get sync run json completion help"
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi
    
    # Complete flags based on command
    case "${COMP_WORDS[1]}" in
        put|p)
            opts="--path --encryption-key --key --value --from-env --from-file --kv-mount --transit-mount --help"
            ;;
        get|g)
            opts="--path --config --encryption-key --key --json --kv-mount --transit-mount --help"
            ;;
        sync|s)
            opts="--config --output --help"
            ;;
        run|r)
            opts="--config --encryption-key --inject --env-file --kv-mount --transit-mount --dry-run --preserve-env --help"
            ;;
        json|j)
            opts="--encryption-key --transit-mount --help"
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

complete -F _vlt_completion vlt
`)
	return err
}

func generateZshCompletion(ctx *cli.Context) error {
	_, err := fmt.Print(`#compdef vlt

_vlt() {
    local context curcontext state line
    typeset -A opt_args
    
    _arguments -C \
        '1: :_vlt_commands' \
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
                        '--config=[YAML config file]:file:_files' \
                        '--encryption-key=[Transit encryption key name]:key:' \
                        '--key=[Specific key to retrieve]:key:' \
                        '--json[Output as JSON format]' \
                        '--kv-mount=[KV v2 mount path]:mount:' \
                        '--transit-mount=[Transit mount path]:mount:' \
                        '--help[Show help]'
                    ;;
                sync|s)
                    _arguments \
                        '--config=[YAML config file]:file:_files' \
                        '--output=[Output .env file]:file:_files' \
                        '--help[Show help]'
                    ;;
                run|r)
                    _arguments \
                        '--config=[YAML config file]:file:_files' \
                        '--encryption-key=[Transit encryption key name]:key:' \
                        '--inject=[Inject specific secret]:inject:' \
                        '--env-file=[Additional .env file]:file:_files' \
                        '--kv-mount=[KV v2 mount path]:mount:' \
                        '--transit-mount=[Transit mount path]:mount:' \
                        '--dry-run[Show env vars without running]' \
                        '--preserve-env[Preserve current environment]' \
                        '--help[Show help]'
                    ;;
                json|j)
                    _arguments \
                        '--encryption-key=[Transit encryption key name]:key:' \
                        '--transit-mount=[Transit mount path]:mount:' \
                        '--help[Show help]' \
                        '1: :_files'
                    ;;
                completion|comp)
                    _arguments '1: :(bash zsh fish powershell)'
                    ;;
            esac
            ;;
    esac
}

_vlt_commands() {
    local -a commands
    commands=(
        'put:Store/update secrets in Vault'
        'get:Retrieve and decrypt secrets from Vault'
        'sync:Sync secrets from YAML config to .env file'
        'run:Run command with secrets injected as environment variables'
        'json:Encrypt .env file content and output as JSON'
        'completion:Generate shell completion scripts'
        'help:Show help'
    )
    _describe 'commands' commands
}

_vlt
`)
	return err
}

func generateFishCompletion(ctx *cli.Context) error {
	_, err := fmt.Print(`# vlt fish completion

# Commands
complete -c vlt -f -n '__fish_use_subcommand' -a 'put' -d 'Store/update secrets in Vault'
complete -c vlt -f -n '__fish_use_subcommand' -a 'get' -d 'Retrieve and decrypt secrets from Vault'
complete -c vlt -f -n '__fish_use_subcommand' -a 'sync' -d 'Sync secrets from YAML config to .env file'
complete -c vlt -f -n '__fish_use_subcommand' -a 'run' -d 'Run command with secrets injected as environment variables'
complete -c vlt -f -n '__fish_use_subcommand' -a 'json' -d 'Encrypt .env file content and output as JSON'
complete -c vlt -f -n '__fish_use_subcommand' -a 'completion' -d 'Generate shell completion scripts'
complete -c vlt -f -n '__fish_use_subcommand' -a 'help' -d 'Show help'

# Aliases
complete -c vlt -f -n '__fish_use_subcommand' -a 'p' -d 'Store/update secrets in Vault (alias)'
complete -c vlt -f -n '__fish_use_subcommand' -a 'g' -d 'Retrieve and decrypt secrets from Vault (alias)'
complete -c vlt -f -n '__fish_use_subcommand' -a 's' -d 'Sync secrets from YAML config to .env file (alias)'
complete -c vlt -f -n '__fish_use_subcommand' -a 'r' -d 'Run command with secrets injected as environment variables (alias)'
complete -c vlt -f -n '__fish_use_subcommand' -a 'j' -d 'Encrypt .env file content and output as JSON (alias)'
complete -c vlt -f -n '__fish_use_subcommand' -a 'comp' -d 'Generate shell completion scripts (alias)'

# Put command options
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'path' -d 'KV path to store secret(s)'
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'encryption-key' -d 'Transit encryption key name'
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'key' -d 'Specific key to update in multi-value secret'
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'value' -d 'Secret value'
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'from-env' -d 'Load multiple key-value pairs from .env file'
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'from-file' -d 'Load file content as base64 encoded value'
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'kv-mount' -d 'KV v2 mount path'
complete -c vlt -f -n '__fish_seen_subcommand_from put p' -l 'transit-mount' -d 'Transit mount path'

# Get command options
complete -c vlt -f -n '__fish_seen_subcommand_from get g' -l 'path' -d 'KV path to retrieve secret'
complete -c vlt -f -n '__fish_seen_subcommand_from get g' -l 'config' -d 'YAML config file with secret definitions'
complete -c vlt -f -n '__fish_seen_subcommand_from get g' -l 'encryption-key' -d 'Transit encryption key name'
complete -c vlt -f -n '__fish_seen_subcommand_from get g' -l 'key' -d 'Specific key to retrieve'
complete -c vlt -f -n '__fish_seen_subcommand_from get g' -l 'json' -d 'Output as JSON format'
complete -c vlt -f -n '__fish_seen_subcommand_from get g' -l 'kv-mount' -d 'KV v2 mount path'
complete -c vlt -f -n '__fish_seen_subcommand_from get g' -l 'transit-mount' -d 'Transit mount path'

# Sync command options
complete -c vlt -f -n '__fish_seen_subcommand_from sync s' -l 'config' -d 'YAML config file'
complete -c vlt -f -n '__fish_seen_subcommand_from sync s' -l 'output' -d 'Output .env file'

# Run command options
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'config' -d 'YAML config file with secret definitions'
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'encryption-key' -d 'Transit encryption key name'
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'inject' -d 'Inject specific secret as ENV_VAR=vault_path'
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'env-file' -d 'Load additional environment variables from .env file'
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'kv-mount' -d 'KV v2 mount path'
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'transit-mount' -d 'Transit mount path'
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'dry-run' -d 'Show environment variables without running command'
complete -c vlt -f -n '__fish_seen_subcommand_from run r' -l 'preserve-env' -d 'Preserve all current environment variables'

# JSON command options
complete -c vlt -f -n '__fish_seen_subcommand_from json j' -l 'encryption-key' -d 'Transit encryption key name'
complete -c vlt -f -n '__fish_seen_subcommand_from json j' -l 'transit-mount' -d 'Transit mount path'

# Completion command options
complete -c vlt -f -n '__fish_seen_subcommand_from completion comp' -a 'bash' -d 'Generate bash completion'
complete -c vlt -f -n '__fish_seen_subcommand_from completion comp' -a 'zsh' -d 'Generate zsh completion'
complete -c vlt -f -n '__fish_seen_subcommand_from completion comp' -a 'fish' -d 'Generate fish completion'
complete -c vlt -f -n '__fish_seen_subcommand_from completion comp' -a 'powershell' -d 'Generate PowerShell completion'

# Global options
complete -c vlt -f -l 'vault-addr' -d 'Vault server address'
complete -c vlt -f -l 'vault-token' -d 'Vault authentication token'
complete -c vlt -f -l 'vault-namespace' -d 'Vault namespace'
complete -c vlt -f -l 'encryption-key' -d 'Default transit encryption key'
complete -c vlt -f -l 'help' -d 'Show help'
complete -c vlt -f -l 'version' -d 'Print version'
`)
	return err
}

func generatePowerShellCompletion(ctx *cli.Context) error {
	_, err := fmt.Print(`# vlt PowerShell completion

Register-ArgumentCompleter -Native -CommandName vlt -ScriptBlock {
    param($commandName, $wordToComplete, $cursorPosition)
    
    $commands = @('put', 'get', 'sync', 'run', 'json', 'completion', 'help')
    $aliases = @('p', 'g', 's', 'r', 'j', 'comp', 'h')
    
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
            return @('--path', '--config', '--encryption-key', '--key', '--json', '--kv-mount', '--transit-mount', '--help') | Where-Object { $_ -like "$wordToComplete*" }
        }
        { $_ -in @('sync', 's') } {
            return @('--config', '--output', '--help') | Where-Object { $_ -like "$wordToComplete*" }
        }
        { $_ -in @('run', 'r') } {
            return @('--config', '--encryption-key', '--inject', '--env-file', '--kv-mount', '--transit-mount', '--dry-run', '--preserve-env', '--help') | Where-Object { $_ -like "$wordToComplete*" }
        }
        { $_ -in @('json', 'j') } {
            return @('--encryption-key', '--transit-mount', '--help') | Where-Object { $_ -like "$wordToComplete*" }
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
