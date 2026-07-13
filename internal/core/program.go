package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var programDimension = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type SpecEconomics struct {
	SpecID       string           `json:"spec_id"`
	Telemetry    *TelemetryReport `json:"telemetry,omitempty"`
	SourceRefs   []string         `json:"source_refs,omitempty"`
	PreviousCost string           `json:"previous_cost,omitempty"`
}

type EconomicDriftAlert struct {
	SpecID     string   `json:"spec_id"`
	CostDelta  string   `json:"cost_delta"`
	SourceRefs []string `json:"source_refs"`
}

type ProgramEconomics struct {
	Cost         string               `json:"cost"`
	InputTokens  int                  `json:"input_tokens"`
	OutputTokens int                  `json:"output_tokens"`
	CachedTokens int                  `json:"cached_tokens"`
	DurationMs   int                  `json:"duration_ms"`
	Specs        []SpecEconomics      `json:"specs"`
	MissingSpecs []string             `json:"missing_specs,omitempty"`
	Alerts       []EconomicDriftAlert `json:"alerts,omitempty"`
}

type PortfolioSpec struct {
	SpecID      string
	Complete    bool
	Deployments []DeploymentV1
}

type PortfolioEnvironment struct {
	SpecID      string           `json:"spec_id"`
	Environment EnvironmentName  `json:"environment"`
	ReleaseID   string           `json:"release_id"`
	Status      DeploymentStatus `json:"status"`
}

type PortfolioBlocker struct {
	SpecID    string   `json:"spec_id"`
	BlockedBy []string `json:"blocked_by"`
}

type PortfolioView struct {
	Environments []PortfolioEnvironment `json:"environments"`
	Blockers     []PortfolioBlocker     `json:"blockers"`
}

// PortfolioScaleLimit is the bounded, documented projection envelope. Callers
// provide compact records only; BuildPortfolioGovernanceStatus never discovers
// or loads spec prose, ledgers, or context.
const PortfolioScaleLimit = 10000
const PortfolioItemLimit = 1000
const ProgramLinkLimit = 100000
const ProgramLinksPerSpecLimit = 1000

type RiskLevel string

