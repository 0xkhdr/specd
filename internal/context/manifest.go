package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// ManifestVersion is the on-wire schema version for the human-readable context
// manifest. The typed machine contract is a separate schema (see MachineManifest,
// distinguished by its `kind` field); neither renderer reinterprets the other,
// and ValidateManifest fails closed on any unknown or unsupported version.
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
	Bytes           int    `json:"bytes,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Reason          string `json:"reason,omitempty"`
	Priority        int    `json:"priority,omitempty"`
	Digest          string `json:"digest,omitempty"`
}

type Manifest struct {
	Version         string                      `json:"version"`
	Mode            string                      `json:"mode"`
	Slug            string                      `json:"slug"`
	TaskID          string                      `json:"task_id"`
	Items           []Item                      `json:"items"`
	Omissions       []Omission                  `json:"omissions,omitempty"`
	EstimatedTokens int                         `json:"estimated_tokens"`
	Reason          string                      `json:"reason,omitempty"`
	Priority        int                         `json:"priority,omitempty"`
	Digest          string                      `json:"digest,omitempty"`
	Routing         *core.RoutingRecommendation `json:"routing,omitempty"`
}

// QualityPacket is the compact, reference-only quality contract carried with
// task context. It deliberately stores labels and digests, never eval payloads.
type QualityPacket struct {
	TaskID    string
	Verify    string
	Required  []QualityRequirement
	Revision  string
	Freshness string
	Dataset   string
	Rubric    string
	Output    string
	Trace     string
	Review    core.ReviewContract
}

type QualityRequirement struct {
	Class       string
	Check       string
	Status      string
	ArtifactRef string
	Digest      string
}

// BuildQualityPacket projects local quality records into a deterministic
// context packet. Evidence bodies stay outside context; only refs, digests,
// and freshness labels cross this boundary.
func BuildQualityPacket(contract core.QualityContract, records []core.EvidenceEnvelopeV1, subject core.FreshnessSubject) QualityPacket {
	status := core.EvaluateQuality(contract, records, subject)
	missing := make(map[string]bool, len(status.Missing))
	stale := make(map[string]bool, len(status.Stale))
	for _, req := range status.Missing {
		missing[string(req.EvidenceClass)+"/"+req.CheckID] = true
	}
	for _, req := range status.Stale {
		stale[string(req.EvidenceClass)+"/"+req.CheckID] = true
	}
	p := QualityPacket{TaskID: contract.TaskID, Verify: contract.Verify, Revision: subject.Revision, Freshness: "current", Dataset: subject.DatasetDigest, Rubric: subject.RubricDigest, Output: subject.OutputDigest, Trace: subject.TraceDigest, Review: core.BuildReviewContract(contract, subject.Revision, nil)}
	if len(status.Stale) > 0 {
		p.Freshness = "stale"
	} else if len(status.Missing) > 0 {
		p.Freshness = "incomplete"
	}
	for _, req := range contract.Required {
		key := string(req.EvidenceClass) + "/" + req.CheckID
		label := "passed"
		if stale[key] {
			label = "stale"
		} else if missing[key] {
			label = "missing"
		}
		entry := QualityRequirement{Class: string(req.EvidenceClass), Check: req.CheckID, Status: label}
		for _, record := range records {
			if record.TaskID == contract.TaskID && string(record.EvidenceClass) == entry.Class && record.CheckID == entry.Check && record.Verdict == core.EvalPass {
				entry.ArtifactRef, entry.Digest = record.ArtifactRef, record.ArtifactDigest
				break
			}
		}
		p.Required = append(p.Required, entry)
	}
	return p
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
	cfg, diagnostics := core.LoadConfig(core.ConfigPaths{Project: filepath.Join(root, "project.yml")}, nil)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return Manifest{}, fmt.Errorf("load routing policy: %s", diagnostic.Message)
		}
	}
	routing, err := core.RecommendRouting(task, cfg.Routing)
	if err != nil {
		return Manifest{}, err
	}
	items := []Item{
		{Kind: "spec", Path: fmt.Sprintf(".specd/specs/%s/requirements.md", slug), Required: true, Reason: "approved task requirements"},
		{Kind: "design", Path: fmt.Sprintf(".specd/specs/%s/design.md", slug), Required: true, Reason: "applicable task design"},
		{Kind: "tasks", Path: fmt.Sprintf(".specd/specs/%s/tasks.md", slug), Required: true, Reason: "selected task DAG"},
		{Kind: "task", TaskID: task.ID, Role: task.Role, Verify: task.Verify, Acceptance: task.Acceptance, Required: true, Reason: "exact selected task"},
		{Kind: "role", Path: fmt.Sprintf(".specd/roles/%s.md", task.Role), Required: true, Reason: "task role authority"},
	}
	for _, file := range task.DeclaredFiles {
		kind := "source"
		if strings.HasSuffix(file, "_test.go") || strings.Contains(file, "_test.") {
			kind = "test"
		}
		items = append(items, Item{Kind: kind, Path: file, Required: true, Reason: "declared task file"})
	}
	for i := range items {
		if items[i].Path != "" {
			// R3.1: the estimate covers the bytes the contract actually loads
			// (design.md, declared source/test files), not just the path string.
			items[i].Bytes = int(fileBytes(filepath.Join(root, filepath.FromSlash(items[i].Path))))
		} else {
			items[i].Bytes = len(items[i].Kind + items[i].TaskID)
		}
		items[i].EstimatedTokens = tokensFromBytes(int64(items[i].Bytes))
		items[i].Digest = itemDigest(root, items[i])
	}
	items = append(items, steeringItems(root, slug)...)
	decisionItems, err := loadActiveDecisionItems(root, slug, core.Clock())
	if err != nil {
		return Manifest{}, fmt.Errorf("load decisions: %w", err)
	}
	items = append(items, decisionItems...)
	for i := range items {
		if items[i].Digest == "" {
			items[i].Digest = itemDigest(root, items[i])
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			if items[i].Path == items[j].Path {
				return items[i].TaskID < items[j].TaskID
			}
			return items[i].Path < items[j].Path
		}
		return items[i].Kind < items[j].Kind
	})
	// R3.2: the required set is never silently truncated. If the required core
	// (spec/design/tasks/task/role + declared files) alone exceeds the budget,
	// fail closed with a concise remediation instead of dropping a required item.
	if maxTokens > 0 {
		required := 0
		for _, item := range items {
			if item.Required {
				required += item.EstimatedTokens
			}
		}
		if required > maxTokens {
			return Manifest{}, BudgetError{RequiredTokens: required, Budget: maxTokens}
		}
	}
	items, omissions := enforceBudget(items, maxTokens)
	manifest := Manifest{Version: ManifestVersion, Mode: mode, Slug: slug, TaskID: taskID, Items: items, Omissions: omissions, Routing: &routing}
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
		nbytes := fileBytes(filepath.Join(root, rel))
		items = append(items, Item{Kind: kind, Path: rel, Mode: mode, Bytes: int(nbytes), EstimatedTokens: tokensFromBytes(nbytes), Reason: "available project guidance"})
	}
	specMem := filepath.Join(".specd", "specs", slug, "memory.md")
	if fi, err := os.Stat(filepath.Join(root, specMem)); err == nil && !fi.IsDir() {
		items = append(items, Item{Kind: "memory", Path: filepath.ToSlash(specMem), Mode: "reference-if-needed", Bytes: int(fi.Size()), EstimatedTokens: tokensFromBytes(fi.Size()), Reason: "reference memory"})
	}
	return items
}

func itemDigest(root string, item Item) string {
	if item.Path == "" {
		return core.Digest([]byte(item.Kind + "\x00" + item.TaskID))
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.Path)))
	if err != nil {
		return core.Digest(nil)
	}
	return core.Digest(raw)
}

// fileBytes reports the on-disk size of path, or 0 when it is missing or a
// directory. It is the single source of the payload the context estimate counts.
func fileBytes(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return 0
	}
	return fi.Size()
}

func tokensFromBytes(n int64) int { return int((n + 3) / 4) }

// enforceBudget drops items until the total fits maxTokens, memory before
// steering (constitution wins). Core items (spec/tasks/task/role) are never
// dropped. Deterministic: items arrive sorted, droppable ones removed from the
// end. Returns the surviving items and one note per drop.
func enforceBudget(items []Item, maxTokens int) ([]Item, []Omission) {
	if maxTokens <= 0 {
		return items, nil
	}
	total := 0
	for _, item := range items {
		total += item.EstimatedTokens
	}
	var omissions []Omission
	for total > maxTokens {
		idx := lastDroppable(items)
		if idx < 0 {
			break
		}
		total -= items[idx].EstimatedTokens
		omissions = append(omissions, Omission{
			Kind:   items[idx].Kind,
			Source: items[idx].Path,
			Reason: "shed over context budget (reference-if-needed)",
		})
		items = append(items[:idx], items[idx+1:]...)
	}
	return items, omissions
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

// --- Context accounting (W3 T12, R3.3/R3.4) ----------------------------------
//
// The three context-cost quantities stay distinct and are never merged into one
// authoritative number (cross-wave rule): the harness's local estimate, the
// host's reported actual load, and the provider's billed usage. Host-reported
// and provider-billed default to unknown (nil), never zero — an absent
// measurement must read as "unknown", not "free".

// AccountedItem is one supplied context reference with its estimated cost.
type AccountedItem struct {
	Kind            string `json:"kind"`
	Ref             string `json:"ref,omitempty"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

