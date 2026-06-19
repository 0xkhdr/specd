package core

import (
	"fmt"
	"strings"
)

// RenderMissionBrief assembles a fully context-engineered worker brief from a
// mission (GAP-3). It is the single place the harness packages a mission for a
// worker agent, so two different hosts hand a worker the same context for the
// same mission. The brief is paste-ready into a sub-agent system prompt: it
// names the role asset to load, the context command to run, the bounded
// contract, the files in scope, the verify command that is the only proof of
// done, and the exact `specd pinky` calls the worker must make.
//
// It performs no IO and no model call — it is a pure rendering of an already
// validated mission.
func RenderMissionBrief(mission PinkyMission) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Pinky mission brief — %s / %s\n\n", mission.Spec, mission.TaskID)
	fmt.Fprintf(&sb, "You are a Pinky worker. Execute exactly this one mission under lease, then report evidence.\n\n")

	sb.WriteString("## Identity\n")
	fmt.Fprintf(&sb, "- role: **%s**%s\n", mission.Role, readOnlySuffix(mission.Authority.ReadOnly))
	fmt.Fprintf(&sb, "- session: `%s`\n", mission.SessionID)
	fmt.Fprintf(&sb, "- worker: `%s`\n", mission.WorkerID)
	fmt.Fprintf(&sb, "- spec / work: `%s` / `%s` (attempt %d)\n", mission.Spec, mission.TaskID, mission.Attempt)
	if mission.Title != "" {
		fmt.Fprintf(&sb, "- title: %s\n", mission.Title)
	}

	sb.WriteString("\n## Context to load (in order)\n")
	fmt.Fprintf(&sb, "1. Role contract: `.specd/roles/%s.md`\n", mission.Role)
	sb.WriteString("2. Pinky skill: `.specd/skills/specd-pinky/SKILL.md`\n")
	fmt.Fprintf(&sb, "3. Spec context: run `%s`\n", mission.ContextCommand)
	if len(mission.Files) > 0 {
		fmt.Fprintf(&sb, "4. Files in scope: %s\n", strings.Join(backticked(mission.Files), ", "))
	}

	sb.WriteString("\n## Contract (bounded — do only this)\n")
	fmt.Fprintf(&sb, "%s\n", mission.Contract)
	if mission.Acceptance != "" {
		sb.WriteString("\n## Acceptance\n")
		fmt.Fprintf(&sb, "%s\n", mission.Acceptance)
	}
	if len(mission.Dependencies) > 0 {
		fmt.Fprintf(&sb, "\nDepends on (already complete): %s\n", strings.Join(backticked(mission.Dependencies), ", "))
	}

	sb.WriteString("\n## Authority\n")
	fmt.Fprintf(&sb, "- allowed actions: %s\n", strings.Join(mission.Authority.AllowedActions, ", "))
	fmt.Fprintf(&sb, "- read-only: %t\n", mission.Authority.ReadOnly)

	sb.WriteString("\n## Proof of done (the ONLY evidence that counts)\n")
	fmt.Fprintf(&sb, "Run `%s`. Your own stdout, checkbox edits, and direct state writes are never evidence.\n", mission.VerifyCommand)

	sb.WriteString("\n## Lifecycle (run these exactly)\n")
	fmt.Fprintf(&sb, "1. Claim:   `specd pinky claim --mission <mission.json>`\n")
	fmt.Fprintf(&sb, "2. Heartbeat every %ds while working: `specd pinky heartbeat %s --worker %s --attempt %d`\n", mission.HeartbeatEvery, mission.SessionID, mission.WorkerID, mission.Attempt)
	fmt.Fprintf(&sb, "3. On a bounded blocker: `specd pinky block ...`\n")
	fmt.Fprintf(&sb, "4. On done (after verify passes): `specd pinky report ...` with the verify record, then `specd pinky release ...`\n")
	fmt.Fprintf(&sb, "\nMission JSON to claim with is emitted by `specd pinky brief ... --json`.\n")
	return sb.String()
}

func readOnlySuffix(readonly bool) string {
	if readonly {
		return " (read-only)"
	}
	return ""
}

func backticked(items []string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = "`" + it + "`"
	}
	return out
}