const (
	RiskUnknown  RiskLevel = "unknown"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

func (r RiskLevel) valid() bool {
	switch r {
	case RiskUnknown, RiskLow, RiskMedium, RiskHigh, RiskCritical:
		return true
	}
	return false
}

type ProductionSignalStatus string

const (
	SignalUnknown    ProductionSignalStatus = "unknown"
	SignalUnresolved ProductionSignalStatus = "unresolved"
	SignalResolved   ProductionSignalStatus = "resolved"
)

type SharedOutcomeStatus string

const (
	OutcomeUnknown SharedOutcomeStatus = "unknown"
	OutcomePending SharedOutcomeStatus = "pending"
	OutcomePassed  SharedOutcomeStatus = "passed"
	OutcomeFailed  SharedOutcomeStatus = "failed"
)

type GovernanceItem struct {
	ID        string           `json:"id"`
	Status    GovernanceStatus `json:"status"`
	ReviewAt  string           `json:"review_at,omitempty"`
	ExpiresAt string           `json:"expires_at,omitempty"`
}
type ProductionSignal struct {
	ID     string                 `json:"id"`
	Status ProductionSignalStatus `json:"status"`
}
type SharedOutcome struct {
	ID          string              `json:"id"`
	Status      SharedOutcomeStatus `json:"status"`
	EvidenceRef string              `json:"evidence_ref,omitempty"`
}
type PortfolioGovernanceInput struct {
	SpecID            string
	Complete          bool
	Risk              RiskLevel
	Owner             string
	Governance        []GovernanceItem
	ProductionSignals []ProductionSignal
	SharedOutcomes    []SharedOutcome
}
type PortfolioGovernanceSpec struct {
	SpecID             string             `json:"spec_id"`
	Complete           bool               `json:"complete"`
	Risk               RiskLevel          `json:"risk"`
	Owner              string             `json:"owner"`
	StaleGovernance    []string           `json:"stale_governance,omitempty"`
	ProductionSignals  []ProductionSignal `json:"production_signals,omitempty"`
	SharedOutcomes     []SharedOutcome    `json:"shared_outcomes,omitempty"`
	UnresolvedSignals  []string           `json:"unresolved_signals,omitempty"`
	IncompleteOutcomes []string           `json:"incomplete_outcomes,omitempty"`
}
type PortfolioGovernanceEdge struct {
	From  string    `json:"from"`
	To    string    `json:"to"`
	Kind  LinkKind  `json:"kind"`
	Risk  RiskLevel `json:"risk"`
	Owner string    `json:"owner"`
}
type PortfolioGovernanceStatus struct {
	Specs    []PortfolioGovernanceSpec `json:"specs"`
	Edges    []PortfolioGovernanceEdge `json:"edges"`
	Blockers []PortfolioBlocker        `json:"blockers"`
	Complete bool                      `json:"complete"`
}

// BuildPortfolioGovernanceStatus is a deterministic, read-only projection over
// bounded caller-supplied metadata. Unknown remains explicit and never passes a
// shared outcome. Ordering completion and outcome completion are independent.
func BuildPortfolioGovernanceStatus(program Program, inputs []PortfolioGovernanceInput, asOf time.Time) (PortfolioGovernanceStatus, error) {
	if len(inputs) > PortfolioScaleLimit {
		return PortfolioGovernanceStatus{}, fmt.Errorf("portfolio exceeds scale limit %d", PortfolioScaleLimit)
	}
	rows := append([]PortfolioGovernanceInput(nil), inputs...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].SpecID < rows[j].SpecID })
	complete := make(map[string]bool, len(rows))
	meta := make(map[string]PortfolioGovernanceInput, len(rows))
	out := PortfolioGovernanceStatus{Complete: true}
	for i, row := range rows {
		if !programDimension.MatchString(row.SpecID) || (i > 0 && rows[i-1].SpecID == row.SpecID) {
			return PortfolioGovernanceStatus{}, fmt.Errorf("invalid or duplicate portfolio spec %q", row.SpecID)
		}
		if !row.Risk.valid() {
			return PortfolioGovernanceStatus{}, fmt.Errorf("unknown risk %q for %s", row.Risk, row.SpecID)
		}
		if row.Owner == "" {
			return PortfolioGovernanceStatus{}, fmt.Errorf("empty owner for %s", row.SpecID)
		}
		if len(row.Governance) > PortfolioItemLimit || len(row.ProductionSignals) > PortfolioItemLimit || len(row.SharedOutcomes) > PortfolioItemLimit {
			return PortfolioGovernanceStatus{}, fmt.Errorf("portfolio spec %s exceeds item limit %d", row.SpecID, PortfolioItemLimit)
		}
		complete[row.SpecID], meta[row.SpecID] = row.Complete, row
		projected := PortfolioGovernanceSpec{SpecID: row.SpecID, Complete: row.Complete, Risk: row.Risk, Owner: row.Owner}
		seen := map[string]bool{}
		for _, item := range row.Governance {
			if item.ID == "" || seen[item.ID] || !validGovernanceStatus(item.Status) {
				return PortfolioGovernanceStatus{}, fmt.Errorf("invalid governance item for %s", row.SpecID)
			}
			seen[item.ID] = true
			stale := item.Status == GovernanceExpired
			for _, raw := range []string{item.ReviewAt, item.ExpiresAt} {
				if raw != "" {
					at, err := time.Parse(time.RFC3339, raw)
					if err != nil {
						return PortfolioGovernanceStatus{}, fmt.Errorf("invalid governance date for %s: %w", row.SpecID, err)
					}
					stale = stale || (!asOf.IsZero() && !at.After(asOf))
				}
			}
			if stale {
				projected.StaleGovernance = append(projected.StaleGovernance, item.ID)
			}
		}
		seen = map[string]bool{}
		for _, signal := range row.ProductionSignals {
			if signal.ID == "" || seen[signal.ID] || (signal.Status != SignalUnknown && signal.Status != SignalUnresolved && signal.Status != SignalResolved) {
				return PortfolioGovernanceStatus{}, fmt.Errorf("invalid production signal for %s", row.SpecID)
			}
			seen[signal.ID] = true
			projected.ProductionSignals = append(projected.ProductionSignals, signal)
			if signal.Status != SignalResolved {
				projected.UnresolvedSignals = append(projected.UnresolvedSignals, signal.ID)
			}
		}
		seen = map[string]bool{}
		for _, outcome := range row.SharedOutcomes {
			if outcome.ID == "" || seen[outcome.ID] || (outcome.Status != OutcomeUnknown && outcome.Status != OutcomePending && outcome.Status != OutcomePassed && outcome.Status != OutcomeFailed) || ((outcome.Status == OutcomePassed || outcome.Status == OutcomeFailed) && outcome.EvidenceRef == "") {
				return PortfolioGovernanceStatus{}, fmt.Errorf("invalid shared outcome for %s", row.SpecID)
			}
			seen[outcome.ID] = true
			projected.SharedOutcomes = append(projected.SharedOutcomes, outcome)
			if outcome.Status != OutcomePassed {
				projected.IncompleteOutcomes = append(projected.IncompleteOutcomes, outcome.ID)
			}
		}
		sort.Strings(projected.StaleGovernance)
		sort.Slice(projected.ProductionSignals, func(i, j int) bool { return projected.ProductionSignals[i].ID < projected.ProductionSignals[j].ID })
		sort.Slice(projected.SharedOutcomes, func(i, j int) bool { return projected.SharedOutcomes[i].ID < projected.SharedOutcomes[j].ID })
		sort.Strings(projected.UnresolvedSignals)
		sort.Strings(projected.IncompleteOutcomes)
		if !row.Complete || len(projected.IncompleteOutcomes) > 0 {
			out.Complete = false
		}
		out.Specs = append(out.Specs, projected)
	}
	links := append([]ProgramLink(nil), program.Links...)
	sort.Slice(links, func(i, j int) bool {
		if links[i].From != links[j].From {
			return links[i].From < links[j].From
		}
		if links[i].To != links[j].To {
			return links[i].To < links[j].To
		}
		return links[i].Kind < links[j].Kind
	})
	for _, link := range links {
		from, ok := meta[link.From]
		if !ok {
			return PortfolioGovernanceStatus{}, fmt.Errorf("unknown portfolio link source %q", link.From)
		}
		if _, ok := meta[link.To]; !ok {
			return PortfolioGovernanceStatus{}, fmt.Errorf("unknown portfolio link target %q", link.To)
		}
		out.Edges = append(out.Edges, PortfolioGovernanceEdge{From: link.From, To: link.To, Kind: link.Kind, Risk: from.Risk, Owner: from.Owner})
	}
	for _, row := range rows {
		if blocked := program.IncompleteDeps(row.SpecID, func(id string) bool { return complete[id] }); len(blocked) > 0 {
			out.Blockers = append(out.Blockers, PortfolioBlocker{SpecID: row.SpecID, BlockedBy: blocked})
			out.Complete = false
		}
	}
	return out, nil
}