// HostAck records the host's acknowledgement of the manifest it actually loaded:
// the digest it echoed back and the token count it reported. It is a separate,
// host-attributed quantity that never overwrites the harness estimate.
type HostAck struct {
	ManifestDigest string `json:"manifest_digest"`
	ReportedTokens int    `json:"reported_tokens"`
}

// ContextAccountingV1 is the honest per-task context ledger (R3.3): estimated,
// host-reported, and provider-billed tokens as distinct fields, the canonical
// manifest digest, and the supplied vs. omitted items with reasons.
type ContextAccountingV1 struct {
	Slug                    string          `json:"slug"`
	TaskID                  string          `json:"task_id"`
	EstimatedInputTokens    int             `json:"estimated_input_tokens"`
	HostReportedInputTokens *int            `json:"host_reported_input_tokens,omitempty"`
	ProviderBilledTokens    *int            `json:"provider_billed_tokens,omitempty"`
	ContextManifestDigest   string          `json:"context_manifest_digest"`
	SuppliedItems           []AccountedItem `json:"supplied_items"`
	OmittedItems            []Omission      `json:"omitted_items,omitempty"`
	HostAck                 *HostAck        `json:"host_ack,omitempty"`
}

// ManifestDigest is the canonical SHA-256 over a built manifest. BuildManifest
// already emits items in a deterministic total order, so the digest is stable
// across identical builds (R3.4). The manifest carries no digest field of its
// own (the manifest receipt is W5/W6's surface), so there is no self-reference.
func ManifestDigest(m Manifest) string {
	raw, _ := json.Marshal(m)
	return core.Digest(raw)
}

