package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunPinky(args cli.Args) int {
	if len(args.Pos) == 0 {
		return usageExit("usage: specd pinky <claim|brief|heartbeat|progress|query|report|block|release|inbox> ...")
	}
	root, ok := core.FindSpecdRoot(".")
	if !ok {
		return specdExit(core.NotFoundError("not in a specd workspace"))
	}
	cfg := core.LoadConfig(root).Orchestration

	switch args.Pos[0] {
	case "claim":
		if len(args.Pos) != 1 || args.Str("mission") == "" {
			return usageExit("usage: specd pinky claim --mission <path|-> [--json]")
		}
		mission, err := readPinkyMission(args.Str("mission"))
		if err != nil {
			return specdExit(err)
		}
		claim, err := core.ClaimPinkyMission(root, mission, cfg)
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, claim)
	case "brief":
		return runPinkyBrief(root, cfg, args)
	case "heartbeat":
		sessionID, workerID, attempt, ok := pinkyLeaseArgs(args)
		if len(args.Pos) != 1 || !ok {
			return usageExit("usage: specd pinky heartbeat --session <id> --worker <id> --attempt <n> [--json]")
		}
		lease, err := core.HeartbeatPinkyClaim(root, sessionID, workerID, attempt, cfg)
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, lease)
	case "progress":
		report, ok := pinkyProgressArgs(args)
		if len(args.Pos) != 1 || !ok {
			return usageExit("usage: specd pinky progress --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --percent <0-100> --message <text> [--json]")
		}
		event, err := core.RecordPinkyProgress(root, report, cfg)
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, event)
	case "block":
		report, ok := pinkyBlockArgs(args)
		if len(args.Pos) != 1 || !ok {
			return usageExit("usage: specd pinky block --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --reason <text> [--json]")
		}
		event, err := core.RecordPinkyBlocker(root, report, cfg)
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, event)
	case "query":
		report, ok := pinkyQueryArgs(args)
		if len(args.Pos) != 1 || !ok {
			return usageExit("usage: specd pinky query --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --text <question> [--json]")
		}
		event, err := core.RecordPinkyQuery(root, report, cfg)
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, event)
	case "inbox":
		if len(args.Pos) != 1 || args.Str("session") == "" || args.Str("worker") == "" {
			return usageExit("usage: specd pinky inbox --session <id> --worker <id> [--json]")
		}
		inbox, err := core.ReadPinkyInbox(root, args.Str("session"), args.Str("worker"))
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, inbox)
	case "report":
		report, ok := pinkyTerminalArgs(args)
		if len(args.Pos) != 1 || !ok {
			return usageExit("usage: specd pinky report --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --verification-ref <ref> --summary <text> [--changed-files <csv>] [--git-head <sha>] [--duration-ms <n>] [--host-tokens <n>] [--host-cost <usd>] [--json]")
		}
		event, err := core.RecordPinkyTerminalReport(root, report, cfg)
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, event)
	case "release":
		sessionID, workerID, attempt, ok := pinkyLeaseArgs(args)
		if len(args.Pos) != 1 || !ok {
			return usageExit("usage: specd pinky release --session <id> --worker <id> --attempt <n>")
		}
		if err := core.ReleasePinkyClaim(root, sessionID, workerID, attempt); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	default:
		return usageExit("usage: specd pinky <claim|brief|heartbeat|progress|query|report|block|release|inbox> ...")
	}
}

// runPinkyBrief renders a paste-ready worker brief (and, with --json, the
// claimable mission JSON) for one mission. It bridges a Brain dispatch decision
// to an actual worker prompt without any ad-hoc context assembly (GAP-3). It
// builds either an execution mission (--task) or a planning authoring mission
// (--artifact).
func runPinkyBrief(root string, cfg core.OrchestrationCfg, args cli.Args) int {
	if len(args.Pos) != 1 {
		return usageExit("usage: specd pinky brief --session <id> --worker <id> --spec <slug> (--task <id> [--attempt <n>] | --artifact <name>) [--json]")
	}
	session, worker, spec := args.Str("session"), args.Str("worker"), args.Str("spec")
	if session == "" || worker == "" || spec == "" {
		return usageExit("specd pinky brief: --session, --worker, and --spec are required")
	}
	task, artifact := args.Str("task"), args.Str("artifact")
	if (task == "") == (artifact == "") {
		return usageExit("specd pinky brief: pass exactly one of --task or --artifact")
	}

	var mission core.PinkyMission
	var err error
	if artifact != "" {
		mission, err = core.BuildAuthoringMission(root, spec, session, worker, artifact, cfg)
	} else {
		attempt := 1
		if args.Has("attempt") {
			n, ok := parsePositiveIntFlag(args, "attempt")
			if !ok {
				return usageExit("specd pinky brief: --attempt must be a positive integer")
			}
			attempt = n
		}
		mission, err = core.BuildPinkyMission(root, spec, session, worker, task, attempt, cfg)
	}
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		if err := core.PrintJSON(mission); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	fmt.Print(core.RenderMissionBrief(mission))
	return core.ExitOK
}