// BuildPortfolioView projects only supplied local ledgers and program state.
// Ledger order is authoritative; no discovery or network call occurs.
func BuildPortfolioView(program Program, inputs []PortfolioSpec) (PortfolioView, error) {
	if len(inputs) > PortfolioScaleLimit {
		return PortfolioView{}, fmt.Errorf("portfolio exceeds scale limit %d", PortfolioScaleLimit)
	}
	rows := append([]PortfolioSpec(nil), inputs...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].SpecID < rows[j].SpecID })
	complete := map[string]bool{}
	var out PortfolioView
	for i, row := range rows {
		if !programDimension.MatchString(row.SpecID) || (i > 0 && rows[i-1].SpecID == row.SpecID) {
			return PortfolioView{}, fmt.Errorf("invalid or duplicate portfolio spec %q", row.SpecID)
		}
		complete[row.SpecID] = row.Complete
		latest := map[EnvironmentName]DeploymentV1{}
		for _, deployment := range row.Deployments {
			if !deployment.Environment.valid() || !deployment.Status.valid() || deployment.ReleaseID == "" {
				return PortfolioView{}, fmt.Errorf("invalid deployment projection for %s", row.SpecID)
			}
			latest[deployment.Environment] = deployment
		}
		for _, env := range []EnvironmentName{EnvironmentDevelopment, EnvironmentStaging, EnvironmentProduction} {
			if deployment, ok := latest[env]; ok {
				out.Environments = append(out.Environments, PortfolioEnvironment{SpecID: row.SpecID, Environment: env, ReleaseID: deployment.ReleaseID, Status: deployment.Status})
			}
		}
	}
	for _, row := range rows {
		blocked := program.IncompleteDeps(row.SpecID, func(slug string) bool { return complete[slug] })
		if len(blocked) > 0 {
			out.Blockers = append(out.Blockers, PortfolioBlocker{SpecID: row.SpecID, BlockedBy: blocked})
		}
	}
	return out, nil
}