// BuildAccounting derives the context ledger from a built manifest. Only the
// harness estimate is known at build time; host-reported and provider-billed
// stay unknown until RecordHostAck / an adapter supplies them.
func BuildAccounting(m Manifest) ContextAccountingV1 {
	acc := ContextAccountingV1{
		Slug:                  m.Slug,
		TaskID:                m.TaskID,
		EstimatedInputTokens:  m.EstimatedTokens,
		ContextManifestDigest: ManifestDigest(m),
		OmittedItems:          m.Omissions,
	}
	for _, item := range m.Items {
		ref := item.Path
		if ref == "" {
			ref = item.TaskID
		}
		acc.SuppliedItems = append(acc.SuppliedItems, AccountedItem{Kind: item.Kind, Ref: ref, EstimatedTokens: item.EstimatedTokens})
	}
	return acc
}

// RecordHostAck records the host's acknowledgement as the host-reported
// quantity, kept distinct from the estimate and the provider-billed total.
func (a *ContextAccountingV1) RecordHostAck(ack HostAck) {
	a.HostAck = &ack
	tokens := ack.ReportedTokens
	a.HostReportedInputTokens = &tokens
}

// --- Typed machine manifest ---------------------------------------------------
//
// The machine manifest is the authoritative machine contract: typed lanes with
// per-item trust, load mode, reason, and digests, plus a canonical
// whole-manifest digest for freshness. It is built and validated alongside the
// human-readable manifest, which stays the default renderer (see ManifestVersion).

