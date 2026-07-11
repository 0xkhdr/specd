package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// ManifestVersion is the on-wire schema version for the context manifest.
//
// W0 V1/V2 compatibility decision (R1.3, R8.2): V1 stays the default
// compatibility renderer — existing plain-path/JSON output is preserved and old
// `.specd/` consumers keep working. The typed V2 contract (Domain 02 W1) is
// added as the authoritative machine contract and will extend the accepted
// version set explicitly. Neither renderer silently reinterprets the other:
// ValidateManifest fails closed on any unknown or unsupported version, so the
// migration is versioned and published rather than inferred. See
// specs/02-context-knowledge-and-skills/design.md ("ManifestVersion").
const ManifestVersion = "1"

type Item struct {
	Kind            string `json:"kind"`
	Path            string `json:"path,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	Role            string `json:"role,omitempty"`
	Verify          string `json:"verify,omitempty"`
	Acceptance      string `json:"acceptance,omitempty"`
	Required        bool   `json:"required,omitempty"`
	Mode            string `json:"mode,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

type Manifest struct {
	Version         string   `json:"version"`
	Mode            string   `json:"mode"`
	Slug            string   `json:"slug"`
	TaskID          string   `json:"task_id"`
	Items           []Item   `json:"items"`
	Notes           []string `json:"notes,omitempty"`
	EstimatedTokens int      `json:"estimated_tokens"`
}

// BuildManifest assembles the context references for one task. The steering
// constitution and memory (R4.3) enter as references + modes, never inlined
// content, bounded against maxTokens: when over budget, memory drops before
// steering (constitution wins), deterministically, with a note. maxTokens <= 0
// disables budget enforcement.
func BuildManifest(root, slug string, tasks []core.TaskRow, taskID string, maxTokens int) (Manifest, error) {
	task, ok := findTask(tasks, taskID)
	if !ok {
		return Manifest{}, fmt.Errorf("task %s not found", taskID)
	}
	mode := ModeForTask(task)
	items := []Item{
		{Kind: "spec", Path: fmt.Sprintf(".specd/specs/%s/requirements.md", slug), Required: true},
		{Kind: "design", Path: fmt.Sprintf(".specd/specs/%s/design.md", slug), Required: true},
		{Kind: "tasks", Path: fmt.Sprintf(".specd/specs/%s/tasks.md", slug), Required: true},
		{Kind: "task", TaskID: task.ID, Role: task.Role, Verify: task.Verify, Acceptance: task.Acceptance, Required: true},
		{Kind: "role", Path: fmt.Sprintf(".specd/roles/%s.md", task.Role), Required: true},
	}
	for _, file := range task.DeclaredFiles {
		kind := "source"
		if strings.HasSuffix(file, "_test.go") || strings.Contains(file, "_test.") {
			kind = "test"
		}
		items = append(items, Item{Kind: kind, Path: file, Required: true})
	}
	for i := range items {
		items[i].EstimatedTokens = EstimateText(items[i].Kind + items[i].Path + items[i].TaskID)
	}
	items = append(items, steeringItems(root, slug)...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			if items[i].Path == items[j].Path {
				return items[i].TaskID < items[j].TaskID
			}
			return items[i].Path < items[j].Path
		}
		return items[i].Kind < items[j].Kind
	})
	items, notes := enforceBudget(items, maxTokens)
	manifest := Manifest{Version: ManifestVersion, Mode: mode, Slug: slug, TaskID: taskID, Items: items, Notes: notes}
	for _, item := range items {
		manifest.EstimatedTokens += item.EstimatedTokens
	}
	return manifest, nil
}

// steeringItems references the constitution (.specd/steering/*.md) and memory
// files as manifest items. Steering carries static-instructions mode; memory
// (steering/memory.md and the spec's own memory.md) carries reference-if-needed.
// Token estimates come from on-disk size so the budget reflects what an agent
// would load. Missing files are skipped, never referenced.
func steeringItems(root, slug string) []Item {
	var items []Item
	steeringDir := filepath.Join(root, ".specd", "steering")
	entries, _ := os.ReadDir(steeringDir)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		rel := ".specd/steering/" + entry.Name()
		kind, mode := "steering", "static-instructions"
		if entry.Name() == "memory.md" {
			kind, mode = "memory", "reference-if-needed"
		}
		items = append(items, Item{Kind: kind, Path: rel, Mode: mode, EstimatedTokens: estimateFile(filepath.Join(root, rel))})
	}
	specMem := filepath.Join(".specd", "specs", slug, "memory.md")
	if fi, err := os.Stat(filepath.Join(root, specMem)); err == nil && !fi.IsDir() {
		items = append(items, Item{Kind: "memory", Path: filepath.ToSlash(specMem), Mode: "reference-if-needed", EstimatedTokens: tokensFromBytes(fi.Size())})
	}
	return items
}

