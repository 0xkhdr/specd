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

func printlnErr(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}