const (
	MachineManifestVersion         = "1"
	machineManifestKind            = "context_manifest"
	ContentTrustTrustedInstruction = "trusted_instruction"
	ContentTrustUntrustedData      = "untrusted_data"
)

// Context lanes (spec 05 R2.1). A lane states what a path *is* to the task, so
// existence, authority, and budget cost stop being inferred from the same flat
// "required source" list:
//
//   - required_input: a bounded file the task must be able to read. Missing or
//     unreadable fails closed with column, path, and recovery (R2.3).
//   - optional_existing_output: a declared output that already exists; its
//     current content is loaded so the task can modify it.
//   - prospective_output: a declared output that does not exist yet. Write
//     authority is retained, content and digest are omitted, and context does
//     not fail (R2.2) — this is the greenfield lane.
//   - directory_query: an explicitly bounded selector over a directory. A bare
//     directory is an authoring error (R2.4).
//   - managed_policy: harness-owned metadata (selected task, role, steering,
//     skills, config) pinned by digest.
const (
	LaneRequiredInput          = "required_input"
	LaneOptionalExistingOutput = "optional_existing_output"
	LaneProspectiveOutput      = "prospective_output"
	LaneDirectoryQuery         = "directory_query"
	LaneManagedPolicy          = "managed_policy"
)

// Existence states for a lane. Absent is legal only for prospective outputs.
const (
	ExistencePresent    = "present"
	ExistenceAbsent     = "absent"
	ExistenceUnreadable = "unreadable"
)

// MachineItem is one typed context reference. Fields mirror design.md ("Item"): the
// harness emits references and compact metadata, never inlined content.
type MachineItem struct {
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
	ContentTrust         string `json:"content_trust"`
	Sensitivity          string `json:"sensitivity"`
	AuthorityLimit       string `json:"authority_limit,omitempty"`
	EstimatedTokens      int    `json:"estimated_tokens"`
	Applicability        string `json:"applicability,omitempty"`
	Route                string `json:"route,omitempty"`
	Capability           string `json:"capability,omitempty"`
	OmissionReason       string `json:"omission_reason,omitempty"`
	// Lane, Existence, and Loaded are the typed lane projection (R2.1). They are
	// additive to schema_version 1: an older consumer that ignores them still
	// reads a well-formed manifest, and a lane it does not know never widens
	// authority on its own.
	Lane      string `json:"lane,omitempty"`
	Existence string `json:"existence,omitempty"`
	Loaded    bool   `json:"loaded,omitempty"`
}

// Omission records one deterministically shed optional item (R3.3): the item
// identity and why it was dropped. Never used for required items.
type Omission struct {
	Kind   string `json:"kind"`
	Source string `json:"source,omitempty"`
	Reason string `json:"reason"`
}

// MachineManifest is the versioned typed context manifest (design.md "MachineManifest").
type MachineManifest struct {
	SchemaVersion  string              `json:"schema_version"`
	Kind           string              `json:"kind"`
	Root           string              `json:"root"`
	Slug           string              `json:"slug"`
	Action         string              `json:"action"`
	Phase          string              `json:"phase"`
	TaskID         string              `json:"task_id"`
	SelectedTask   MachineSelectedTask `json:"selected_task"`
	Authority      *core.AuthorityV1   `json:"authority,omitempty"`
	ConfigDigest   string              `json:"config_digest,omitempty"`
	PaletteDigest  string              `json:"palette_digest,omitempty"`
	Items          []MachineItem       `json:"items"`
	RequiredTokens int                 `json:"required_tokens"`
	OptionalTokens int                 `json:"optional_tokens"`
	Budget         int                 `json:"budget"`
	Omissions      []Omission          `json:"omissions,omitempty"`
	Provenance     string              `json:"provenance,omitempty"`
	ManifestDigest string              `json:"manifest_digest,omitempty"`
	// Assurance is how much the authority packet is actually worth (R2.6). The
	// manifest builder has no proof of host containment, so it reports the
	// fail-safe ceiling for a host that declares none: advisory. Nothing here
	// can raise it — only a host that proves isolation can, and it must say so.
	Assurance string `json:"assurance,omitempty"`
}