func estimateFile(path string) int {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return 0
	}
	return tokensFromBytes(fi.Size())
}

func tokensFromBytes(n int64) int { return int((n + 3) / 4) }

// enforceBudget drops items until the total fits maxTokens, memory before
// steering (constitution wins). Core items (spec/tasks/task/role) are never
// dropped. Deterministic: items arrive sorted, droppable ones removed from the
// end. Returns the surviving items and one note per drop.
func enforceBudget(items []Item, maxTokens int) ([]Item, []string) {
	if maxTokens <= 0 {
		return items, nil
	}
	total := 0
	for _, item := range items {
		total += item.EstimatedTokens
	}
	var notes []string
	for total > maxTokens {
		idx := lastDroppable(items)
		if idx < 0 {
			break
		}
		total -= items[idx].EstimatedTokens
		notes = append(notes, fmt.Sprintf("dropped %s (%s) over context budget", items[idx].Path, items[idx].Kind))
		items = append(items[:idx], items[idx+1:]...)
	}
	return items, notes
}

// lastDroppable returns the index of the last memory item, else the last
// steering item, else -1. Memory always sheds before steering.
func lastDroppable(items []Item) int {
	steer := -1
	for i, item := range items {
		switch item.Kind {
		case "memory":
			memoryIdx := i
			// keep scanning for the last memory item
			for j := i + 1; j < len(items); j++ {
				if items[j].Kind == "memory" {
					memoryIdx = j
				}
			}
			return memoryIdx
		case "steering":
			steer = i
		}
	}
	return steer
}

func ModeForTask(task core.TaskRow) string {
	switch task.Role {
	case "validator":
		return "validator"
	case "scout":
		return "scout"
	case "auditor":
		return "auditor"
	case "craftsman":
		return "craftsman"
	default:
		return "invalid"
	}
}

func EstimateText(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

func ValidateManifest(raw []byte) error {
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("unsupported manifest version %q", manifest.Version)
	}
	if manifest.Mode == "" || manifest.Slug == "" || manifest.TaskID == "" {
		return fmt.Errorf("manifest mode, slug, and task_id are required")
	}
	if len(manifest.Items) < 4 {
		return fmt.Errorf("manifest must contain the four core items")
	}
	for _, item := range manifest.Items {
		if item.Kind == "" {
			return fmt.Errorf("manifest item kind is required")
		}
	}
	return nil
}

func findTask(tasks []core.TaskRow, taskID string) (core.TaskRow, bool) {
	for _, task := range tasks {
		if task.ID == taskID {
			return task, true
		}
	}
	return core.TaskRow{}, false
}

// --- Typed manifest V2 (Domain 02 W1) ----------------------------------------
//
// V2 is the authoritative machine contract: typed lanes with per-item trust,
// load mode, reason, and digests, plus a canonical whole-manifest digest for
// freshness. It is built and validated alongside V1, which stays the default
// renderer until the migration wave switches the default (see ManifestVersion).

const (
	ManifestVersionV2 = "2"
	manifestKindV2    = "context_manifest"
)

// ItemV2 is one typed context reference. Fields mirror design.md ("Item"): the
// harness emits references and compact metadata, never inlined content.
type ItemV2 struct {
	Kind                 string `json:"kind"`
	Source               string `json:"source,omitempty"`
	Selector             string `json:"selector,omitempty"`
	SourceDigest         string `json:"source_digest,omitempty"`
	RepresentationDigest string `json:"representation_digest,omitempty"`
	Required             bool   `json:"required"`
	LoadMode             string `json:"load_mode"`
	Priority             int    `json:"priority"`
	Reason               string `json:"reason"`
	Trust                string `json:"trust"`
	Sensitivity          string `json:"sensitivity"`
	AuthorityLimit       string `json:"authority_limit,omitempty"`
	EstimatedTokens      int    `json:"estimated_tokens"`
	Applicability        string `json:"applicability,omitempty"`
	Route                string `json:"route,omitempty"`
	Capability           string `json:"capability,omitempty"`
	OmissionReason       string `json:"omission_reason,omitempty"`
}

