package core

import "os"

// Pre-spec preflight (GAP-6).
//
// SenseOrchestration assumes a loaded spec; it cannot start from a bare repo.
// The preflight closes that bootstrap gap: before a drive begins it reports the
// deterministic, non-creative steps needed to reach a sensable spec — workspace
// init and spec creation — so the harness can run "bare repo → runnable spec →
// driven" instead of requiring a human to hand-run `init`/`new` first.
//
// It is detection only: it performs no mutation and no LLM call. The driver (or
// the host) decides whether to auto-apply the remedies. Authoring the spec's
// *content* (requirements/design/tasks) is the authoring frontier's job
// (GAP-1), not the preflight's.

// PreflightItem is one missing precondition and the deterministic command that
// satisfies it.
type PreflightItem struct {
	Kind    string `json:"kind"`    // "workspace" | "steering" | "spec"
	Message string `json:"message"` // human-readable reason
	Remedy  string `json:"remedy"`  // the `specd` command that fixes it
}

// OrchestrationPreflight reports, in apply order, the preconditions a bare or
// partial repo is missing before slug can be driven. An empty result means the
// spec is ready to sense.
func OrchestrationPreflight(root, slug string) []PreflightItem {
	var items []PreflightItem
	if !dirExists(SpecdDir(root)) {
		items = append(items, PreflightItem{
			Kind:    "workspace",
			Message: ".specd workspace is not initialized",
			Remedy:  "specd init",
		})
	} else if !dirExists(SteeringDir(root)) {
		// Workspace exists but steering was removed — repair restores the
		// scaffolded steering templates.
		items = append(items, PreflightItem{
			Kind:    "steering",
			Message: "steering directory is missing",
			Remedy:  "specd init --repair",
		})
	}
	if !SpecExists(root, slug) {
		items = append(items, PreflightItem{
			Kind:    "spec",
			Message: "spec " + slug + " does not exist",
			Remedy:  "specd new " + slug,
		})
	}
	return items
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
