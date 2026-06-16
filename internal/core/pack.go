package core

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

//go:embed embed_packs
var packFS embed.FS

// PackFile is one scaffold file a pack writes, relative to the project root.
// Content is inline and declarative — packs never reference scripts, commands,
// or hooks (see ParsePack), so applying a pack can only ever write files.
type PackFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Pack is a declarative spec-scaffold bundle. The manifest is pure data: a set
// of files to write plus template variables. It is intentionally NOT executable
// — there is no hook, command, or script field — so resolving and applying a
// pack carries no code-execution surface.
type Pack struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Files       []PackFile        `json:"files"`
	Vars        map[string]string `json:"vars,omitempty"`
}

// forbiddenPackKeys are executable-intent fields a pack manifest must never
// carry. DisallowUnknownFields already rejects them, but matching them
// explicitly lets ParsePack emit a precise "declarative-only" diagnostic.
var forbiddenPackKeys = []string{
	"hooks", "hook", "exec", "run", "command", "commands",
	"script", "scripts", "preinstall", "postinstall", "preapply", "postapply",
}

// ParsePack decodes and validates a pack manifest. It fails closed: unknown
// fields (including any executable-hook key), unsafe file paths, and missing
// required fields are all rejected with no partial Pack returned.
func ParsePack(raw []byte) (*Pack, error) {
	// First pass: detect executable-intent keys for a precise error before the
	// generic unknown-field rejection.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, GateError(fmt.Sprintf("pack manifest is not valid JSON: %v", err))
	}
	forbidden := sliceToSet(forbiddenPackKeys)
	for k := range probe {
		if forbidden[strings.ToLower(k)] {
			return nil, GateError(fmt.Sprintf("pack manifest is declarative-only: field %q (executable hooks) is not allowed", k))
		}
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p Pack
	if err := dec.Decode(&p); err != nil {
		return nil, GateError(fmt.Sprintf("invalid pack manifest: %v", err))
	}

	if strings.TrimSpace(p.Name) == "" {
		return nil, GateError("pack manifest missing required field: name")
	}
	if strings.TrimSpace(p.Version) == "" {
		return nil, GateError("pack manifest missing required field: version")
	}
	if len(p.Files) == 0 {
		return nil, GateError(fmt.Sprintf("pack %q declares no files", p.Name))
	}
	seen := map[string]bool{}
	for _, f := range p.Files {
		if err := validatePackPath(f.Path); err != nil {
			return nil, err
		}
		if seen[f.Path] {
			return nil, GateError(fmt.Sprintf("pack %q declares duplicate file path %q", p.Name, f.Path))
		}
		seen[f.Path] = true
	}
	return &p, nil
}

// validatePackPath rejects any file path that could escape the project root:
// absolute paths, "..", and paths that do not stay within the tree once cleaned.
func validatePackPath(p string) error {
	if p == "" {
		return GateError("pack file has empty path")
	}
	if path.IsAbs(p) || strings.HasPrefix(p, "/") {
		return GateError(fmt.Sprintf("pack file path %q must be relative", p))
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return GateError(fmt.Sprintf("pack file path %q escapes the project root", p))
	}
	if clean != p {
		return GateError(fmt.Sprintf("pack file path %q is not in canonical form (want %q)", p, clean))
	}
	return nil
}

// BuiltinPacks returns the embedded built-in packs, sorted by name. A malformed
// embedded pack is a build/test failure surfaced here, not a silent skip.
func BuiltinPacks() ([]*Pack, error) {
	entries, err := packFS.ReadDir("embed_packs")
	if err != nil {
		return nil, err
	}
	var packs []*Pack
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := packFS.ReadFile("embed_packs/" + e.Name())
		if err != nil {
			return nil, err
		}
		p, err := ParsePack(b)
		if err != nil {
			return nil, GateError(fmt.Sprintf("embedded pack %s: %v", e.Name(), err))
		}
		packs = append(packs, p)
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].Name < packs[j].Name })
	return packs, nil
}

// BuiltinPack returns the embedded pack with the given name, or an error if no
// such built-in exists.
func BuiltinPack(name string) (*Pack, error) {
	packs, err := BuiltinPacks()
	if err != nil {
		return nil, err
	}
	for _, p := range packs {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, NotFoundError(fmt.Sprintf("no built-in pack named %q", name))
}