// Omission records one deterministically shed optional item (R3.3): the item
// identity and why it was dropped. Never used for required items.
type Omission struct {
	Kind   string `json:"kind"`
	Source string `json:"source,omitempty"`
	Reason string `json:"reason"`
}

// ManifestV2 is the versioned typed context manifest (design.md "ManifestV2").
type ManifestV2 struct {
	SchemaVersion  string            `json:"schema_version"`
	Kind           string            `json:"kind"`
	Root           string            `json:"root"`
	Slug           string            `json:"slug"`
	Action         string            `json:"action"`
	Phase          string            `json:"phase"`
	TaskID         string            `json:"task_id"`
	SelectedTask   SelectedTaskV2    `json:"selected_task"`
	Authority      *core.AuthorityV1 `json:"authority,omitempty"`
	ConfigDigest   string            `json:"config_digest,omitempty"`
	PaletteDigest  string            `json:"palette_digest,omitempty"`
	Items          []ItemV2          `json:"items"`
	RequiredTokens int               `json:"required_tokens"`
	OptionalTokens int               `json:"optional_tokens"`
	Budget         int               `json:"budget"`
	Omissions      []Omission        `json:"omissions,omitempty"`
	Provenance     string            `json:"provenance,omitempty"`
	ManifestDigest string            `json:"manifest_digest,omitempty"`
}

// SelectedTaskV2 carries exact machine-readable task scope. DeclaredFiles is
// normalized by the byte-stable tasks parser; raw Markdown remains untouched.
type SelectedTaskV2 struct {
	ID            string   `json:"id"`
	Role          string   `json:"role"`
	DeclaredFiles []string `json:"declared_files"`
	Verify        string   `json:"verify"`
	Acceptance    string   `json:"acceptance"`
}

// Known enum values. Unknown values fail closed (R1.3) — the manifest never
// silently reinterprets an unrecognized lane, load mode, or trust class.
var (
	knownKindsV2 = map[string]bool{
		// R2.1 required action lanes:
		"task": true, "requirements": true, "design": true, "role": true,
		"source": true, "test": true,
		// R1.1 typed lanes:
		"instructions": true, "guardrails": true, "knowledge": true,
		"memory": true, "examples": true, "tools": true, "skill": true,
	}
	knownLoadModesV2 = map[string]bool{"eager": true, "lazy": true, "reference": true}
	// Trust precedence chain (design "Authority and trust"), strongest first.
	knownTrustV2 = map[string]bool{
		"harness": true, "guardrail": true, "role": true, "project": true,
		"knowledge": true, "example": true, "memory": true, "external": true,
	}
	knownSensitivityV2 = map[string]bool{"public": true, "internal": true, "secret": true}
)