// MachineSelectedTask carries exact machine-readable task scope. DeclaredFiles is
// normalized by the byte-stable tasks parser; raw Markdown remains untouched.
type MachineSelectedTask struct {
	ID            string   `json:"id"`
	Role          string   `json:"role"`
	DeclaredFiles []string `json:"declared_files"`
	Verify        string   `json:"verify"`
	Acceptance    string   `json:"acceptance"`
}

// Known enum values. Unknown values fail closed (R1.3) — the manifest never
// silently reinterprets an unrecognized lane, load mode, or trust class.
var (
	knownMachineKinds = map[string]bool{
		// R2.1 required action lanes:
		"task": true, "requirements": true, "design": true, "role": true,
		"source": true, "test": true,
		// R1.1 typed lanes:
		"instructions": true, "guardrails": true, "knowledge": true,
		"memory": true, "examples": true, "tools": true, "skill": true,
	}
	knownMachineLoadModes = map[string]bool{"eager": true, "lazy": true, "reference": true}
	// Trust precedence chain (design "Authority and trust"), strongest first.
	knownMachineTrust = map[string]bool{
		"harness": true, "guardrail": true, "role": true, "project": true,
		"knowledge": true, "example": true, "memory": true, "external": true,
	}
	knownMachineSensitivity  = map[string]bool{"public": true, "internal": true, "secret": true}
	knownMachineContentTrust = map[string]bool{ContentTrustTrustedInstruction: true, ContentTrustUntrustedData: true}
	knownMachineLanes        = map[string]bool{
		LaneRequiredInput: true, LaneOptionalExistingOutput: true, LaneProspectiveOutput: true,
		LaneDirectoryQuery: true, LaneManagedPolicy: true,
	}
	knownMachineExistence = map[string]bool{ExistencePresent: true, ExistenceAbsent: true, ExistenceUnreadable: true}
)