// RollupEconomics is a pure portfolio projection. Dimensions stay bounded to
// spec IDs; missing telemetry remains distinct from a measured zero.
func RollupEconomics(inputs []SpecEconomics, driftThreshold string) (ProgramEconomics, error) {
	threshold := new(big.Rat)
	if driftThreshold != "" {
		if !decimalPattern.MatchString(driftThreshold) {
			return ProgramEconomics{}, fmt.Errorf("invalid drift threshold %q", driftThreshold)
		}
		threshold.SetString(driftThreshold)
	}
	rows := append([]SpecEconomics(nil), inputs...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].SpecID < rows[j].SpecID })
	out := ProgramEconomics{Specs: rows}
	total := new(big.Rat)
	for i, row := range rows {
		if !programDimension.MatchString(row.SpecID) {
			return ProgramEconomics{}, fmt.Errorf("invalid spec dimension %q", row.SpecID)
		}
		if i > 0 && rows[i-1].SpecID == row.SpecID {
			return ProgramEconomics{}, fmt.Errorf("duplicate spec dimension %q", row.SpecID)
		}
		if row.Telemetry == nil {
			out.MissingSpecs = append(out.MissingSpecs, row.SpecID)
			continue
		}
		cost, ok := new(big.Rat).SetString(row.Telemetry.Cost)
		if !ok || !decimalPattern.MatchString(row.Telemetry.Cost) {
			return ProgramEconomics{}, fmt.Errorf("invalid cost for %s", row.SpecID)
		}
		total.Add(total, cost)
		out.InputTokens += row.Telemetry.InputTokens
		out.OutputTokens += row.Telemetry.OutputTokens
		out.CachedTokens += row.Telemetry.CachedTokens
		out.DurationMs += row.Telemetry.DurationMs
		if row.PreviousCost != "" {
			previous, ok := new(big.Rat).SetString(row.PreviousCost)
			if !ok || !decimalPattern.MatchString(row.PreviousCost) {
				return ProgramEconomics{}, fmt.Errorf("invalid previous cost for %s", row.SpecID)
			}
			delta := new(big.Rat).Sub(cost, previous)
			if delta.Sign() < 0 {
				delta.Neg(delta)
			}
			if driftThreshold != "" && delta.Cmp(threshold) > 0 {
				refs := append([]string(nil), row.SourceRefs...)
				sort.Strings(refs)
				out.Alerts = append(out.Alerts, EconomicDriftAlert{SpecID: row.SpecID, CostDelta: formatRat(delta), SourceRefs: refs})
			}
		}
	}
	out.Cost = strings.TrimSpace(formatRat(total))
	return out, nil
}

// ProgramSchemaVersion versions the program.json shape, following the same
// forward-migration discipline as state.json (spec 02). Bump it when the shape
// changes and add a migration in LoadProgram.
const ProgramSchemaVersion = 2

