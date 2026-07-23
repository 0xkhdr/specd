package cmd

import "github.com/0xkhdr/specd/internal/core"

// flagHelpLine renders one flag for per-command text help. A value flag that
// declares a value shape or provenance note (core.Flag.Values, spec R4.3)
// carries it between the name and the description, so the text surface and
// `help --json` expose the same contract.
func flagHelpLine(flag core.Flag) string {
	line := "  --" + flag.Name
	if hint := flag.ValueHint(); hint != "" {
		line += "  <" + hint + ">"
	}
	if flag.Description != "" {
		line += "  " + flag.Description
	}
	return line
}
