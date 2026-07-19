package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

func runHandshake(root string, args []string, flags map[string]string) error {
	if len(args) < 1 || len(args) > 2 || args[0] != "bootstrap" {
		return errors.New("usage: handshake bootstrap [<spec>] [--json] [--expect-<identity> <value>]")
	}
	config, _ := core.LoadConfig(configPaths(root), getenv())
	explicit := ""
	if len(args) == 2 {
		explicit = args[1]
	}
	var state *core.State
	var nextCommands []string
	resolution, resolveErr := core.ResolveSpec(root, explicit, os.Getenv("SPECD_SPEC"))
	if resolveErr == nil {
		current, err := core.LoadState(core.StatePath(root, resolution.Slug))
		if err != nil {
			return err
		}
		state = &current
		if guide, err := driverGuideForSpec(root, resolution.Slug); err == nil {
			for _, action := range guide.NextActions {
				nextCommands = append(nextCommands, strings.TrimSpace("specd "+action.Command+" "+strings.Join(action.Args, " ")))
			}
		} else {
			nextCommands = []string{"specd status " + resolution.Slug + " --guide --json"}
		}
	} else if core.FindingCode(resolveErr) == "SPEC_REQUIRED" {
		nextCommands = []string{"specd new <slug> --title <title>"}
	} else {
		return resolveErr
	}
	handshake, err := core.BootstrapHandshakeForRoot(root, config, state, nextCommands)
	if err != nil {
		return err
	}
	activeSlug, revision := "<none>", "<none>"
	if handshake.ActiveSpec != nil {
		activeSlug = handshake.ActiveSpec.Slug
		revision = strconv.FormatInt(handshake.ActiveSpec.Revision, 10)
	}
	preconditions := []struct{ flag, current string }{
		{"binary-version", handshake.Binary.Version},
		{"binary-commit", handshake.Binary.Commit},
		{"state-schema", strconv.Itoa(handshake.StateSchemaVersion)},
		{"context-schema", handshake.ContextSchemaVersion},
		{"template-schema", strconv.Itoa(handshake.TemplateSchemaVersion)},
		{"root", handshake.WorkspaceRoot},
		{"spec", activeSlug},
		{"revision", revision},
		{"palette-digest", handshake.PaletteDigest},
		{"config-digest", handshake.ConfigDigest},
		{"managed-digest", handshake.ManagedDigest},
	}
	for _, precondition := range preconditions {
		if expected, ok := flags["expect-"+precondition.flag]; ok && expected != precondition.current {
			hint := ""
			switch precondition.flag {
			case "palette-digest":
				hint = " (palette digest drift)"
			case "config-digest":
				hint = " (config digest drift)"
			}
			return fmt.Errorf("precondition %s mismatch: expected %s, current %s%s", precondition.flag, expected, precondition.current, hint)
		}
	}

	if flagEnabled(flags, "json") {
		return writeJSON(handshake)
	}
	fmt.Fprintf(os.Stdout, "version: %s\n", handshake.Version)
	fmt.Fprintf(os.Stdout, "palette_digest: %s\n", handshake.PaletteDigest)
	fmt.Fprintf(os.Stdout, "config_digest: %s\n", handshake.ConfigDigest)
	fmt.Fprintf(os.Stdout, "managed_digest: %s\n", handshake.ManagedDigest)
	for _, tool := range handshake.Tools {
		fmt.Fprintf(os.Stdout, "tool: %s\n", tool)
	}
	return nil
}

// guidanceForSpec builds the machine driving guidance for a spec (spec 01
// R6.1): current phase, the artifact it must produce, the machine-legal
// commands, the human-only actions, and the deterministic blockers that stop the
// next approval. Blockers come from the gate registry run for the next gate; the
// guidance never invents them.
func guidanceForSpec(root, slug string) (core.Guidance, error) {
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return core.Guidance{}, err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return core.Guidance{}, err
	}
	var blockers []string
	if next := core.NextStatus(state.Status); next != "" {
		for _, f := range gates.CoreRegistry().Run(buildCheckCtx(root, slug, spec, string(next))) {
			if f.Severity == gates.Error {
				blockers = append(blockers, f.Message)
			}
		}
	}
	g := core.GuidanceForPhase(state.Status, blockers)
	// R6.2: only suggest task-bearing commands (task verify/context) when the
	// spec actually has an executable task. requireTaskGate fails closed before
	// execution is approved; an empty frontier means nothing to run.
	if !hasExecutableTask(root, slug, spec) {
		kept := g.LegalCommands[:0]
		for _, name := range g.LegalCommands {
			if c, ok := core.CommandByName(name); ok && c.RequiresTask {
				continue
			}
			kept = append(kept, name)
		}
		g.LegalCommands = kept
	}
	return g, nil
}

// hasExecutableTask reports whether the spec has a task ready to run: execution
// must be gate-approved and the frontier non-empty.
func hasExecutableTask(root, slug string, spec specData) bool {
	if requireTaskGate(root, slug) != nil {
		return false
	}
	frontier, err := core.FrontierExcluding(spec.Tasks, taskStatus(spec.Tasks), nil)
	return err == nil && len(frontier) > 0
}
