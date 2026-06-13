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

	if len(argv) == 0 {
		fmt.Print(core.RenderHelp())
		return core.ExitUsage
	}

	if argv[0] == "--json" {
		// json mode first
		os.Setenv("SPECd_JSON", "1")
	}

	command := argv[0]

	switch command {
	case "--help", "-h", "help":
		jsonMode := false
		for _, a := range argv {
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
			if len(argv) == 1 {
				return core.ExitUsage
			}
			return core.ExitOK
		}
		if len(argv) > 1 && !strings.HasPrefix(argv[1], "-") {
			s, err := core.RenderCommandHelp(argv[1])
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
		if len(argv) == 1 {
			return core.ExitOK
		}
		return core.ExitOK

	case "--version", "-v", "version":
		fmt.Printf("specd %s\n", version)
		return core.ExitOK
	}

	// Set JSON mode if --json anywhere in argv.
	for _, a := range argv[1:] {
		if a == "--json" {
			os.Setenv("SPECd_JSON", "1")
		}
	}

	args := cli.ParseArgs(argv[1:])
	return dispatch(command, args)
}

func dispatch(command string, args cli.Args) int {
	switch command {
	case "init":
		return cmd.RunInit(args)
	case "new":
		return cmd.RunNew(args)
	case "status":
		return cmd.RunStatus(args)
	case "context":
		return cmd.RunContext(args)
	case "check":
		return cmd.RunCheck(args)
	case "next":
		return cmd.RunNext(args)
	case "dispatch":
		return cmd.RunDispatch(args)
	case "program":
		return cmd.RunProgram(args)
	case "verify":
		return cmd.RunVerify(args)
	case "task":
		return cmd.RunTask(args)
	case "approve":
		return cmd.RunApprove(args)
	case "decision":
		return cmd.RunDecision(args)
	case "midreq":
		return cmd.RunMidreq(args)
	case "memory":
		return cmd.RunMemory(args)
	case "report":
		return cmd.RunReport(args)
	case "waves":
		return cmd.RunWaves(args)
	case "update":
		return cmd.RunUpdate(args)
	default:
		help := core.RenderHelp()
		core.Error(fmt.Sprintf("unknown command: %s\n\n%s", command, strings.TrimRight(help, "\n")))
		return core.ExitUsage
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}
