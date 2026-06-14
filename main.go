package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
)

// version is set at build time: go build -ldflags "-X main.version=v1.0.0"
var version = "dev"

func run(argv []string) int {
	// Propagate version to core for help output.
	core.Version = version

	// A leading --json is a global flag, not a command. Strip any leading
	// --json token(s) so the command is the first non-flag argument; the flag is
	// re-threaded into the command's args below so per-command --json handling
	// (and JSON env mode) still fire. This makes `specd --json status` behave
	// like `specd status --json`.
	jsonMode := false
	for len(argv) > 0 && argv[0] == "--json" {
		jsonMode = true
		argv = argv[1:]
	}

	if len(argv) == 0 {
		fmt.Print(core.RenderHelp())
		return core.ExitUsage
	}

	command := argv[0]
	rest := argv[1:]

	switch command {
	case "--help", "-h", "help":
		for _, a := range rest {
			if a == "--json" {
				jsonMode = true
			}
		}
		if jsonMode {
			s, err := core.RenderHelpJSON()
			if err != nil {
				core.Error(err.Error())
				return core.ExitGate
			}
			fmt.Println(s)
			if len(rest) == 0 {
				return core.ExitUsage
			}
			return core.ExitOK
		}
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			s, err := core.RenderCommandHelp(rest[0])
			if err != nil {
				core.Error(err.Error())
				return core.ExitUsage
			}
			fmt.Print(strings.TrimRight(s, "\n"))
			fmt.Println()
			return core.ExitOK
		}
		fmt.Print(strings.TrimRight(core.RenderHelp(), "\n"))
		fmt.Println()
		return core.ExitOK

	case "--version", "-v", "version":
		fmt.Printf("specd %s\n", version)
		return core.ExitOK
	}

	// Enable JSON mode if --json appears anywhere after the command, too.
	for _, a := range rest {
		if a == "--json" {
			jsonMode = true
		}
	}
	if jsonMode {
		os.Setenv("SPECD_JSON", "1")
	}

	// Re-thread a stripped leading --json so the command's own flag parsing sees
	// it (cli.ParseArgs treats a duplicate --json as a no-op).
	parseArgv := rest
	if jsonMode {
		parseArgv = append(append([]string{}, rest...), "--json")
	}
	args := cli.ParseArgs(parseArgv)
	return dispatch(command, args)
}

func dispatch(command string, args cli.Args) int {
	if code, ok := cmd.Dispatch(command, args); ok {
		return code
	}
	help := core.RenderHelp()
	core.Error(fmt.Sprintf("unknown command: %s\n\n%s", command, strings.TrimRight(help, "\n")))
	return core.ExitUsage
}

func main() {
	os.Exit(run(os.Args[1:]))
}
