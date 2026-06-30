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

// nextMinorVersion is the default removal target for the current crop of
// grace-period legacy aliases. At this release the corresponding entries in
// legacyAliases are deleted so the old top-level names fall through to the
// unknown-command help path (cmd-deprecate REQ-1.3 — every deprecation has a
// recorded removal version, not an open-ended grace period).
const nextMinorVersion = "v0.2.0"

// legacyAliasMeta describes one deprecated top-level command name still accepted
// by Dispatch during its grace period.
//
// Sunset policy (decision, see .specd/specs/cmd-merge/decisions.md): during the
// grace period an alias stays FUNCTIONAL — it runs its original handler and
// returns that handler's exit code — after emitting a one-line stderr
// deprecation warning naming the survivor home (cmd-deprecate REQ-1.2). Aliases
// with no functional survivor (migrate/update/uninstall) are terminal: they
// warn and exit non-zero. At removedIn the entry is deleted and the name flips
// to the unknown-command path.
//
// We deliberately call the ORIGINAL handler rather than re-routing through the
// survivor + injected flag: for the report/check/next/status survivors the flag
// already delegates back to the original handler, so the result is identical;
// for doctor the survivor does NOT preserve full capability (doctor's
// diagnostics ≠ `init --repair`), so calling the original handler is the only
// behaviour-preserving choice the grace period allows. (mode's set/recommend
// paths now have survivor homes under `status` — see optimization-plan Phase 2 —
// but its alias still routes to RunMode for the remaining grace period.)
type legacyAliasMeta struct {
	home       string             // survivor home named in the warning
	removedIn  string             // release at which this alias is deleted
	functional bool               // true: run the handler; false: warn + exit non-zero
	run        func(cli.Args) int // original handler (nil when not functional)
}

// legacyAliases is the single source of truth for the deprecated runtime command
// surface. TestLegacyAliasSunset reads it to assert every alias has a recorded
// removal version and emits a deprecation warning — the guard that lets the
// gates see the kitchen (hidden aliases), not just the visible palette menu.
var legacyAliases = map[string]legacyAliasMeta{
	"doctor":   {home: "specd init --repair", removedIn: nextMinorVersion, functional: true, run: RunDoctor},
	"dispatch": {home: "specd next --dispatch", removedIn: nextMinorVersion, functional: true, run: RunDispatch},
	"program":  {home: "specd status --program", removedIn: nextMinorVersion, functional: true, run: RunProgram},
	"validate": {home: "specd check --schema-only", removedIn: nextMinorVersion, functional: true, run: RunValidate},
	"schema":   {home: "specd check --schema", removedIn: nextMinorVersion, functional: true, run: RunSchema},
	"replay":   {home: "specd report --history", removedIn: nextMinorVersion, functional: true, run: RunReplay},
	"diff":     {home: "specd report --diff", removedIn: nextMinorVersion, functional: true, run: RunDiff},
	"serve":    {home: "specd report --serve", removedIn: nextMinorVersion, functional: true, run: RunServe},
	"watch":    {home: "specd report --watch", removedIn: nextMinorVersion, functional: true, run: RunWatch},
	// mode's capabilities now all have survivor homes (optimization-plan Phase 2):
	// view → `specd status`, create → `specd new --orchestrated`, mutate an
	// existing spec's mode → `specd status <slug> --set-mode`, advise →
	// `specd status <slug> --recommend`. The alias stays functional through its
	// recorded grace period; removing it at v0.3.0 now drops no capability.
	"mode": {home: "specd status <slug> --set-mode | --recommend (set/advise), specd new --orchestrated (create)", removedIn: "v0.3.0", functional: true, run: RunMode},
	// Genuinely retired from the runtime: no functional survivor, so these warn
	// and exit non-zero rather than running anything.
	"migrate":   {home: "specd init --migrate", removedIn: nextMinorVersion, functional: false},
	"update":    {home: "scripts/install.sh or your package manager", removedIn: nextMinorVersion, functional: false},
	"uninstall": {home: "scripts/uninstall.sh or your package manager", removedIn: nextMinorVersion, functional: false},
}

func legacyAlias(command string) (func(cli.Args) int, bool) {
	meta, ok := legacyAliases[command]
	if !ok {
		return nil, false
	}
	return func(args cli.Args) int {
		if !meta.functional {
			return terminalDeprecation(command, meta, args)
		}
		// Functional alias: warn to stderr only (the survivor handler owns
		// stdout, including any --json payload) then delegate.
		core.Error(deprecationMessage(command, meta))
		return meta.run(args)
	}, true
}

// deprecationMessage renders the one-line warning naming the survivor home and
// the removal version.
func deprecationMessage(command string, meta legacyAliasMeta) string {
	return fmt.Sprintf("deprecated: 'specd %s' moved to %s (removed in %s)", command, meta.home, meta.removedIn)
}

// terminalDeprecation handles a retired (non-functional) alias: it emits the
// machine-readable deprecation object under --json or the stderr warning
// otherwise, and always exits non-zero.
func terminalDeprecation(command string, meta legacyAliasMeta, args cli.Args) int {
	message := deprecationMessage(command, meta)
	if args.Bool("json") {
		if err := core.PrintJSON(map[string]any{
			"kind":      "deprecated-command",
			"command":   command,
			"movedTo":   meta.home,
			"removedIn": meta.removedIn,
			"message":   message,
		}); err != nil {
			return specdExit(err)
		}
		return core.ExitGate
	}
	core.Error(message)
	return core.ExitGate
}
