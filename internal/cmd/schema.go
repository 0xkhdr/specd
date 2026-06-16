package cmd

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunSchema writes the embedded JSON Schema for the open spec format to stdout.
// `--version` selects a schema version (default: the current one); an unknown
// version fails closed. It is pure output — no spec, no .specd/ root required —
// so it works anywhere as the published, machine-readable format contract.
func RunSchema(args cli.Args) int {
	doc, err := core.Schema(args.Str("version"))
	if err != nil {
		return specdExit(err)
	}
	if _, err := os.Stdout.Write(doc); err != nil {
		return specdExit(err)
	}
	// Embedded schema files do not carry a trailing newline guarantee; add one
	// so piping to a terminal or file is clean.
	if len(doc) > 0 && doc[len(doc)-1] != '\n' {
		fmt.Println()
	}
	return core.ExitOK
}
