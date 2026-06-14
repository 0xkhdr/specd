package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EvidenceRef points the enriching agent at a deterministic on-disk source it
// should read before authoring a steering section. Nothing here is inferred —
// every ref is a file/dir that exists or a fact already in boot.json.
type EvidenceRef struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // readme | contributing | docs | manifest | tree | framework
	Note string `json:"note,omitempty"`
}

// EnrichTargetBrief is the per-file instruction packet inside an EnrichBrief.
type EnrichTargetBrief struct {
	Target       string   `json:"target"`
	File         string   `json:"file"`
	Marker       string   `json:"marker"`
	State        string   `json:"state"` // stub | enriched | stale
	Sections     []string `json:"sections"`
	Instructions string   `json:"instructions"`
	TokenBudget  int      `json:"tokenBudget"`
}

// EnrichBrief is the deterministic, AI-free contract `specd enrich plan` emits:
// the evidence to read, the targets to author, and how to write them back. The
// binary builds it; the calling agent performs the inference.
type EnrichBrief struct {
	ProjectName string              `json:"projectName"`
	Stacks      []string            `json:"stacks"`
	BootStale   bool                `json:"bootStale"`
	BootNote    string              `json:"bootNote,omitempty"`
	Evidence    []EvidenceRef       `json:"evidence"`
	Targets     []EnrichTargetBrief `json:"targets"`
	ApplyHint   string              `json:"applyHint"`
}

// targetSections lists the headings an agent should author per target. These
// mirror the stub steering templates so enriched output stays consistent.
var targetSections = map[string][]string{
	"product":   {"Product", "Users", "Value / why it exists", "Out of scope"},
	"structure": {"Layout", "Module boundaries", "Naming"},
	"tech":      {"Conventions", "Stack notes"},
}

var targetInstructions = map[string]string{
	"product":   "Author WHAT this product is, who uses it, the problem it solves, and explicit out-of-scope. Infer from README/docs/manifest. Cite evidence; never invent users or scope.",
	"structure": "Describe the real top-level layout, which modules may depend on what, and naming conventions a builder must follow. Infer from the dir tree and imports — not aspiration.",
	"tech":      "Author conventions (style, naming, error-handling) and stack notes beyond the boot-detected block. Do NOT duplicate the `SPECD BOOT` block — it owns the detected stack.",
}

// enrichSources returns the sorted, de-duplicated evidence source paths (repo
// relative) used by enrichment: boot.json sources plus discovered docs.
func enrichSources(root string) []string {
	set := map[string]bool{}
	for _, e := range discoverEvidence(root) {
		if e.Kind == "tree" || e.Kind == "framework" {
			continue
		}
		set[e.Path] = true
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// discoverEvidence scans root for deterministic enrichment evidence: top-level
// README/CONTRIBUTING files, a docs/ directory, manifest sources recorded by
// boot, a shallow dir tree, and detected frameworks.
func discoverEvidence(root string) []EvidenceRef {
	var refs []EvidenceRef

	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		name := e.Name()
		upper := strings.ToUpper(name)
		switch {
		case !e.IsDir() && strings.HasPrefix(upper, "README"):
			refs = append(refs, EvidenceRef{Path: name, Kind: "readme", Note: "product intent, usage"})
		case !e.IsDir() && strings.HasPrefix(upper, "CONTRIBUTING"):
			refs = append(refs, EvidenceRef{Path: name, Kind: "contributing", Note: "conventions, workflow"})
		case e.IsDir() && (name == "docs" || name == "doc"):
			refs = append(refs, EvidenceRef{Path: name + "/", Kind: "docs", Note: "narrative documentation"})
		}
	}

	// Manifest sources recorded by boot (package.json, go.mod, pyproject, ...).
	if raw := ReadOrNull(filepath.Join(SpecdDir(root), "boot.json")); raw != nil {
		var res BootResult
		if json.Unmarshal([]byte(*raw), &res) == nil {
			for _, s := range res.Sources {
				refs = append(refs, EvidenceRef{Path: s, Kind: "manifest", Note: "stack/dependency facts"})
			}
			for stack, fws := range res.Frameworks {
				if len(fws) > 0 {
					refs = append(refs, EvidenceRef{Path: stack, Kind: "framework", Note: strings.Join(fws, ", ")})
				}
			}
		}
	}

	// Shallow top-level directory tree (dirs only) for the structure target.
	var dirs []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() && name != ".specd" && name != ".git" && !strings.HasPrefix(name, ".") {
			dirs = append(dirs, name+"/")
		}
	}
	sort.Strings(dirs)
	if len(dirs) > 0 {
		refs = append(refs, EvidenceRef{Path: strings.Join(dirs, " "), Kind: "tree", Note: "top-level layout"})
	}

	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		return refs[i].Path < refs[j].Path
	})
	return refs
}

// BuildEnrichBrief assembles the deterministic enrichment contract. It reads
// boot.json + the steering files and reports current per-target state, but
// performs no inference. boot must already exist (callers gate on that).
func BuildEnrichBrief(root string) EnrichBrief {
	stacks := []string{}
	project := baseName(root)
	if raw := ReadOrNull(filepath.Join(SpecdDir(root), "boot.json")); raw != nil {
		var res BootResult
		if json.Unmarshal([]byte(*raw), &res) == nil {
			stacks = res.Stacks
			if res.ProjectName != "" {
				project = res.ProjectName
			}
		}
	}

	fresh, ferr := CheckBootFreshness(root)
	bootStale := ferr != nil || fresh.Stale
	bootNote := ""
	if ferr != nil {
		bootNote = "boot.json missing or invalid — run `specd boot` first"
	} else if fresh.Stale {
		bootNote = "boot.json is stale — run `specd boot --force` before relying on this brief"
	}

	rec, hasRec := LoadEnrichRecord(root)
	currentHash := bootHash(root)
	begin := enrichMarkerBegin()

	var targets []EnrichTargetBrief
	for _, key := range EnrichTargetKeys() {
		path, _ := EnrichTargetPath(root, key)
		state := "stub"
		if raw := ReadOrNull(path); raw != nil && strings.Contains(*raw, begin) {
			state = "enriched"
			if hasRec && rec.BootHash != "" && currentHash != "" && rec.BootHash != currentHash {
				state = "stale"
			}
		}
		targets = append(targets, EnrichTargetBrief{
			Target:       key,
			File:         strings.TrimPrefix(path, root+"/"),
			Marker:       "SPECD ENRICH " + EnrichMarkerVersion,
			State:        state,
			Sections:     targetSections[key],
			Instructions: targetInstructions[key],
			TokenBudget:  400,
		})
	}

	return EnrichBrief{
		ProjectName: project,
		Stacks:      stacks,
		BootStale:   bootStale,
		BootNote:    bootNote,
		Evidence:    discoverEvidence(root),
		Targets:     targets,
		ApplyHint:   "For each target: read the evidence, author the sections, then `specd enrich apply --target <key>` (markdown on stdin or --content-file). Verify with `specd enrich status`.",
	}
}
