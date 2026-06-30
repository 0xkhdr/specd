// Package cmd implements specd's CLI commands, one file per command
// (RunInit, RunNext, RunVerify, and so on), dispatched through the
// command-name-to-handler table in registry.go. Each handler parses its
// already-tokenized internal/cli.Args, applies command-specific validation,
// and calls into internal/core for the actual state/spec mutation —
// keeping domain logic out of the command layer.
//
// helpers.go centralizes the shared exit/usage/error-line conventions so
// every command produces a consistent CLI and JSON-mode contract.
package cmd
