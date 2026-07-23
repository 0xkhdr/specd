package core

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
)

// CompatSurface is one deprecated surface tracked for eventual, deliberate
// removal. It is pure registry metadata: a stable diagnostic code, the surface
// name, the version it was introduced in, the minimum version and date at which
// removal MAY be proposed, the replacement command, and the accountable owner.
// Removal is never automatic — the registry only measures; a separate release
// decision (T31) deletes. Time alone never deletes support.
type CompatSurface struct {
	Code              string
	Surface           string
	Introduced        string
	MinRemovalVersion string
	MinRemovalDate    string // RFC3339 date (YYYY-MM-DD)
	Replacement       string
	Owner             string
	// Detect returns the identities of every ACTIVE use of this surface found in
	// the supplied, already-loaded metadata. It receives only governed
	// identities and booleans — never source prose or secret values — so it
	// cannot leak them into a diagnostic. Empty result means no active use.
	Detect func(CompatFacts) []string
}

// CompatFacts is the explicit, pre-loaded, redaction-safe metadata the detectors
// read. Every field is a governed identity or boolean; there is deliberately no
// field carrying file contents, prompts, command output, or secret values, so
// detectors structurally cannot enumerate them. Migrated records which codes
// have been migrated away — a migrated surface stops being reported as active
// but its row is retained as migration history.
type CompatFacts struct {
	LegacyConfigSource string          // path/identity of a legacy config source, "" if none
	LegacyStateSchema  bool            // a state file still on the pre-current schema
	LegacyStatusWrites bool            // deprecated status-projection writes still enabled
	LegacyOutputSchema bool            // deprecated machine-output route still requested
	UnknownActors      []string        // actor identities lacking recognised provenance
	LegacyTaskGrammar  bool            // deprecated task-grammar alias still in tasks.md
	Migrated           map[string]bool // codes whose surface has been migrated away
}

// CompatDiagnostic is one row of the compatibility inventory. It carries only
// identities and governance metadata; never the underlying source or secret.
type CompatDiagnostic struct {
	Code            string `json:"code"`
	Surface         string `json:"surface"`
	Entity          string `json:"entity,omitempty"`
	Active          bool   `json:"active"`
	Migrated        bool   `json:"migrated"`
	Replacement     string `json:"replacement"`
	Window          string `json:"window"`
	Owner           string `json:"owner"`
	RemovalEligible bool   `json:"removal_eligible"`
	UnmetGate       string `json:"unmet_gate,omitempty"`
}

// CompatRegistry is the fixed set of tracked deprecated surfaces. It is a pure
// function of the binary, not of any on-disk or network state.
func CompatRegistry() []CompatSurface {
	return []CompatSurface{
		{
			Code: "LEGACY_CONFIG_SOURCE", Surface: "config-source",
			Introduced: "1.0.0", MinRemovalVersion: "1.4.0", MinRemovalDate: "2026-06-01",
			Replacement: "specd config migrate", Owner: "project maintainers",
			Detect: func(f CompatFacts) []string {
				if f.LegacyConfigSource != "" {
					return []string{f.LegacyConfigSource}
				}
				return nil
			},
		},
		{
			Code: "LEGACY_STATE_SCHEMA", Surface: "state-schema",
			Introduced: "1.0.0", MinRemovalVersion: "1.4.0", MinRemovalDate: "2026-06-01",
			Replacement: "specd migrate", Owner: "project maintainers",
			Detect: func(f CompatFacts) []string {
				if f.LegacyStateSchema {
					return []string{"state.json"}
				}
				return nil
			},
		},
		{
			Code: "LEGACY_STATUS_PROJECTION", Surface: "status-projection",
			Introduced: "1.0.0", MinRemovalVersion: "1.4.0", MinRemovalDate: "2026-06-01",
			Replacement: "specd status --json", Owner: "project maintainers",
			Detect: func(f CompatFacts) []string {
				if f.LegacyStatusWrites {
					return []string{"status"}
				}
				return nil
			},
		},
		{
			Code: "LEGACY_OUTPUT_SCHEMA", Surface: "machine-output",
			Introduced: "1.0.0", MinRemovalVersion: "1.4.0", MinRemovalDate: "2026-06-01",
			Replacement: "specd report --json", Owner: "project maintainers",
			Detect: func(f CompatFacts) []string {
				if f.LegacyOutputSchema {
					return []string{"output"}
				}
				return nil
			},
		},
		{
			Code: "UNKNOWN_ACTOR_PROVENANCE", Surface: "actor-provenance",
			Introduced: "1.0.0", MinRemovalVersion: "1.4.0", MinRemovalDate: "2026-06-01",
			Replacement: "specd handshake bootstrap", Owner: "project maintainers",
			Detect: func(f CompatFacts) []string {
				return append([]string(nil), f.UnknownActors...)
			},
		},
		{
			Code: "LEGACY_TASK_GRAMMAR", Surface: "task-grammar",
			Introduced: "1.0.0", MinRemovalVersion: "1.4.0", MinRemovalDate: "2026-06-01",
			Replacement: "specd check", Owner: "project maintainers",
			Detect: func(f CompatFacts) []string {
				if f.LegacyTaskGrammar {
					return []string{"tasks.md"}
				}
				return nil
			},
		},
	}
}

// LoadCompatFacts assembles redaction-safe compatibility facts from on-disk
// state. It only reads (never writes), so it reports correctly even on a
// read-only filesystem, and it extracts only governed identities and schema
// versions — never file contents or secrets. Missing or unreadable state
// yields no active use rather than an error, matching graceful degradation
// elsewhere in the report path.
func LoadCompatFacts(root, slug string) CompatFacts {
	facts := CompatFacts{}
	// Read only the schema header, not the full state, so a state file that fails
	// strict validation still contributes its (safe) schema-version signal.
	if raw, err := os.ReadFile(StatePath(root, slug)); err == nil {
		var header struct {
			SchemaVersion *int `json:"schema_version"`
		}
		if json.Unmarshal(raw, &header) == nil && header.SchemaVersion != nil && *header.SchemaVersion < StateSchemaVersion {
			facts.LegacyStateSchema = true
		}
	}
	return facts
}

