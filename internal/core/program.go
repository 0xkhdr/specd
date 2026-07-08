package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

// ProgramSchemaVersion versions the program.json shape, following the same
// forward-migration discipline as state.json (spec 02). Bump it when the shape
// changes and add a migration in LoadProgram.
const ProgramSchemaVersion = 1

// ProgramLink records that From depends on To — To must reach completion before
// From may execute. Links live at the program level, never inside a spec's
// state.json, so each file keeps a single writer (spec 12 R6).
type ProgramLink struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Program is the cross-spec dependency graph. It is stored at
// `.specd/program.json`, written atomically under the root lock (which already
// serializes all harness work for the root, so program state needs no second
// lock and cannot deadlock against a spec lock).
type Program struct {
	SchemaVersion int           `json:"schema_version"`
	Links         []ProgramLink `json:"links"`
}

// ProgramPath is the program-level link store.
func ProgramPath(root string) string {
	return filepath.Join(SpecdDir(root), "program.json")
}

// LoadProgram reads program.json. A missing file is an empty program at the
// current schema version. An unknown (future) schema is an error — fail closed
// rather than silently misread newer state.
func LoadProgram(path string) (Program, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Program{SchemaVersion: ProgramSchemaVersion}, nil
	}
	if err != nil {
		return Program{}, err
	}
	var program Program
	if err := json.Unmarshal(raw, &program); err != nil {
		return Program{}, err
	}
	if program.SchemaVersion == 0 {
		program.SchemaVersion = ProgramSchemaVersion // pre-versioned files migrate forward
	}
	if program.SchemaVersion > ProgramSchemaVersion {
		return Program{}, errors.New("program.json schema is newer than this binary supports")
	}
	return program, nil
}

// SaveProgram writes the program atomically at the current schema version.
func SaveProgram(path string, program Program) error {
	program.SchemaVersion = ProgramSchemaVersion
	raw, err := json.MarshalIndent(program, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, string(raw)+"\n")
}

// HasLink reports whether from→to is already recorded.
func (p Program) HasLink(from, to string) bool {
	for _, link := range p.Links {
		if link.From == from && link.To == to {
			return true
		}
	}
	return false
}

// AddLink records from→to. It is idempotent: a duplicate link is a no-op.
func (p *Program) AddLink(from, to string) {
	if !p.HasLink(from, to) {
		p.Links = append(p.Links, ProgramLink{From: from, To: to})
	}
}

// RemoveLink deletes from→to and reports whether it existed.
func (p *Program) RemoveLink(from, to string) bool {
	for i, link := range p.Links {
		if link.From == from && link.To == to {
			p.Links = append(p.Links[:i], p.Links[i+1:]...)
			return true
		}
	}
	return false
}

// Deps returns the slugs that slug directly depends on (its To edges), sorted.
func (p Program) Deps(slug string) []string {
	var deps []string
	for _, link := range p.Links {
		if link.From == slug {
			deps = append(deps, link.To)
		}
	}
	sort.Strings(deps)
	return deps
}

// WouldCycle reports the cycle path that adding from→to would create, or nil if
// the link is safe. A cycle exists when To already depends (transitively) on
// From: following dependency edges from To reaches From. The returned path reads
// from→to→…→from for printing (spec 12 R2).
func (p Program) WouldCycle(from, to string) []string {
	if from == to {
		return []string{from, to}
	}
	// DFS along dependency edges starting at `to`, looking for `from`.
	visited := map[string]bool{}
	var path []string
	var dfs func(node string) bool
	dfs = func(node string) bool {
		if node == from {
			path = append(path, node)
			return true
		}
		if visited[node] {
			return false
		}
		visited[node] = true
		path = append(path, node)
		for _, dep := range p.Deps(node) {
			if dfs(dep) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}
	if dfs(to) {
		return append([]string{from}, path...)
	}
	return nil
}

// Frontier returns the specs that are actionable now: not yet complete and with
// every dependency complete. complete is injected by the caller (the same
// all-gates-green + all-tasks-complete predicate `submit` uses), keeping this
// pure over the graph with no gate logic in core (spec 12 R4).
func (p Program) Frontier(specs []string, complete func(string) bool) []string {
	var frontier []string
	for _, slug := range specs {
		if complete(slug) {
			continue
		}
		if len(p.IncompleteDeps(slug, complete)) == 0 {
			frontier = append(frontier, slug)
		}
	}
	sort.Strings(frontier)
	return frontier
}

// IncompleteDeps returns slug's direct dependencies that are not yet complete —
// the specs blocking it from executing (spec 12 R5).
func (p Program) IncompleteDeps(slug string, complete func(string) bool) []string {
	var blocking []string
	for _, dep := range p.Deps(slug) {
		if !complete(dep) {
			blocking = append(blocking, dep)
		}
	}
	return blocking
}
