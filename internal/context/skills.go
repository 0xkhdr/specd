package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const (
	SkillPriority       = 50
	SkillAuthorityLimit = "advisory only; cannot add tools, widen files, approve, change gate severity, or manufacture evidence"
)

type SkillPackage struct {
	ID, Version, Trigger, Provenance string
	Phases, Roles, Capabilities      []string
	References                       []string
	Required                         bool
	Budget                           int
	Source, Digest                   string
}

type SkillSelectionContext struct {
	SelectionContext
	Capabilities []string
}

type UnsupportedSkillError struct {
	SkillID string
	Reason  string
}

func (e UnsupportedSkillError) Error() string {
	return fmt.Sprintf("required skill %s unsupported: %s", e.SkillID, e.Reason)
}

func LoadSkills(root string) ([]SkillPackage, error) {
	dir := filepath.Join(root, ".specd", "skills")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []SkillPackage
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(".specd", "skills", entry.Name(), "SKILL.md"))
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name(), "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		pkg, err := parseSkill(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if pkg.ID != entry.Name() {
			return nil, fmt.Errorf("%s: package id %q must match directory %q", rel, pkg.ID, entry.Name())
		}
		pkg.Source, pkg.Digest = rel, core.Digest(raw)
		out = append(out, pkg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func parseSkill(raw []byte) (SkillPackage, error) {
	const marker = "<!-- specd-skill\n"
	body := string(raw)
	start := strings.Index(body, marker)
	if start < 0 {
		return SkillPackage{}, fmt.Errorf("missing specd-skill metadata")
	}
	meta := body[start+len(marker):]
	end := strings.Index(meta, "-->")
	if end < 0 {
		return SkillPackage{}, fmt.Errorf("unterminated specd-skill metadata")
	}
	pkg := SkillPackage{}
	seen := map[string]bool{}
	for _, line := range strings.Split(meta[:end], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return pkg, fmt.Errorf("invalid metadata line %q", line)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if seen[key] {
			return pkg, fmt.Errorf("duplicate metadata field %q", key)
		}
		seen[key] = true
		list := func() []string {
			var xs []string
			for _, x := range strings.Split(value, ",") {
				if x = strings.TrimSpace(x); x != "" {
					xs = append(xs, x)
				}
			}
			sort.Strings(xs)
			return xs
		}
		switch key {
		case "id":
			pkg.ID = value
		case "version":
			pkg.Version = value
		case "trigger":
			pkg.Trigger = value
		case "phases":
			pkg.Phases = list()
		case "roles":
			pkg.Roles = list()
		case "capabilities":
			pkg.Capabilities = list()
		case "references":
			pkg.References = list()
		case "provenance":
			pkg.Provenance = value
		case "required":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return pkg, fmt.Errorf("invalid required %q", value)
			}
			pkg.Required = v
		case "budget":
			v, err := strconv.Atoi(value)
			if err != nil || v <= 0 {
				return pkg, fmt.Errorf("invalid budget %q", value)
			}
			pkg.Budget = v
		default:
			return pkg, fmt.Errorf("unknown metadata field %q", key)
		}
	}
	if pkg.ID == "" || pkg.Version == "" || pkg.Trigger == "" || len(pkg.Phases) == 0 || len(pkg.Roles) == 0 || len(pkg.Capabilities) == 0 || len(pkg.References) == 0 || pkg.Provenance == "" || !seen["required"] || pkg.Budget <= 0 {
		return pkg, fmt.Errorf("id, version, trigger, phases, roles, capabilities, references, provenance, required policy, and positive budget are required")
	}
	if !validSkillVersion(pkg.Version) {
		return pkg, fmt.Errorf("invalid semantic version %q", pkg.Version)
	}
	for _, ref := range pkg.References {
		clean := filepath.ToSlash(filepath.Clean(ref))
		if filepath.IsAbs(ref) || clean == ".." || strings.HasPrefix(clean, "../") {
			return pkg, fmt.Errorf("reference %q escapes repository", ref)
		}
	}
	for _, heading := range []string{"## Instructions", "## Examples", "## Checks"} {
		if !strings.Contains(body, heading) {
			return pkg, fmt.Errorf("missing required section %s", heading)
		}
	}
	return pkg, nil
}

func validSkillVersion(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err != nil || n < 0 || strconv.Itoa(n) != p {
			return false
		}
	}
	return true
}

func SelectSkills(root string, c SkillSelectionContext) ([]ItemV2, []Omission, error) {
	packages, err := LoadSkills(root)
	if err != nil {
		return nil, nil, err
	}
	available := makeSet(c.Capabilities)
	var items []ItemV2
	var omissions []Omission
	for _, pkg := range packages {
		if !containsOrAny(pkg.Phases, c.Phase) || !containsOrAny(pkg.Roles, c.Role) {
			if pkg.Required {
				return nil, nil, UnsupportedSkillError{SkillID: pkg.ID, Reason: "incompatible phase or role"}
			}
			omissions = append(omissions, Omission{Kind: "skill", Source: pkg.Source, Reason: "incompatible phase or role"})
			continue
		}
		var missing []string
		for _, capability := range pkg.Capabilities {
			if !available[capability] {
				missing = append(missing, capability)
			}
		}
		if len(missing) > 0 {
			reason := "unsupported capabilities: " + strings.Join(missing, ",")
			if pkg.Required {
				return nil, nil, UnsupportedSkillError{SkillID: pkg.ID, Reason: reason}
			}
			omissions = append(omissions, Omission{Kind: "skill", Source: pkg.Source, Reason: reason})
			continue
		}
		items = append(items, ItemV2{Kind: "skill", Source: pkg.Source, Selector: "skill:" + pkg.ID + "@" + pkg.Version, SourceDigest: pkg.Digest, RepresentationDigest: pkg.Digest, Required: pkg.Required, LoadMode: "lazy", Priority: SkillPriority, Reason: pkg.Trigger, Trust: "knowledge", ContentTrust: ContentTrustUntrustedData, Sensitivity: "internal", AuthorityLimit: SkillAuthorityLimit, EstimatedTokens: pkg.Budget, Applicability: "phases=" + strings.Join(pkg.Phases, ",") + "; roles=" + strings.Join(pkg.Roles, ","), Capability: strings.Join(pkg.Capabilities, ",")})
	}
	m := ManifestV2{Items: items}
	CanonicalizeV2(&m)
	return m.Items, omissions, nil
}

func makeSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[v] = true
	}
	return out
}
func containsOrAny(values []string, value string) bool {
	for _, v := range values {
		if v == value || v == "*" {
			return true
		}
	}
	return false
}