// ValidateMachineManifest fails closed on an unknown required version, kind, field
// value, or invalid item (R1.3). A required item may never carry an omission
// reason, and no item may promote itself past its trust class into policy.
func ValidateMachineManifest(m MachineManifest) error {
	if m.SchemaVersion != MachineManifestVersion {
		return fmt.Errorf("unsupported manifest schema_version %q", m.SchemaVersion)
	}
	if m.Kind != machineManifestKind {
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
		if !knownMachineKinds[it.Kind] {
			return fmt.Errorf("item %d: unknown kind %q", i, it.Kind)
		}
		if !knownMachineLoadModes[it.LoadMode] {
			return fmt.Errorf("item %d (%s): unknown load_mode %q", i, it.Kind, it.LoadMode)
		}
		if !knownMachineTrust[it.Trust] {
			return fmt.Errorf("item %d (%s): unknown trust %q", i, it.Kind, it.Trust)
		}
		if !knownMachineContentTrust[it.ContentTrust] {
			return fmt.Errorf("item %d (%s): unknown content_trust %q", i, it.Kind, it.ContentTrust)
		}
		if it.Lane != "" && !knownMachineLanes[it.Lane] {
			return fmt.Errorf("item %d (%s): unknown lane %q", i, it.Kind, it.Lane)
		}
		if it.Existence != "" && !knownMachineExistence[it.Existence] {
			return fmt.Errorf("item %d (%s): unknown existence %q", i, it.Kind, it.Existence)
		}
		// A prospective output has no content by definition (R2.2), so it carries
		// no digest and must never be marked required or loaded — that is what
		// keeps a greenfield task from failing context or paying budget for a
		// file it is about to create.
		if it.Lane == LaneProspectiveOutput {
			if it.SourceDigest != "" || it.Loaded || it.Required {
				return fmt.Errorf("item %d (%s): a prospective output cannot be required, loaded, or digested", i, it.Kind)
			}
			if it.Existence != ExistenceAbsent && it.Existence != ExistenceUnreadable {
				return fmt.Errorf("item %d (%s): a prospective output must record an absent or unreadable existence", i, it.Kind)
			}
		} else if it.SourceDigest == "" {
			return fmt.Errorf("item %d (%s): source_digest is required", i, it.Kind)
		}
		if it.Sensitivity != "" && !knownMachineSensitivity[it.Sensitivity] {
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

// CanonicalizeMachineManifest sorts items into a deterministic total order so identical
// inputs render byte-identically (R1.4): priority, then kind, source, selector.
func CanonicalizeMachineManifest(m *MachineManifest) {
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

// EnforceMachineBudget fits items within budget truthfully (R3): the required total
// is measured and, if it alone exceeds budget, the build fails closed
// (BudgetError) without truncating any required item. Optional items then shed
// in deterministic priority order — least important first — until the total
// fits, each drop recorded as an Omission naming item and reason (R3.3). A
// budget <= 0 disables enforcement. Returns surviving items, omissions, and the
// required/optional token split.
func EnforceMachineBudget(items []MachineItem, budget int) (kept []MachineItem, omissions []Omission, requiredTokens, optionalTokens int, err error) {
	total := 0
	for _, it := range items {
		if !CountsAgainstBudget(it) {
			continue
		}
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
		if !it.Required && CountsAgainstBudget(it) {
			optionalTokens += it.EstimatedTokens
		}
	}
	return kept, omissions, requiredTokens, optionalTokens, nil
}

// leastImportantOptional returns the index of the optional item to shed first:
// the highest priority number (least important), ties broken by (kind, source)
// descending for a deterministic order. Returns -1 when no optional item remains.
func leastImportantOptional(items []MachineItem) int {
	best := -1
	for i, it := range items {
		// A prospective output is optional only in the sense that it has no
		// content: it carries the task's write authority for a path that does not
		// exist yet. Shedding it would silently revoke that authority, so it is
		// never a shedding candidate (R2.2/R2.5).
		if it.Required || !CountsAgainstBudget(it) {
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

// MachineManifestDigest is the stable SHA-256 over the canonical manifest, computed
// with the manifest_digest field itself excluded so it never self-references.
// Callers should CanonicalizeMachineManifest first; this also sorts a copy defensively.
func MachineManifestDigest(m MachineManifest) string {
	m.ManifestDigest = ""
	items := make([]MachineItem, len(m.Items))
	copy(items, m.Items)
	m.Items = items
	CanonicalizeMachineManifest(&m)
	raw, _ := json.Marshal(m)
	return core.Digest(raw)
}

// DriverItems projects guardrail and palette metadata before mutable action.
func DriverItems(handshake core.Handshake, phase, role string) []MachineItem {
	items := []MachineItem{{Kind: "guardrails", Source: "inline:driver-policy", SourceDigest: handshake.ConfigDigest, Required: true, LoadMode: "eager", Priority: 0, Reason: "driver authority and drift contract", Trust: "guardrail", ContentTrust: ContentTrustTrustedInstruction, Sensitivity: "internal", AuthorityLimit: "role=" + role + "; phase=" + phase + "; human-only tools forbidden", EstimatedTokens: 1}}
	for _, tool := range handshake.ToolContracts {
		items = append(items, MachineItem{Kind: "tools", Source: "inline:tool/" + tool.Name, SourceDigest: handshake.PaletteDigest, Required: true, LoadMode: "eager", Priority: 1, Reason: "canonical command palette route", Trust: "harness", ContentTrust: ContentTrustTrustedInstruction, Sensitivity: "internal", AuthorityLimit: fmt.Sprintf("mutable=%t; human_only=%t; exit_semantics=declared", tool.Mutable, tool.HumanOnly), EstimatedTokens: 1, Applicability: phase, Route: tool.Route, Capability: tool.Capability})
	}
	return items
}

// BuildMachineManifest assembles required action knowledge and driver lanes into the
// authoritative machine contract. Plain V1 rendering remains separate.
func BuildMachineManifest(root, slug string, tasks []core.TaskRow, taskID, action, phase string, budget int, handshake core.Handshake) (MachineManifest, error) {
	task, ok := findTask(tasks, taskID)
	if !ok {
		return MachineManifest{}, fmt.Errorf("task %s not found", taskID)
	}
	items, err := SelectRequiredLanes(root, slug, task)
	if err != nil {
		return MachineManifest{}, err
	}
	items = append(items, DriverItems(handshake, phase, task.Role)...)
	selection := SelectionContext{Phase: phase, Role: task.Role, TaskID: task.ID, RequirementIDs: splitStaticValues(task.Acceptance), TaskFields: []string{action}, Files: append([]string(nil), task.DeclaredFiles...)}
	steering, steeringOmissions, err := SelectSteering(root, selection)
	if err != nil {
		return MachineManifest{}, err
	}
	memory, memoryOmissions, err := SelectMemory(root, slug, selection)
	if err != nil {
		return MachineManifest{}, err
	}
	examples, exampleOmissions, err := SelectExamples(root, selection)
	if err != nil {
		return MachineManifest{}, err
	}
	skills, skillOmissions, err := SelectSkills(root, SkillSelectionContext{
		SelectionContext: selection,
		Capabilities:     core.SupportedToolCapabilities(handshake.ToolContracts, core.Phase(phase)),
	})
	if err != nil {
		return MachineManifest{}, err
	}
	items = append(items, steering...)
	items = append(items, memory...)
	items = append(items, examples...)
	items = append(items, skills...)
	kept, omissions, required, optional, err := EnforceMachineBudget(items, budget)
	if err != nil {
		return MachineManifest{}, err
	}
	omissions = append(append(append(append(steeringOmissions, memoryOmissions...), exampleOmissions...), skillOmissions...), omissions...)
	m := MachineManifest{SchemaVersion: MachineManifestVersion, Kind: machineManifestKind, Root: filepath.Clean(root), Slug: slug, Action: action, Phase: phase, TaskID: taskID, SelectedTask: MachineSelectedTask{ID: task.ID, Role: task.Role, DeclaredFiles: append([]string(nil), task.DeclaredFiles...), Verify: task.Verify, Acceptance: task.Acceptance}, ConfigDigest: handshake.ConfigDigest, PaletteDigest: handshake.PaletteDigest, Items: kept, RequiredTokens: required, OptionalTokens: optional, Budget: budget, Omissions: omissions, Provenance: "local deterministic selection", Assurance: string(core.AssuranceCeiling(core.HostCapabilities{}))}
	CanonicalizeMachineManifest(&m)
	if err := ValidateMachineManifest(m); err != nil {
		return MachineManifest{}, err
	}
	m.ManifestDigest = MachineManifestDigest(m)
	return m, nil
}

func splitStaticValues(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
	sort.Strings(fields)
	return fields
}

func AttachAuthority(m MachineManifest, authority core.AuthorityV1) (MachineManifest, error) {
	m.Authority = &authority
	m.ManifestDigest = ""
	if err := ValidateMachineManifest(m); err != nil {
		return MachineManifest{}, err
	}
	m.ManifestDigest = MachineManifestDigest(m)
	return m, nil
}

// RequiredDigests returns the digest of every required item in the manifest, in
// deterministic order. It is what a host must acknowledge loading before mutable
// authority activates (R3.1), and what core.BuildContextReceipt derives the
// missing set against.
//
// Digests rather than paths: the receipt contract is content-free, and a digest
// also pins the version the host read. A host that loaded a stale copy of a
// required file reports a digest that no longer matches, which is a miss rather
// than a silent pass.
func RequiredDigests(m MachineManifest) []string {
	digests := make([]string, 0, len(m.Items))
	for _, item := range m.Items {
		if !item.Required {
			continue
		}
		if digest := selectedDigest(item); digest != "" {
			digests = append(digests, digest)
		}
	}
	sort.Strings(digests)
	return slices.Compact(digests)
}
