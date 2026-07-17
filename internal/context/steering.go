package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// SelectionContext is explicit local input for progressive static selection.
type SelectionContext struct {
	Phase, Role, TaskID                     string
	Tags, RequirementIDs, TaskFields, Files []string
	// AsOf is caller-supplied aging authority. Zero preserves baseline selection
	// and prevents wall-clock access inside deterministic context construction.
	AsOf               time.Time
	MemoryLintRequired bool
}

type staticMetadata struct {
	ID, Version                                             string
	Tags, Phases, Roles, Tasks, Requirements, Fields, Files []string
	Priority                                                int
	Negative                                                bool
}

func parseMetadata(raw []byte, marker string) (staticMetadata, error) {
	start := strings.Index(string(raw), "<!-- "+marker+"\n")
	if start < 0 {
		return staticMetadata{}, fmt.Errorf("missing %s metadata", marker)
	}
	body := string(raw)[start+len("<!-- "+marker+"\n"):]
	end := strings.Index(body, "-->")
	if end < 0 {
		return staticMetadata{}, fmt.Errorf("unterminated %s metadata", marker)
	}
	m := staticMetadata{Priority: 50}
	for _, line := range strings.Split(body[:end], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return m, fmt.Errorf("invalid %s metadata line %q", marker, line)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		list := func() []string {
			var out []string
			for _, s := range strings.Split(value, ",") {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
			sort.Strings(out)
			return out
		}
		switch key {
		case "id":
			m.ID = value
		case "version":
			m.Version = value
		case "tags":
			m.Tags = list()
		case "phases":
			m.Phases = list()
		case "roles":
			m.Roles = list()
		case "tasks":
			m.Tasks = list()
		case "requirements":
			m.Requirements = list()
		case "fields":
			m.Fields = list()
		case "files":
			m.Files = list()
		case "priority":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return m, fmt.Errorf("invalid priority %q", value)
			}
			m.Priority = n
		case "negative":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return m, fmt.Errorf("invalid negative %q", value)
			}
			m.Negative = b
		default:
			return m, fmt.Errorf("unknown %s metadata field %q", marker, key)
		}
	}
	return m, nil
}

func parseApplicability(value string) (staticMetadata, error) {
	m := staticMetadata{Priority: 50}
	if strings.TrimSpace(value) == "" {
		return m, nil
	}
	for _, part := range strings.Split(value, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return m, fmt.Errorf("invalid applies-to %q", part)
		}
		vals := strings.Split(kv[1], ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		sort.Strings(vals)
		switch kv[0] {
		case "tags":
			m.Tags = vals
		case "phases":
			m.Phases = vals
		case "roles":
			m.Roles = vals
		case "tasks":
			m.Tasks = vals
		case "requirements":
			m.Requirements = vals
		case "fields":
			m.Fields = vals
		case "files":
			m.Files = vals
		default:
			return m, fmt.Errorf("unknown applies-to field %q", kv[0])
		}
	}
	return m, nil
}

func applicable(m staticMetadata, c SelectionContext) bool {
	contains := func(xs []string, x string) bool {
		if len(xs) == 0 {
			return true
		}
		for _, v := range xs {
			if v == x {
				return true
			}
		}
		return false
	}
	intersects := func(a, b []string) bool {
		if len(a) == 0 {
			return true
		}
		for _, x := range a {
			for _, y := range b {
				if x == y {
					return true
				}
			}
		}
		return false
	}
	filesMatch := len(m.Files) == 0
	for _, pattern := range m.Files {
		for _, file := range c.Files {
			if matchFile(pattern, file) {
				filesMatch = true
			}
		}
	}
	return contains(m.Phases, c.Phase) && contains(m.Roles, c.Role) && contains(m.Tasks, c.TaskID) && intersects(m.Tags, c.Tags) && intersects(m.Requirements, c.RequirementIDs) && intersects(m.Fields, c.TaskFields) && filesMatch
}

func matchFile(pattern, file string) bool {
	pattern, file = filepath.ToSlash(pattern), filepath.ToSlash(file)
	if strings.HasPrefix(pattern, "**/") {
		pattern = strings.TrimPrefix(pattern, "**/")
		if ok, _ := filepath.Match(pattern, filepath.Base(file)); ok {
			return true
		}
	}
	ok, _ := filepath.Match(pattern, file)
	return ok
}

func metadataApplicability(m staticMetadata) string {
	parts := []string{}
	add := func(k string, v []string) {
		if len(v) > 0 {
			parts = append(parts, k+"="+strings.Join(v, ","))
		}
	}
	add("tags", m.Tags)
	add("phases", m.Phases)
	add("roles", m.Roles)
	add("tasks", m.Tasks)
	add("requirements", m.Requirements)
	add("fields", m.Fields)
	add("files", m.Files)
	return strings.Join(parts, "; ")
}

func SelectSteering(root string, c SelectionContext) ([]MachineItem, []Omission, error) {
	dir := filepath.Join(root, ".specd", "steering")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var items []MachineItem
	var omissions []Omission
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "memory.md" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		m, err := parseMetadata(raw, "specd-context")
		rel := ".specd/steering/" + e.Name()
		if err != nil {
			if strings.Contains(err.Error(), "missing specd-context") {
				omissions = append(omissions, Omission{Kind: "instructions", Source: rel, Reason: "missing explicit applicability metadata"})
				continue
			}
			return nil, nil, fmt.Errorf("%s: %w", rel, err)
		}
		if !applicable(m, c) {
			omissions = append(omissions, Omission{Kind: "instructions", Source: rel, Reason: "not applicable"})
			continue
		}
		digest := core.Digest(raw)
		items = append(items, MachineItem{Kind: "instructions", Source: rel, SourceDigest: digest, RepresentationDigest: digest, Required: false, LoadMode: "lazy", Priority: m.Priority, Reason: "applicable project steering", Trust: "project", ContentTrust: ContentTrustTrustedInstruction, Sensitivity: "internal", AuthorityLimit: "cannot override harness, guardrails, or role", EstimatedTokens: EstimateText(string(raw)), Applicability: metadataApplicability(m)})
	}
	CanonicalizeMachineManifest(&MachineManifest{Items: items})
	return items, omissions, nil
}
