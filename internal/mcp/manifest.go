package mcp

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

// activeManifest loads the context-manifest tool policy (C1) for the project's
// active spec — the same furthest-along spec the phase watcher tracks. It
// degrades to an empty policy (no filtering) when no root/spec/manifest exists,
// so absent manifests leave the config/phase output unchanged (R5). Read-only.
func activeManifest() core.ContextManifestTools {
	root, slug, _, ok := activeSpec()
	if !ok {
		return core.ContextManifestTools{}
	}
	return core.LoadContextManifest(root, slug)
}

// allToolNames is the universe of tool names the server can emit (command
// mirrors, composites, intents), used to validate manifest/negotiation entries:
// unknown names warn and are ignored rather than silently mis-filtering (C1
// §5.1). Meta commands are never tools, so they are excluded.
func allToolNames() map[string]bool {
	set := make(map[string]bool, len(core.Commands)+len(compositeTools)+len(intentTools))
	for _, c := range core.Commands {
		if metaCommands[c.Command] {
			continue
		}
		set[toolPrefix+c.Command] = true
	}
	for _, ct := range compositeTools {
		set[ct.name] = true
	}
	for _, it := range intentTools {
		set[it.name] = true
	}
	return set
}

// applyManifestFilter restricts a config/phase-built candidate list to a spec's
// contextManifest policy (C1 §5.2). Precedence: forbidden > config-gate >
// required/optional allowlist > phase plan. The candidate set already had config
// meta/orchestration gates applied, so a manifest-required tool absent from it
// was gated off by config — R4 keeps it excluded and emits a stderr diagnostic
// (config safety wins over manifest "required"). Unknown names are ignored with
// a warning. The slice order is preserved so output stays deterministic (R6).
func applyManifestFilter(tools []toolDef, manifest core.ContextManifestTools) []toolDef {
	if !manifest.Present() {
		return tools // R5: no manifest ⇒ identical to config/phase output.
	}
	known := allToolNames()
	present := make(map[string]bool, len(tools))
	for _, t := range tools {
		present[t.Name] = true
	}

	allow := make(map[string]bool)
	collect := func(names []string, field string) {
		for _, n := range names {
			if !known[n] {
				fmt.Fprintf(os.Stderr, "specd mcp: contextManifest %s names unknown tool %q; ignored\n", field, n)
				continue
			}
			allow[n] = true
		}
	}
	collect(manifest.RequiredTools, "requiredTools")
	collect(manifest.OptionalTools, "optionalTools")

	// R4: a required tool the config gate already excluded never reaches the
	// candidate set; surface the conflict, but the config gate still wins.
	for _, n := range manifest.RequiredTools {
		if known[n] && !present[n] {
			fmt.Fprintf(os.Stderr, "specd mcp: contextManifest requires %q but config gating excludes it; honoring config gate\n", n)
		}
	}

	forbidden := make(map[string]bool, len(manifest.ForbiddenTools))
	for _, n := range manifest.ForbiddenTools {
		if !known[n] {
			fmt.Fprintf(os.Stderr, "specd mcp: contextManifest forbiddenTools names unknown tool %q; ignored\n", n)
			continue
		}
		forbidden[n] = true
	}

	filtered := tools[:0:0]
	for _, t := range tools {
		if forbidden[t.Name] { // R3: forbidden wins unconditionally.
			continue
		}
		if !allow[t.Name] { // R2: restrict to required∪optional.
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

// manifestRequiredSet is the set of validated, present-able required tool names
// a manifest declares — the tools host negotiation (C2 R4) must never drop to
// satisfy maxTools.
func manifestRequiredSet(manifest core.ContextManifestTools) map[string]bool {
	if len(manifest.RequiredTools) == 0 {
		return nil
	}
	known := allToolNames()
	set := make(map[string]bool, len(manifest.RequiredTools))
	for _, n := range manifest.RequiredTools {
		if known[n] {
			set[n] = true
		}
	}
	return set
}
