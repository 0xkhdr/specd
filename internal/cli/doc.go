// Package cli parses specd's raw command-line arguments into Args: a list
// of positional tokens and a flag map. It has no knowledge of any specific
// command — that mapping from parsed Args to behavior lives in
// internal/cmd — so it stays a single-purpose, dependency-free tokenizer
// shared by every command.
package cli