func readPinkyMission(path string) (core.PinkyMission, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = os.ReadFile("/dev/stdin")
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return core.PinkyMission{}, err
	}
	var mission core.PinkyMission
	if err := json.Unmarshal(raw, &mission); err != nil {
		return core.PinkyMission{}, err
	}
	return mission, nil
}

func pinkyLeaseArgs(args cli.Args) (string, string, int, bool) {
	sessionID, workerID := args.Str("session"), args.Str("worker")
	attempt, ok := parsePositiveIntFlag(args, "attempt")
	return sessionID, workerID, attempt, sessionID != "" && workerID != "" && ok
}

func pinkyProgressArgs(args cli.Args) (core.PinkyProgressReport, bool) {
	attempt, ok := parsePositiveIntFlag(args, "attempt")
	if !ok || args.Str("session") == "" || args.Str("worker") == "" || args.Str("spec") == "" || args.Str("task") == "" || args.Str("message") == "" {
		return core.PinkyProgressReport{}, false
	}
	percent, ok := parseNonNegativeIntFlag(args, "percent")
	if !ok || percent > 100 {
		fmt.Println("--percent must be between 0 and 100")
		return core.PinkyProgressReport{}, false
	}
	return core.PinkyProgressReport{
		SessionID: args.Str("session"),
		WorkerID:  args.Str("worker"),
		Spec:      args.Str("spec"),
		TaskID:    args.Str("task"),
		Attempt:   attempt,
		Percent:   percent,
		Message:   args.Str("message"),
	}, true
}

func pinkyBlockArgs(args cli.Args) (core.PinkyBlockerReport, bool) {
	attempt, ok := parsePositiveIntFlag(args, "attempt")
	if !ok || args.Str("session") == "" || args.Str("worker") == "" || args.Str("spec") == "" || args.Str("task") == "" || args.Str("reason") == "" {
		return core.PinkyBlockerReport{}, false
	}
	return core.PinkyBlockerReport{
		SessionID: args.Str("session"),
		WorkerID:  args.Str("worker"),
		Spec:      args.Str("spec"),
		TaskID:    args.Str("task"),
		Attempt:   attempt,
		Reason:    args.Str("reason"),
	}, true
}

func pinkyQueryArgs(args cli.Args) (core.PinkyQueryReport, bool) {
	attempt, ok := parsePositiveIntFlag(args, "attempt")
	if !ok || args.Str("session") == "" || args.Str("worker") == "" || args.Str("spec") == "" || args.Str("task") == "" || args.Str("text") == "" {
		return core.PinkyQueryReport{}, false
	}
	return core.PinkyQueryReport{
		SessionID: args.Str("session"),
		WorkerID:  args.Str("worker"),
		Spec:      args.Str("spec"),
		TaskID:    args.Str("task"),
		Attempt:   attempt,
		Text:      args.Str("text"),
	}, true
}

func pinkyTerminalArgs(args cli.Args) (core.PinkyTerminalReport, bool) {
	attempt, ok := parsePositiveIntFlag(args, "attempt")
	if !ok || args.Str("session") == "" || args.Str("worker") == "" || args.Str("spec") == "" || args.Str("task") == "" || args.Str("verification-ref") == "" || args.Str("summary") == "" {
		return core.PinkyTerminalReport{}, false
	}
	duration := int64(0)
	if args.Str("duration-ms") != "" {
		n, err := strconv.ParseInt(args.Str("duration-ms"), 10, 64)
		if err != nil || n < 0 {
			fmt.Println("--duration-ms must be a non-negative integer")
			return core.PinkyTerminalReport{}, false
		}
		duration = n
	}
	tokens := 0
	if args.Str("host-tokens") != "" {
		var ok bool
		tokens, ok = parseNonNegativeIntFlag(args, "host-tokens")
		if !ok {
			return core.PinkyTerminalReport{}, false
		}
	}
	changed := []string{}
	if args.Str("changed-files") != "" {
		for _, file := range strings.Split(args.Str("changed-files"), ",") {
			if trimmed := strings.TrimSpace(file); trimmed != "" {
				changed = append(changed, trimmed)
			}
		}
	}
	return core.PinkyTerminalReport{
		SessionID:       args.Str("session"),
		WorkerID:        args.Str("worker"),
		Spec:            args.Str("spec"),
		TaskID:          args.Str("task"),
		Attempt:         attempt,
		VerificationRef: args.Str("verification-ref"),
		Summary:         args.Str("summary"),
		ChangedFiles:    changed,
		GitHead:         args.Str("git-head"),
		DurationMs:      duration,
		HostTokens:      tokens,
		HostCost:        args.Str("host-cost"),
	}, true
}