type LinkKind string

const (
	LinkKindFollows    LinkKind = "follows"
	LinkKindRegresses  LinkKind = "regresses"
	LinkKindMaintains  LinkKind = "maintains"
	LinkKindSupersedes LinkKind = "supersedes"
)

func (kind LinkKind) Valid() bool {
	switch kind {
	case LinkKindFollows, LinkKindRegresses, LinkKindMaintains, LinkKindSupersedes:
		return true
	default:
		return false
	}
}

// ProgramLink records that From depends on To — To must reach completion before
// From may execute. Links live at the program level, never inside a spec's
// state.json, so each file keeps a single writer (spec 12 R6).
type ProgramLink struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Kind   LinkKind `json:"kind"`
	Reason string   `json:"reason,omitempty"`
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
	for i := range program.Links {
		if program.Links[i].Kind == "" {
			program.Links[i].Kind = LinkKindFollows
		}
		if !program.Links[i].Kind.Valid() {
			return Program{}, fmt.Errorf("unknown link kind %q", program.Links[i].Kind)
		}
	}
	validated := Program{SchemaVersion: ProgramSchemaVersion}
	for _, link := range program.Links {
		if err := validated.AddTypedLink(link.From, link.To, link.Kind, link.Reason); err != nil {
			return Program{}, fmt.Errorf("invalid program link: %w", err)
		}
	}
	program.SchemaVersion = ProgramSchemaVersion
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
	_ = p.AddTypedLink(from, to, LinkKindFollows, "")
}

// AddTypedLink records a validated, traceable dependency edge. Link kinds are
// metadata: every kind preserves the existing cycle and ordering semantics.
func (p *Program) AddTypedLink(from, to string, kind LinkKind, reason string) error {
	if err := ValidateSlug(from); err != nil {
		return fmt.Errorf("invalid link source: %w", err)
	}
	if err := ValidateSlug(to); err != nil {
		return fmt.Errorf("invalid link target: %w", err)
	}
	if !kind.Valid() {
		return fmt.Errorf("unknown link kind %q", kind)
	}
	for _, link := range p.Links {
		if link.From == from && link.To == to {
			if link.Kind == kind && link.Reason == reason {
				return nil
			}
			return fmt.Errorf("link conflict for %s -> %s: existing kind/reason differs", from, to)
		}
	}
	if len(p.Links) >= ProgramLinkLimit {
		return fmt.Errorf("program exceeds link limit %d", ProgramLinkLimit)
	}
	count := 0
	for _, link := range p.Links {
		if link.From == from {
			count++
		}
	}
	if count >= ProgramLinksPerSpecLimit {
		return fmt.Errorf("spec %s exceeds link limit %d", from, ProgramLinksPerSpecLimit)
	}
	p.Links = append(p.Links, ProgramLink{From: from, To: to, Kind: kind, Reason: reason})
	return nil
}

// AddFeedbackLink links successor-owned maintenance work to completed history.
// It only appends a typed edge: runtime feedback cannot reopen, edit, or remove
// source history. Completion authority stays with the caller's gate predicate.
func (p *Program) AddFeedbackLink(successor, source, reason string, complete func(string) bool) error {
	if err := ValidateSlug(successor); err != nil {
		return fmt.Errorf("invalid feedback successor: %w", err)
	}
	if err := ValidateSlug(source); err != nil {
		return fmt.Errorf("invalid feedback source: %w", err)
	}
	if reason == "" {
		return errors.New("feedback provenance reason is required")
	}
	if complete == nil || !complete(source) {
		return errors.New("feedback source must be completed; create maintenance work only after history is immutable")
	}
	if complete(successor) {
		return errors.New("feedback successor must be new mutable maintenance work")
	}
	if cycle := p.WouldCycle(successor, source); len(cycle) != 0 {
		return fmt.Errorf("feedback successor link would create cycle: %s", strings.Join(cycle, " -> "))
	}
	return p.AddTypedLink(successor, source, LinkKindMaintains, reason)
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
