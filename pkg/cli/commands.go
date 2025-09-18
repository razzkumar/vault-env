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