// CompatInventory enumerates every tracked surface against the supplied facts,
// deterministically sorted by code then entity. It writes nothing and is a pure
// function of its inputs. currentVersion and today (YYYY-MM-DD) gate removal
// eligibility; a migrated surface is retained in the output but never active.
func CompatInventory(facts CompatFacts, currentVersion, today string) []CompatDiagnostic {
	var out []CompatDiagnostic
	for _, s := range CompatRegistry() {
		migrated := facts.Migrated[s.Code]
		entities := s.Detect(facts)
		if migrated {
			// Migrated surfaces stop reporting active use but stay in history.
			entities = nil
		}
		window := ">=" + s.MinRemovalVersion + " / " + s.MinRemovalDate
		base := CompatDiagnostic{
			Code: s.Code, Surface: s.Surface, Migrated: migrated,
			Replacement: s.Replacement, Window: window, Owner: s.Owner,
		}
		if len(entities) == 0 {
			d := base
			d.Active = false
			d.RemovalEligible, d.UnmetGate = removalEligible(false, s, currentVersion, today)
			out = append(out, d)
			continue
		}
		for _, entity := range entities {
			d := base
			d.Entity = entity
			d.Active = true
			d.RemovalEligible, d.UnmetGate = removalEligible(true, s, currentVersion, today)
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Entity < out[j].Entity
	})
	return out
}

// removalEligible reports whether a surface may be proposed for removal and, if
// not, names the first unmet exit gate. Active use always blocks removal; time
// and version thresholds gate the rest. It never deletes — it only reports.
func removalEligible(active bool, s CompatSurface, currentVersion, today string) (bool, string) {
	if active {
		return false, "active-use"
	}
	if versionLess(currentVersion, s.MinRemovalVersion) {
		return false, "unmet-window-version"
	}
	if today < s.MinRemovalDate {
		return false, "unmet-window-date"
	}
	return true, ""
}

// RemovalInputs is the explicit, pre-loaded release audit a removal proposal is
// judged against (spec R2.1). Every field is a governed fact loaded elsewhere;
// the gate performs no I/O and reaches no network. A field left at its zero
// value fails closed — removal is never granted on missing proof, so a surface
// with no recorded release-owner decision or unproven journeys stays supported.
type RemovalInputs struct {
	CurrentVersion  string          // this binary's version
	Today           string          // YYYY-MM-DD, evaluated in UTC
	ActiveUse       map[string]bool // code -> unsupported active use found in release fixtures
	ReleaseDecision map[string]bool // code -> explicit release-owner removal decision recorded
	JourneysPass    bool            // upgrade, downgrade-preflight, archive, default, and production journeys passed
	DocsSynced      bool            // command-reference, upgrade guide, archival guide, examples, and changelog regenerated
}

// RemovalReadiness is the exit verdict for one tracked surface: whether it may
// be removed and, when not, the first unmet gate plus the code path retained.
type RemovalReadiness struct {
	Code         string `json:"code"`
	Surface      string `json:"surface"`
	Eligible     bool   `json:"eligible"`
	BlockingGate string `json:"blocking_gate,omitempty"`
	RetainedPath string `json:"retained_path,omitempty"`
	Owner        string `json:"owner"`
}

// RemovalPlan judges every tracked surface against the release audit, sorted by
// code. It is the deterministic removal-exit gate: time alone never deletes —
// window (two-minor-release minimum by version AND date), zero unsupported
// active use, an explicit release-owner decision, passing upgrade/downgrade/
// archive/default/production journeys, and synchronized generated docs must all
// pass. The first failed prerequisite blocks removal and names the retained
// path (spec R2.2). It reads nothing and writes nothing.
func RemovalPlan(in RemovalInputs) []RemovalReadiness {
	out := make([]RemovalReadiness, 0, len(CompatRegistry()))
	for _, s := range CompatRegistry() {
		r := RemovalReadiness{Code: s.Code, Surface: s.Surface, Owner: s.Owner}
		if gate := removalExitGate(s, in); gate != "" {
			r.BlockingGate = gate
			r.RetainedPath = s.Surface + " (" + s.Replacement + ")"
		} else {
			r.Eligible = true
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// removalExitGate returns the first unmet removal prerequisite for a surface, or
// "" when every prerequisite passes. The order is fixed so the reported gate is
// deterministic: window before usage before governance before proof before docs.
func removalExitGate(s CompatSurface, in RemovalInputs) string {
	switch {
	case versionLess(in.CurrentVersion, s.MinRemovalVersion):
		return "unmet-window-version"
	case in.Today < s.MinRemovalDate:
		return "unmet-window-date"
	case in.ActiveUse[s.Code]:
		return "active-use"
	case !in.ReleaseDecision[s.Code]:
		return "release-decision"
	case !in.JourneysPass:
		return "journeys"
	case !in.DocsSynced:
		return "docs-sync"
	default:
		return ""
	}
}

// versionLess reports whether dotted-numeric version a precedes b. Non-numeric
// or missing components compare as zero; this is enough for the internal
// release scheme and adds no dependency.
func versionLess(a, b string) bool {
	av, bv := splitVersion(a), splitVersion(b)
	for i := 0; i < len(av) || i < len(bv); i++ {
		var x, y int
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func splitVersion(v string) []int {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}