// ValidateManifestV2 fails closed on an unknown required version, kind, field
// value, or invalid item (R1.3). A required item may never carry an omission
// reason, and no item may promote itself past its trust class into policy.
func ValidateManifestV2(m ManifestV2) error {
	if m.SchemaVersion != ManifestVersionV2 {
		return fmt.Errorf("unsupported manifest schema_version %q", m.SchemaVersion)
	}
	if m.Kind != manifestKindV2 {
		return fmt.Errorf("unexpected manifest kind %q", m.Kind)
	}
	if m.Root == "" || m.Slug == "" || m.Action == "" || m.Phase == "" || m.TaskID == "" {
		return fmt.Errorf("manifest root, slug, action, phase, and task_id are required")
	}
	if len(m.Items) == 0 {
		return fmt.Errorf("manifest must contain at least one item")
	}
	if m.SelectedTask.ID != "" {
		if m.SelectedTask.ID != m.TaskID || m.SelectedTask.Role == "" {
			return fmt.Errorf("selected_task must match task_id and declare role")
		}
		for _, file := range m.SelectedTask.DeclaredFiles {
			clean := filepath.ToSlash(filepath.Clean(file))
			if filepath.IsAbs(file) || clean == ".." || strings.HasPrefix(clean, "../") {
				return fmt.Errorf("selected_task declared file %q escapes repository base", file)
			}
		}
	}
	if m.Authority != nil {
		if err := core.ValidateAuthority(*m.Authority, m.Authority.IssuedAt, m.Phase); err != nil {
			return fmt.Errorf("manifest authority: %w", err)
		}
		if m.Authority.TaskID != m.TaskID || m.Authority.SpecID != m.Slug {
			return fmt.Errorf("manifest authority subject mismatch")
		}
	}
	for i, it := range m.Items {
		if !knownKindsV2[it.Kind] {
			return fmt.Errorf("item %d: unknown kind %q", i, it.Kind)
		}
		if !knownLoadModesV2[it.LoadMode] {
			return fmt.Errorf("item %d (%s): unknown load_mode %q", i, it.Kind, it.LoadMode)
		}
		if !knownTrustV2[it.Trust] {
			return fmt.Errorf("item %d (%s): unknown trust %q", i, it.Kind, it.Trust)
		}
		if it.Sensitivity != "" && !knownSensitivityV2[it.Sensitivity] {
			return fmt.Errorf("item %d (%s): unknown sensitivity %q", i, it.Kind, it.Sensitivity)
		}
		if it.Reason == "" {
			return fmt.Errorf("item %d (%s): reason is required", i, it.Kind)
		}
		if it.Required && it.OmissionReason != "" {
			return fmt.Errorf("item %d (%s): a required item cannot carry an omission reason", i, it.Kind)
		}
	}
	return nil
}

