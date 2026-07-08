package cmd

import (
	"github.com/0xkhdr/specd/internal/cli"
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
	{"handshake", RunHandshake},
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
	{"review", RunReview},
	{"eval", RunEval},
	{"promote", RunPromote},
	{"conductor", RunConductor},
	{"orchestrate", RunOrchestrate},
	{"submit", RunSubmit},
	{"deploy", RunDeploy},
	{"observe", RunObserve},
	{"ingest", RunIngest},
	{"report", RunReport},
	{"waves", RunWaves},
	{"harness", RunHarness},
	{"dashboard", RunDashboard},
	{"migrate", RunMigrate},
}

// Dispatch runs the handler registered for command. It returns (exitCode, true)
// when the command is known, or (0, false) when it is not — letting the caller
// render the unknown-command help path.
func Dispatch(command string, args cli.Args) (int, bool) {
	for _, registered := range Registry {
		if registered.Name == command {
			return registered.Run(args), true
		}
	}
	return 0, false
}
