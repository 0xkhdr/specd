package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InventoryFileName is the CLI-owned, deterministic inventory of a legacy
// codebase brought under the harness (V10/P5.3). The binary only *inventories*
// (countable facts); the agent *understands* (semantics via the skill); the gate
// *enforces coverage* (countable). This is the boot/enrich lesson codified —
// zero perception in the binary (invariant 1).
const InventoryFileName = "inventory.json"

// MaxManifestBytes caps how much of a manifest file the stdlib parsers read.
// Manifests are small; a large one is skipped for module-name extraction (its
// size still appears in the file inventory).
const MaxManifestBytes = 256 * 1024

// InventoryFile is one countable file fact: its repo-relative path and byte size.
type InventoryFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Inventory is the deterministic projection of an ingested tree. Files and
// Modules are sorted so the same tree yields byte-identical inventory.json.
// Waivers maps an inventory path to the reason it is intentionally excluded from
// requirement coverage (same discipline as the security allowlist).
type Inventory struct {
	Base    string            `json:"base"`
	Files   []InventoryFile   `json:"files"`
	Modules []string          `json:"modules,omitempty"`
	Waivers map[string]string `json:"waivers,omitempty"`
}

// InventoryPath is the inventory file for a spec.
func InventoryPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), InventoryFileName)
}

// BuildInventory produces the deterministic inventory for baseDir given the
// already-resolved repo-relative file list (the caller scopes via `git ls-files`
// or a bounded walk). It stats each file, skips symlinks and unreadable entries,
// and extracts module names from recognized manifests with stdlib only. It never
// reads source semantics — only countable facts.
func BuildInventory(baseDir, base string, relFiles []string) (Inventory, error) {
	inv := Inventory{Base: base}
	seen := map[string]bool{}
	moduleSet := map[string]bool{}
	files := append([]string(nil), relFiles...)
	sort.Strings(files)
	for _, rel := range files {
		rel = filepath.ToSlash(rel)
		if rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		abs := filepath.Join(baseDir, filepath.FromSlash(rel))
		info, err := os.Lstat(abs)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue // symlinks are not followed (explicit policy, V10 §5)
		}
		inv.Files = append(inv.Files, InventoryFile{Path: rel, Size: info.Size()})
		if mod := manifestModule(abs, rel); mod != "" {
			moduleSet[mod] = true
		}
	}
	inv.Modules = sortedSetKeys(moduleSet)
	return inv, nil
}

// manifestModule extracts a module/package name from a recognized manifest, or
// "" when the file is not a manifest or has no parseable name. Stdlib parsers
// only — hostile-input hardened (see ingest fuzz tests).
func manifestModule(abs, rel string) string {
	name := filepath.Base(rel)
	data, err := readCapped(abs, MaxManifestBytes)
	if err != nil {
		return ""
	}
	switch name {
	case "go.mod":
		return goModModule(data)
	case "package.json":
		return packageJSONName(data)
	case "Cargo.toml", "pyproject.toml":
		return tomlPackageName(data)
	}
	return ""
}

// readCapped reads at most max bytes from path.
func readCapped(path string, max int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readAllCapped(f, max)
}

func readAllCapped(f *os.File, max int) ([]byte, error) {
	buf := make([]byte, max+1)
	n := 0
	for n <= max {
		m, err := f.Read(buf[n:])
		n += m
		if err != nil {
			break
		}
		if n > max {
			break
		}
	}
	if n > max {
		n = max
	}
	return buf[:n], nil
}

// goModModule parses the `module` directive from a go.mod. It scans lines and
// tolerates comments/blank lines; anything unexpected yields "".
func goModModule(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

// packageJSONName parses the top-level "name" from a package.json via
// encoding/json — no field is trusted beyond the string name.
func packageJSONName(data []byte) string {
	var doc struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Name)
}