// CanonicalizeV2 sorts items into a deterministic total order so identical
// inputs render byte-identically (R1.4): priority, then kind, source, selector.
func CanonicalizeV2(m *ManifestV2) {
	sort.SliceStable(m.Items, func(i, j int) bool {
		a, b := m.Items[i], m.Items[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Selector < b.Selector
	})
}

// BudgetError signals that the required context alone exceeds the configured
// budget (R3.2). Required context is never truncated to fit; the task must be
// decomposed or its declared files narrowed instead.
type BudgetError struct {
	RequiredTokens int
	Budget         int
}

func (e BudgetError) Error() string {
	return fmt.Sprintf("required context %d tokens exceeds budget %d — decompose the task or narrow declared files", e.RequiredTokens, e.Budget)
}

// EnforceBudgetV2 fits items within budget truthfully (R3): the required total
// is measured and, if it alone exceeds budget, the build fails closed
// (BudgetError) without truncating any required item. Optional items then shed
// in deterministic priority order — least important first — until the total
// fits, each drop recorded as an Omission naming item and reason (R3.3). A
// budget <= 0 disables enforcement. Returns surviving items, omissions, and the
// required/optional token split.
func EnforceBudgetV2(items []ItemV2, budget int) (kept []ItemV2, omissions []Omission, requiredTokens, optionalTokens int, err error) {
	total := 0
	for _, it := range items {
		total += it.EstimatedTokens
		if it.Required {
			requiredTokens += it.EstimatedTokens
		}
	}
	if budget > 0 && requiredTokens > budget {
		return nil, nil, requiredTokens, 0, BudgetError{RequiredTokens: requiredTokens, Budget: budget}
	}
	kept = append(kept, items...)
	for budget > 0 && total > budget {
		idx := leastImportantOptional(kept)
		if idx < 0 {
			break // only required items remain, and those already fit
		}
		dropped := kept[idx]
		total -= dropped.EstimatedTokens
		omissions = append(omissions, Omission{
			Kind:   dropped.Kind,
			Source: dropped.Source,
			Reason: fmt.Sprintf("shed over context budget (priority %d)", dropped.Priority),
		})
		kept = append(kept[:idx], kept[idx+1:]...)
	}
	for _, it := range kept {
		if !it.Required {
			optionalTokens += it.EstimatedTokens
		}
	}
	return kept, omissions, requiredTokens, optionalTokens, nil
}

// leastImportantOptional returns the index of the optional item to shed first:
// the highest priority number (least important), ties broken by (kind, source)
// descending for a deterministic order. Returns -1 when no optional item remains.
func leastImportantOptional(items []ItemV2) int {
	best := -1
	for i, it := range items {
		if it.Required {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		b := items[best]
		switch {
		case it.Priority != b.Priority:
			if it.Priority > b.Priority {
				best = i
			}
		case it.Kind != b.Kind:
			if it.Kind > b.Kind {
				best = i
			}
		case it.Source > b.Source:
			best = i
		}
	}
	return best
}

// ManifestV2Digest is the stable SHA-256 over the canonical manifest, computed
// with the manifest_digest field itself excluded so it never self-references.
// Callers should CanonicalizeV2 first; this also sorts a copy defensively.
func ManifestV2Digest(m ManifestV2) string {
	m.ManifestDigest = ""
	items := make([]ItemV2, len(m.Items))
	copy(items, m.Items)
	m.Items = items
	CanonicalizeV2(&m)
	raw, _ := json.Marshal(m)
	return core.Digest(raw)
}

// DriverItems projects guardrail and palette metadata before mutable action.
func DriverItems(handshake core.Handshake, phase, role string) []ItemV2 {
	items := []ItemV2{{Kind: "guardrails", Source: "inline:driver-policy", SourceDigest: handshake.ConfigDigest, Required: true, LoadMode: "eager", Priority: 0, Reason: "driver authority and drift contract", Trust: "guardrail", Sensitivity: "internal", AuthorityLimit: "role=" + role + "; phase=" + phase + "; human-only tools forbidden", EstimatedTokens: 1}}
	for _, tool := range handshake.ToolContracts {
		items = append(items, ItemV2{Kind: "tools", Source: "inline:tool/" + tool.Name, SourceDigest: handshake.PaletteDigest, Required: true, LoadMode: "eager", Priority: 1, Reason: "canonical command palette route", Trust: "harness", Sensitivity: "internal", AuthorityLimit: fmt.Sprintf("mutable=%t; human_only=%t; exit_semantics=declared", tool.Mutable, tool.HumanOnly), EstimatedTokens: 1, Applicability: phase, Route: tool.Route, Capability: tool.Capability})
	}
	return items
}

// BuildManifestV2 assembles required action knowledge and driver lanes into the
// authoritative machine contract. Plain V1 rendering remains separate.
func BuildManifestV2(root, slug string, tasks []core.TaskRow, taskID, action, phase string, budget int, handshake core.Handshake) (ManifestV2, error) {
	task, ok := findTask(tasks, taskID)
	if !ok {
		return ManifestV2{}, fmt.Errorf("task %s not found", taskID)
	}
	items, err := SelectRequiredLanes(root, slug, task)
	if err != nil {
		return ManifestV2{}, err
	}
	items = append(items, DriverItems(handshake, phase, task.Role)...)
	kept, omissions, required, optional, err := EnforceBudgetV2(items, budget)
	if err != nil {
		return ManifestV2{}, err
	}
	m := ManifestV2{SchemaVersion: ManifestVersionV2, Kind: manifestKindV2, Root: filepath.Clean(root), Slug: slug, Action: action, Phase: phase, TaskID: taskID, SelectedTask: SelectedTaskV2{ID: task.ID, Role: task.Role, DeclaredFiles: append([]string(nil), task.DeclaredFiles...), Verify: task.Verify, Acceptance: task.Acceptance}, ConfigDigest: handshake.ConfigDigest, PaletteDigest: handshake.PaletteDigest, Items: kept, RequiredTokens: required, OptionalTokens: optional, Budget: budget, Omissions: omissions, Provenance: "local deterministic selection"}
	CanonicalizeV2(&m)
	if err := ValidateManifestV2(m); err != nil {
		return ManifestV2{}, err
	}
	m.ManifestDigest = ManifestV2Digest(m)
	return m, nil
}

func AttachAuthority(m ManifestV2, authority core.AuthorityV1) (ManifestV2, error) {
	m.Authority = &authority
	m.ManifestDigest = ""
	if err := ValidateManifestV2(m); err != nil {
		return ManifestV2{}, err
	}
	m.ManifestDigest = ManifestV2Digest(m)
	return m, nil
}
