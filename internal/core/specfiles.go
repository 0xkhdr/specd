package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	Version            int       `json:"version"`
	DefaultVerify      string    `json:"defaultVerify"`
	Report             ReportCfg `json:"report"`
	Roles              RolesCfg  `json:"roles"`
	PromotionThreshold int       `json:"promotionThreshold"`
	Gates              GatesCfg  `json:"gates"`
}

type ReportCfg struct {
	Format             string `json:"format"`
	AutoRefreshSeconds int    `json:"autoRefreshSeconds"`
}

type RolesCfg struct {
	SubagentMode string `json:"subagentMode"`
}

type GatesCfg struct {
	Traceability string `json:"traceability"`
	Acceptance   string `json:"acceptance"`
}

var DefaultConfig = Config{
	Version:            1,
	DefaultVerify:      "npm test",
	Report:             ReportCfg{Format: "md", AutoRefreshSeconds: 0},
	Roles:              RolesCfg{SubagentMode: "inline"},
	PromotionThreshold: 3,
	Gates:              GatesCfg{Traceability: "warn", Acceptance: "off"},
}

func LoadConfig(root string) Config {
	raw := ReadOrNull(ConfigPath(root))
	if raw == nil {
		return DefaultConfig
	}
	var partial struct {
		Version            *int       `json:"version"`
		DefaultVerify      *string    `json:"defaultVerify"`
		Report             *ReportCfg `json:"report"`
		Roles              *RolesCfg  `json:"roles"`
		PromotionThreshold *int       `json:"promotionThreshold"`
		Gates              *GatesCfg  `json:"gates"`
	}
	if err := json.Unmarshal([]byte(*raw), &partial); err != nil {
		return DefaultConfig
	}
	cfg := DefaultConfig
	if partial.Version != nil {
		cfg.Version = *partial.Version
	}
	if partial.DefaultVerify != nil {
		cfg.DefaultVerify = *partial.DefaultVerify
	}
	if partial.Report != nil {
		if partial.Report.Format != "" {
			cfg.Report.Format = partial.Report.Format
		}
		cfg.Report.AutoRefreshSeconds = partial.Report.AutoRefreshSeconds
	}
	if partial.Roles != nil && partial.Roles.SubagentMode != "" {
		cfg.Roles.SubagentMode = partial.Roles.SubagentMode
	}
	if partial.PromotionThreshold != nil {
		cfg.PromotionThreshold = *partial.PromotionThreshold
	}
	if partial.Gates != nil {
		if partial.Gates.Traceability != "" {
			cfg.Gates.Traceability = partial.Gates.Traceability
		}
		if partial.Gates.Acceptance != "" {
			cfg.Gates.Acceptance = partial.Gates.Acceptance
		}
	}
	return cfg
}

var Artifacts = []string{
	"requirements.md", "design.md", "tasks.md",
	"decisions.md", "memory.md", "mid-requirements.md",
}

func ArtifactPath(root, slug, name string) string {
	return filepath.Join(SpecDir(root, slug), name)
}

func ReadArtifact(root, slug, name string) *string {
	return ReadOrNull(ArtifactPath(root, slug, name))
}

func ReadRole(root, role string) *string {
	return ReadOrNull(filepath.Join(RolesDir(root), role+".md"))
}

func SpecExists(root, slug string) bool {
	_, err := os.Stat(filepath.Join(SpecDir(root, slug), "state.json"))
	return err == nil
}

func RequireSpec(root, slug string) error {
	if !SpecExists(root, slug) {
		return NotFoundError("spec '" + slug + "' not found under .specd/specs/")
	}
	return nil
}

func ListSpecs(root string) []string {
	dir := filepath.Join(root, ".specd", "specs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), "state.json")); err == nil {
				out = append(out, e.Name())
			}
		}
	}
	sort.Strings(out)
	return out
}

func Reconcile(state *State, doc ParsedTasks) {
	next := make(map[string]TaskState, len(doc.Tasks))
	for _, t := range doc.Tasks {
		prev, hasPrev := state.Tasks[t.ID]
		depends := ParseDepends(t.Meta["depends"])
		var reqs []int
		if _, ok := t.Meta["requirements"]; ok {
			reqs = ParseRequirements(t.Meta["requirements"])
		} else if hasPrev {
			reqs = prev.Requirements
		}
		ts := TaskState{
			ID:           t.ID,
			Title:        t.Title,
			Wave:         t.Wave,
			Depends:      depends,
			Requirements: reqs,
			Status:       TaskPending,
		}
		if t.Meta["role"] != "" {
			ts.Role = t.Meta["role"]
		} else if hasPrev {
			ts.Role = prev.Role
		}
		if ts.Role == "" {
			ts.Role = "builder"
		}
		if hasPrev {
			ts.Status = prev.Status
			ts.StartedAt = prev.StartedAt
			ts.FinishedAt = prev.FinishedAt
			ts.Evidence = prev.Evidence
			ts.Verification = prev.Verification
			ts.Blocker = prev.Blocker
		}
		if ts.Depends == nil {
			ts.Depends = []string{}
		}
		if ts.Requirements == nil {
			ts.Requirements = []int{}
		}
		next[t.ID] = ts
	}
	state.Tasks = next
	var blockers []Blocker
	for _, b := range state.Blockers {
		if _, ok := next[b.Task]; ok {
			blockers = append(blockers, b)
		}
	}
	if blockers == nil {
		blockers = []Blocker{}
	}
	state.Blockers = blockers
}

func ParseTasksMd(root, slug string) (ParsedTasks, error) {
	raw := ReadArtifact(root, slug, "tasks.md")
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return ParsedTasks{Title: slug, Tasks: nil}, nil
	}
	return ParseTasks(*raw)
}

type LoadedSpec struct {
	State *State
	Doc   ParsedTasks
}

func LoadSpec(root, slug string) (LoadedSpec, error) {
	if err := RequireSpec(root, slug); err != nil {
		return LoadedSpec{}, err
	}
	return WithSpecLock[LoadedSpec](root, slug, func() (LoadedSpec, error) {
		state, err := LoadState(root, slug)
		if err != nil {
			return LoadedSpec{}, err
		}
		doc, err := ParseTasksMd(root, slug)
		if err != nil {
			return LoadedSpec{}, err
		}
		before, _ := json.Marshal(state.Tasks)
		Reconcile(state, doc)
		after, _ := json.Marshal(state.Tasks)
		if string(before) != string(after) {
			if err := SaveState(root, slug, state); err != nil {
				return LoadedSpec{}, err
			}
		}
		return LoadedSpec{State: state, Doc: doc}, nil
	})
}