// tomlPackageName extracts a `name = "..."` under a `[package]` table without a
// TOML dependency (zero external deps, invariant 3). It only reads the first
// name in the package table; malformed input yields "".
func tomlPackageName(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	inPackage := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inPackage = line == "[package]" || line == "[project]" || line == "[tool.poetry]"
			continue
		}
		if !inPackage || !strings.HasPrefix(line, "name") {
			continue
		}
		if eq := strings.IndexByte(line, '='); eq >= 0 {
			val := strings.TrimSpace(line[eq+1:])
			val = strings.Trim(val, `"'`)
			if val != "" {
				return val
			}
		}
	}
	return ""
}

// MarshalInventory renders the inventory as deterministic, indented JSON with a
// trailing newline — byte-identical for the same inventory (V10 §5).
func MarshalInventory(inv Inventory) ([]byte, error) {
	out, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// LoadInventory reads and decodes a spec's inventory.json. A missing file is a
// nil inventory (feature inert), not an error.
func LoadInventory(root, slug string) (*Inventory, error) {
	data, err := os.ReadFile(InventoryPath(root, slug))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, fmt.Errorf("inventory.json: %w", err)
	}
	return &inv, nil
}

// IngestCoverage is the countable coverage result over an inventory: which files
// a requirement references, which are waived (with a reason), and which are
// unmapped. Mapped = the file path appears verbatim in requirements.md.
type IngestCoverage struct {
	Mapped   []string `json:"mapped"`
	Waived   []string `json:"waived"`
	Unmapped []string `json:"unmapped"`
}

// ComputeIngestCoverage is the pure coverage math (V10/P5.3): every inventory
// file is mapped (referenced by ≥1 requirement) or waived (has a reason) or
// unmapped. A waiver with an empty reason does not count — reason strings are
// mandatory (same discipline as the security allowlist).
func ComputeIngestCoverage(inv Inventory, requirementsMd string) IngestCoverage {
	var cov IngestCoverage
	for _, f := range inv.Files {
		switch {
		case strings.Contains(requirementsMd, f.Path):
			cov.Mapped = append(cov.Mapped, f.Path)
		case strings.TrimSpace(inv.Waivers[f.Path]) != "":
			cov.Waived = append(cov.Waived, f.Path)
		default:
			cov.Unmapped = append(cov.Unmapped, f.Path)
		}
	}
	return cov
}

// GateIngest is the opt-in ingestion-coverage gate. It is a no-op unless
// cfg.Gates.Ingest names a severity and the spec has an inventory.json. When
// enabled it flags every inventory file that no requirement references and no
// waiver excuses — coverage as a countable fact (V10/P5.3).
func GateIngest(c CheckCtx) (violations, warnings []Violation) {
	mode := c.Cfg.Gates.Ingest
	if mode == "" || mode == "off" || mode == "*" {
		return nil, nil
	}
	inv, err := LoadInventory(c.Root, c.Slug)
	if err != nil {
		return []Violation{{Gate: "ingest", Location: InventoryFileName, Message: err.Error()}}, nil
	}
	if inv == nil {
		return nil, nil
	}
	reqMd := ""
	if c.ReqMd != nil {
		reqMd = *c.ReqMd
	}
	cov := ComputeIngestCoverage(*inv, reqMd)
	if len(cov.Unmapped) == 0 {
		return nil, nil
	}
	v := Violation{
		Gate:     "ingest",
		Location: InventoryFileName,
		Message: fmt.Sprintf("%d inventory file(s) neither referenced by a requirement nor waived: %s",
			len(cov.Unmapped), strings.Join(firstN(cov.Unmapped, 5), ", ")),
	}
	if mode == "error" {
		return []Violation{v}, nil
	}
	return nil, []Violation{v}
}

// firstN returns up to n elements of ss (for bounded diagnostic messages).
func firstN(ss []string, n int) []string {
	if len(ss) <= n {
		return ss
	}
	return ss[:n]
}
