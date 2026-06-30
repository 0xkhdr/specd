package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
)

// ProgramVersion is the current schema version for program.json.
const ProgramVersion = 1

// ProgramManifest is the persisted program.json document: the schema version
// and the declared cross-spec dependency edges, keyed by spec slug.
type ProgramManifest struct {
	Version   int                 `json:"version"`
	DependsOn map[string][]string `json:"dependsOn"`
}

// ProgramPath returns the path to program.json under the spec root's .specd
// directory.
func ProgramPath(root string) string {
	return filepath.Join(SpecdDir(root), "program.json")
}

// LoadProgram reads and parses program.json, returning a default manifest
// with an empty dependency map when the file does not exist. Each spec's
// dependency list is deduplicated before it is returned.
func LoadProgram(root string) (ProgramManifest, error) {
	raw := ReadOrNull(ProgramPath(root))
	if raw == nil {
		return ProgramManifest{Version: ProgramVersion, DependsOn: map[string][]string{}}, nil
	}
	var parsed ProgramManifest
	if err := json.Unmarshal([]byte(*raw), &parsed); err != nil {
		return ProgramManifest{}, GateError(fmt.Sprintf("corrupt program.json: %v", err))
	}
	// Deduplicate edge lists.
	for slug, deps := range parsed.DependsOn {
		seen := make(map[string]bool)
		var uniq []string
		for _, d := range deps {
			if !seen[d] {
				seen[d] = true
				uniq = append(uniq, d)
			}
		}
		parsed.DependsOn[slug] = uniq
	}
	if parsed.DependsOn == nil {
		parsed.DependsOn = map[string][]string{}
	}
	return parsed, nil
}

// SaveProgram writes manifest to program.json, sorting spec slugs and
// deduplicating/sorting each spec's dependency list so the file is
// deterministic and diff-stable across saves.
func SaveProgram(root string, manifest ProgramManifest) error {
	dependsOn := make(map[string][]string)
	slugs := make([]string, 0, len(manifest.DependsOn))
	for slug := range manifest.DependsOn {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		deps := manifest.DependsOn[slug]
		seen := make(map[string]bool)
		var uniq []string
		for _, d := range deps {
			if !seen[d] {
				seen[d] = true
				uniq = append(uniq, d)
			}
		}
		sort.Strings(uniq)
		if len(uniq) > 0 {
			dependsOn[slug] = uniq
		}
	}
	out := map[string]interface{}{"version": manifest.Version, "dependsOn": dependsOn}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(ProgramPath(root), string(b)+"\n")
}

// SpecNode is one spec's resolved position in the program graph: its current
// status, its filtered (known-only) dependencies, its computed wave, and
// whether it has reached completion.
type SpecNode struct {
	Slug      string     `json:"slug"`
	Status    SpecStatus `json:"status"`
	DependsOn []string   `json:"dependsOn"`
	Wave      int        `json:"wave"`
	Complete  bool       `json:"complete"`
}

// ProgramGraph is the resolved cross-spec dependency graph for a program: its
// specs ordered by wave then slug, the equivalent DAG task list, any
// dependency edges pointing at unknown specs (orphans), and any dependency
// cycle that was detected.
type ProgramGraph struct {
	Specs   []SpecNode
	Dag     []DagTask
	Orphans []struct{ Spec, Dep string }
	Cycle   []string
}

func specStatusToTask(s SpecStatus) TaskStatus {
	switch s {
	case StatusComplete:
		return TaskComplete
	case StatusBlocked:
		return TaskBlocked
	default:
		return TaskPending
	}
}

func deriveWaves(edges map[string][]string, slugs []string) map[string]int {
	wave := make(map[string]int, len(slugs))
	visiting := make(map[string]bool)
	var compute func(slug string) int
	compute = func(slug string) int {
		if v, ok := wave[slug]; ok {
			return v
		}
		if visiting[slug] {
			return 1
		}
		visiting[slug] = true
		w := 1
		for _, dep := range edges[slug] {
			dw := compute(dep) + 1
			if dw > w {
				w = dw
			}
		}
		delete(visiting, slug)
		wave[slug] = w
		return w
	}
	for _, s := range slugs {
		compute(s)
	}
	return wave
}

// BuildProgram loads (or reuses the given) program manifest and combines it
// with each spec's on-disk state to produce a ProgramGraph: it computes
// dependency waves, filters out edges to unknown specs as orphans, detects
// cycles, and returns specs sorted by wave then slug.
func BuildProgram(root string, manifest *ProgramManifest) (ProgramGraph, error) {
	if manifest == nil {
		m, err := LoadProgram(root)
		if err != nil {
			return ProgramGraph{}, err
		}
		manifest = &m
	}
	slugs := ListSpecs(root)
	known := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		known[s] = true
	}

	var orphans []struct{ Spec, Dep string }
	edges := make(map[string][]string, len(slugs))
	for _, slug := range slugs {
		declared := manifest.DependsOn[slug]
		for _, dep := range declared {
			if !known[dep] {
				orphans = append(orphans, struct{ Spec, Dep string }{slug, dep})
			}
		}
		var filtered []string
		for _, dep := range declared {
			if known[dep] {
				filtered = append(filtered, dep)
			}
		}
		edges[slug] = filtered
	}

	waves := deriveWaves(edges, slugs)
	statuses := make(map[string]SpecStatus, len(slugs))
	for _, slug := range slugs {
		state, err := LoadState(root, slug)
		if err != nil {
			return ProgramGraph{}, err
		}
		if state == nil {
			// TOCTOU: ListSpecs saw the spec dir, but its state.json has since
			// vanished (concurrent delete). Fail closed with a gate error rather
			// than dereferencing a nil state.
			return ProgramGraph{}, GateError(fmt.Sprintf("state.json for spec '%s' is missing — concurrent delete detected, reload and retry", slug))
		}
		statuses[slug] = state.Status
	}

	dag := make([]DagTask, len(slugs))
	for i, slug := range slugs {
		dag[i] = DagTask{
			ID:      slug,
			Wave:    waves[slug],
			Depends: edges[slug],
			Status:  specStatusToTask(statuses[slug]),
		}
	}
	cycle := DetectCycle(dag)

	specs := make([]SpecNode, len(slugs))
	for i, slug := range slugs {
		specs[i] = SpecNode{
			Slug:      slug,
			Status:    statuses[slug],
			DependsOn: edges[slug],
			Wave:      waves[slug],
			Complete:  statuses[slug] == StatusComplete,
		}
	}
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Wave != specs[j].Wave {
			return specs[i].Wave < specs[j].Wave
		}
		return specs[i].Slug < specs[j].Slug
	})

	return ProgramGraph{Specs: specs, Dag: dag, Orphans: orphans, Cycle: cycle}, nil
}
