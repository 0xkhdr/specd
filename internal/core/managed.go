package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

// TemplateVersion is the current version stamp for specd-managed scaffold assets.
// It rides in every managed-region marker so `init --refresh` can detect a region
// written by an older binary. Bump it whenever a role/steering template changes.
const TemplateVersion = 2

// ManagedAsset is one specd-managed scaffold file (a role or steering template).
// Its Template is wrapped in stable marker comments so `init --repair`/`--refresh`
// can regenerate the managed region while leaving any user content outside the
// markers byte-for-byte untouched (spec 11 R2/R3/R4).
type ManagedAsset struct {
	Name     string // logical asset id, e.g. "roles/craftsman.md"
	RelPath  string // path under the project root, e.g. ".specd/roles/craftsman.md"
	Version  int
	Template string
}

// ManagedAssets enumerates the role and steering templates baked into the binary.
func ManagedAssets() ([]ManagedAsset, error) {
	var assets []ManagedAsset
	for _, base := range []string{"roles", "steering", "maintenance"} {
		entries, err := embedtemplates.FS.ReadDir(base)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := base + "/" + entry.Name()
			raw, err := embedtemplates.FS.ReadFile(name)
			if err != nil {
				return nil, err
			}
			relBase := base
			if base == "maintenance" {
				relBase = filepath.Join("templates", base)
			}
			assets = append(assets, ManagedAsset{
				Name:     name,
				RelPath:  filepath.Join(".specd", relBase, entry.Name()),
				Version:  TemplateVersion,
				Template: string(raw),
			})
		}
	}
	return assets, nil
}

func managedBegin(name string, version int) string {
	return fmt.Sprintf("<!-- specd:managed:%s:v%d begin -->", name, version)
}

func managedEnd(name string, version int) string {
	return fmt.Sprintf("<!-- specd:managed:%s:v%d end -->", name, version)
}

// Block is the marker-wrapped managed region for the asset at its current version.
func (a ManagedAsset) Block() string {
	return managedBegin(a.Name, a.Version) + "\n" + strings.TrimRight(a.Template, "\n") + "\n" + managedEnd(a.Name, a.Version)
}

// Merge returns existing with this asset's managed region replaced by the current
// template block (matching a marker of *any* version, so a refresh restamps an
// older region). Content outside the markers is preserved exactly; when no region
// is present the block is appended. A brand-new file becomes just the block.
func (a ManagedAsset) Merge(existing string) string {
	block := a.Block()
	beginRe := regexp.MustCompile(`<!-- specd:managed:` + regexp.QuoteMeta(a.Name) + `:v\d+ begin -->`)
	endRe := regexp.MustCompile(`<!-- specd:managed:` + regexp.QuoteMeta(a.Name) + `:v\d+ end -->`)
	bLoc := beginRe.FindStringIndex(existing)
	eLoc := endRe.FindStringIndex(existing)
	if bLoc != nil && eLoc != nil && eLoc[1] >= bLoc[0] {
		return existing[:bLoc[0]] + block + existing[eLoc[1]:]
	}
	if strings.TrimSpace(existing) == "" {
		return block + "\n"
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
}

// AssetChange is one file a repair/refresh/dry-run would touch.
type AssetChange struct {
	Name    string
	RelPath string
	Before  string
	After   string
}

// PlanManagedRepair computes the changes needed to bring every managed asset's
// region back in sync with the current templates, without writing anything. A
// file whose managed region already matches is not listed. This is the pure core
// of `init --repair`/`--refresh`/`--dry-run` (spec 11 R3/R4/R5).
func PlanManagedRepair(root string) ([]AssetChange, error) {
	assets, err := ManagedAssets()
	if err != nil {
		return nil, err
	}
	var changes []AssetChange
	for _, asset := range assets {
		target := filepath.Join(root, asset.RelPath)
		before := ""
		if raw, err := os.ReadFile(target); err == nil {
			before = string(raw)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		after := asset.Merge(before)
		if after != before {
			changes = append(changes, AssetChange{Name: asset.Name, RelPath: asset.RelPath, Before: before, After: after})
		}
	}
	return changes, nil
}

// ApplyManagedRepair writes every planned change atomically and returns the list
// of files it touched. It is the mutating counterpart to PlanManagedRepair.
func ApplyManagedRepair(root string) ([]AssetChange, error) {
	changes, err := PlanManagedRepair(root)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		target := filepath.Join(root, change.RelPath)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := AtomicWrite(target, change.After); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

// Unifiedish renders a minimal line-oriented diff for a managed asset change, so
// `--dry-run` shows what would change before any write (spec 11 R5). It is not a
// full unified diff — it flags added/removed lines by prefix, which is enough for
// an operator to see the managed-region delta.
func Unifiedish(change AssetChange) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", change.RelPath)
	beforeLines := strings.Split(change.Before, "\n")
	afterLines := strings.Split(change.After, "\n")
	before := map[string]int{}
	for _, line := range beforeLines {
		before[line]++
	}
	after := map[string]int{}
	for _, line := range afterLines {
		after[line]++
	}
	for _, line := range beforeLines {
		if after[line] == 0 {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	for _, line := range afterLines {
		if before[line] == 0 {
			fmt.Fprintf(&b, "+ %s\n", line)
		}
	}
	return b.String()
}
