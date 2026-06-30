package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// Command binds a CLI command name to its handler. The user-facing metadata
// (summary, usage, flags, exit codes, examples) lives in core.Commands, which
// is what `specd help` renders. Registry is the single dispatch source consumed
// by main.go. A parity test (registry_test.go) asserts Registry and
// core.Commands never drift — adding a command requires an entry in both, so
// help can never silently omit a dispatchable command and vice versa.
//
// `help` and `version` are intentionally absent: they are handled in main.run
// before dispatch (they are not spec-scoped command handlers).
type Command struct {
	Name string
	Run  func(cli.Args) int
}

// Registry lists every dispatchable command in help-display order.
var Registry = []Command{
	{"init", RunInit},
	{"fusion", RunFusion},
	{"new", RunNew},
	{"approve", RunApprove},
	{"decision", RunDecision},
	{"midreq", RunMidreq},
	{"memory", RunMemory},
	{"brain", RunBrain},
	{"pinky", RunPinky},
	{"next", RunNext},
	{"verify", RunVerify},
	{"task", RunTask},
	{"status", RunStatus},
	{"context", RunContext},
	{"check", RunCheck},
	{"report", RunReport},
	{"waves", RunWaves},
}

// Dispatch runs the handler registered for command. It returns (exitCode, true)
// when the command is known, or (0, false) when it is not — letting the caller
// render the unknown-command help path.
func Dispatch(command string, args cli.Args) (int, bool) {
	if run, ok := legacyAlias(command); ok {
		return run(args), true
	}
	for _, registered := range Registry {
		if registered.Name == command {
			return registered.Run(args), true
		}
	}
	return 0, false
}

func legacyAlias(command string) (func(cli.Args) int, bool) {
	aliases := map[string]func(cli.Args) int{
		"doctor":    RunDoctor,
		"migrate":   deprecatedRuntimeCommand("migrate", "use `specd init --migrate` for one-time config migration"),
		"mode":      RunMode,
		"dispatch":  RunDispatch,
		"program":   RunProgram,
		"validate":  RunValidate,
		"schema":    RunSchema,
		"replay":    RunReplay,
		"diff":      RunDiff,
		"serve":     RunServe,
		"watch":     RunWatch,
		"update":    deprecatedRuntimeCommand("update", "use `scripts/install.sh` or your package manager to update specd"),
		"uninstall": deprecatedRuntimeCommand("uninstall", "use `scripts/uninstall.sh` or your package manager to uninstall specd"),
	}
	run, ok := aliases[command]
	return run, ok
}

func deprecatedRuntimeCommand(name, hint string) func(cli.Args) int {
	return func(args cli.Args) int {
		message := fmt.Sprintf("specd %s has moved out of the runtime palette; %s", name, hint)
		if args.Bool("json") {
			if err := core.PrintJSON(map[string]any{"kind": "deprecated-command", "command": name, "message": message}); err != nil {
				return specdExit(err)
			}
			return core.ExitGate
		}
		core.Error(message)
		return core.ExitGate
	}
}
