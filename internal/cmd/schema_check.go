package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

func schemaFindings(root, slug string) []gates.Finding {
	if _, err := core.LoadState(core.StatePath(root, slug)); err != nil {
		return []gates.Finding{{
			Gate:     "schema",
			Severity: gates.Error,
			Message:  fmt.Sprintf("state.json schema invalid: %v", err),
		}}
	}
	return nil
}
