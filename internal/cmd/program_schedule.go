package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/runner"
)

const scheduleUsage = "usage: specd program schedule [<name> --interval <seconds> --command <cmd> [--sandbox <backend>] | <name> --remove | --json]"

// jsonOrExit prints v as JSON and returns the process exit code.
func jsonOrExit(v interface{}) int {
	if err := core.PrintJSON(v); err != nil {
		return specdExit(err)
	}
	return core.ExitOK
}

// maintenanceTimeout bounds a single scheduled command's wall clock.
const maintenanceTimeout = 600 * time.Second

// programSchedule implements `specd program schedule` (P3.5): with no name it
// lists registered maintenance schedules; with a name it registers/replaces one
// (or removes it with --remove). Registration is declaration only — nothing runs
// until a host invokes `specd program tick`.
func programSchedule(root string, args cli.Args) int {
	name := ""
	if len(args.Pos) >= 2 {
		name = args.Pos[1]
	}

	if name == "" {
		return programScheduleList(root, args.Bool("json"))
	}

	if args.Bool("remove") {
		removed, err := core.RemoveSchedule(root, name)
		if err != nil {
			return specdExit(err)
		}
		if !removed {
			return specdExit(core.NotFoundError(fmt.Sprintf("no maintenance schedule named %q", name)))
		}
		if args.Bool("json") {
			return jsonOrExit(map[string]interface{}{"removed": name})
		}
		fmt.Printf("✓ removed maintenance schedule '%s'\n", name)
		return core.ExitOK
	}

	if !args.Has("interval") || !args.Has("command") {
		return usageExit(scheduleUsage)
	}
	interval, err := strconv.ParseInt(strings.TrimSpace(args.Str("interval")), 10, 64)
	if err != nil {
		return usageExit(fmt.Sprintf("invalid --interval %q — provide a whole number of seconds", args.Str("interval")))
	}
	s := core.MaintenanceSchedule{
		Name:            name,
		Command:         args.Str("command"),
		Sandbox:         args.Str("sandbox"),
		IntervalSeconds: interval,
	}
	if err := core.UpsertSchedule(root, s); err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		return jsonOrExit(map[string]interface{}{"name": s.Name, "intervalSeconds": s.IntervalSeconds})
	}
	fmt.Printf("✓ registered maintenance schedule '%s' (every %ds)\n", s.Name, s.IntervalSeconds)
	return core.ExitOK
}

func programScheduleList(root string, asJSON bool) int {
	m, err := core.LoadProgram(root)
	if err != nil {
		return specdExit(err)
	}
	schedules := append([]core.MaintenanceSchedule(nil), m.Schedules...)
	sort.Slice(schedules, func(i, j int) bool { return schedules[i].Name < schedules[j].Name })
	if asJSON {
		return jsonOrExit(map[string]interface{}{"schedules": schedules})
	}
	if len(schedules) == 0 {
		fmt.Println("no maintenance schedules registered")
		return core.ExitOK
	}
	for _, s := range schedules {
		last := "never"
		if s.LastRunUnix > 0 {
			last = time.Unix(s.LastRunUnix, 0).UTC().Format(time.RFC3339)
		}
		fmt.Printf("%s  every %ds  last-run %s\n", s.Name, s.IntervalSeconds, last)
	}
	return core.ExitOK
}

// programTick implements `specd program tick` (P3.5): a host scheduler invokes
// it; for every registered schedule whose interval has elapsed it claims the
// schedule (advancing its last-run under the program lock) and runs the command
// through the shared sandboxed exec path. It never daemonizes and is idempotent
// — a second tick in the same window finds nothing due and runs nothing.
// `--now <unix>` overrides the clock for deterministic host scheduling and tests.
func programTick(root string, args cli.Args) int {
	now := time.Now().Unix()
	if args.Has("now") {
		n, err := strconv.ParseInt(strings.TrimSpace(args.Str("now")), 10, 64)
		if err != nil {
			return usageExit(fmt.Sprintf("invalid --now %q — provide a unix timestamp in seconds", args.Str("now")))
		}
		now = n
	}

	m, err := core.LoadProgram(root)
	if err != nil {
		return specdExit(err)
	}

	type tickResult struct {
		Name     string `json:"name"`
		ExitCode int    `json:"exitCode"`
		TimedOut bool   `json:"timedOut,omitempty"`
	}
	var ran []tickResult
	worstExit := 0

	for _, due := range core.DueSchedules(m, now) {
		claimed, ok, err := core.ClaimSchedule(root, due.Name, now)
		if err != nil {
			return specdExit(err)
		}
		if !ok {
			// Lost the race to a concurrent tick — skip without executing.
			continue
		}
		res := execMaintenanceCommand(root, claimed.Command, claimed.Sandbox)
		ran = append(ran, tickResult{Name: claimed.Name, ExitCode: res.ExitCode, TimedOut: res.TimedOut})
		if res.ExitCode != 0 {
			worstExit = res.ExitCode
			if !args.Bool("json") {
				errLine("✗ schedule '%s' exited %d%s\n%s", claimed.Name, res.ExitCode, timedOutNote(res.TimedOut), strings.TrimSpace(res.Stderr))
			}
		} else if !args.Bool("json") {
			fmt.Printf("✓ schedule '%s' ran (exit 0)\n", claimed.Name)
		}
	}

	if args.Bool("json") {
		if err := core.PrintJSON(map[string]interface{}{"now": now, "ran": ran}); err != nil {
			return specdExit(err)
		}
	} else if len(ran) == 0 {
		fmt.Println("tick: nothing due")
	}
	if worstExit != 0 {
		return core.ExitGate
	}
	return core.ExitOK
}

// execMaintenanceCommand runs a scheduled command through the shared sandboxed
// runner with a scrubbed env — the same fail-closed path as submit and verify.
func execMaintenanceCommand(root, command, sandbox string) runner.RunResult {
	r, err := runner.SelectRunner(sandbox)
	if err != nil {
		return runner.RunResult{ExitCode: 1, Stderr: err.Error()}
	}
	shell := strings.TrimSpace(os.Getenv("SPECD_VERIFY_SHELL"))
	if shell == "" {
		shell = "sh"
	}
	return r.Run(context.Background(), runner.RunSpec{
		Root:    root,
		Shell:   shell,
		Command: command,
		Env:     core.ScrubbedEnv(),
		Timeout: maintenanceTimeout,
	})
}
