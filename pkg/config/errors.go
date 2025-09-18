package config

import "errors"

var (
	// ErrMissingVaultAddr is returned when VAULT_ADDR is not set
	ErrMissingVaultAddr = errors.New("VAULT_ADDR environment variable is required")

	// ErrMissingVaultToken is returned when VAULT_TOKEN is not set
	ErrMissingVaultToken = errors.New("VAULT_TOKEN environment variable is required")

	// ErrInvalidConfig is returned when configuration is invalid
	ErrInvalidConfig = errors.New("invalid configuration")
)