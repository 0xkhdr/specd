package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

func specdExit(err error) int {
	var se *core.SpecdError
	if errors.As(err, &se) {
		core.Error(se.Message)
		return se.Code
	}
	core.Error(err.Error())
	return core.ExitGate
}

func usageExit(msg string) int {
	core.Error(msg)
	return core.ExitUsage
}

// errLine writes a diagnostic line to stderr. It is the canonical stderr path
// for command-level `fail …` / `✗ …` output, keeping machine-readable results
// on stdout. A trailing newline is always appended. For styled single-line
// errors prefer core.Error; use errLine for the multi-line gate dumps.
func errLine(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
