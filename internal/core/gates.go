package core

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	contextpkg "github.com/0xkhdr/specd/internal/context"
)

// CheckCtx is the shared, read-only context every check gate operates over. It
// is built once by the caller (RunCheck) and passed to each gate in turn.
type CheckCtx struct {
	Root, Slug string
	ReqMd      *string
	Doc        *ParsedTasks
	State      *State
	Cfg        Config
}

// Gate is a pure check over CheckCtx returning the violations and warnings it
// found. Gates never mutate the context and never perform IO beyond reading the
// artifacts referenced by the context (design.md is read by GateDesign).
type CheckGate func(CheckCtx) (violations, warnings []Violation)

// CheckGates is the ordered gate pipeline run by `specd check <slug>`. Order is
// a user/agent-visible contract (it determines violation listing order) and
// must not change without intent.
var CheckGates = []CheckGate{
	GateEars,
	GateDesign,
	GateTaskSchema,
	GateDAG,
	GateSync,
	GateTraceability,
	GateEvidence,
	GateAcceptance,
	GateScope,
	GateContextBudget,
	GateModeCapability,
}

// RunGates runs the full check pipeline: the ordered pure-gate slice followed by
// any configured external custom gates. It is the single entry point shared by
// `specd check` and `specd report --pr-summary`, so both surfaces apply an
// identical gate set (including custom gates) and can never drift.
func RunGates(c CheckCtx) (violations, warnings []Violation) {
	for _, gate := range CheckGates {
		v, w := gate(c)
		violations = append(violations, v...)
		warnings = append(warnings, w...)
	}
	v, w := runCustomGates(c)
	violations = append(violations, v...)
	warnings = append(warnings, w...)
	return violations, warnings
}

// GateEars lints requirements.md for EARS-form violations.
func GateEars(c CheckCtx) (violations, warnings []Violation) {
	if c.ReqMd != nil {
		for _, iss := range LintEars(*c.ReqMd) {
			violations = append(violations, Violation{Gate: "ears", Location: fmt.Sprintf("requirements.md:%d", iss.Line), Message: iss.Message})
		}
	} else {
		violations = append(violations, Violation{Gate: "ears", Location: "requirements.md", Message: "requirements.md missing"})
	}
	return violations, nil
}

// GateDesign checks design.md for the mandatory sections.
func GateDesign(c CheckCtx) (violations, warnings []Violation) {
	return DesignGate(ReadArtifact(c.Root, c.Slug, "design.md")), nil
}

// GateTaskSchema validates per-task role and verify-command schema.
func GateTaskSchema(c CheckCtx) (violations, warnings []Violation) {
	if c.Doc == nil {
		return nil, nil
	}
	if len(c.Doc.Tasks) == 0 {
		violations = append(violations, Violation{Gate: "task-schema", Location: "tasks.md", Message: "no tasks defined"})
	}
	for _, t := range c.Doc.Tasks {
		role := t.Meta["role"]
		if !IsValidRole(role) {
			violations = append(violations, Violation{Gate: "task-schema", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: invalid role '%s'", t.ID, role)})
		}
		verify := strings.TrimSpace(t.Meta["verify"])
		isNA := verify == "" || strings.ToUpper(verify[:min(len(verify), 3)]) == "N/A"
		if isNA && !IsReadonlyRole(role) {
			violations = append(violations, Violation{Gate: "task-schema", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: verify N/A only allowed for read-only roles (got '%s')", t.ID, role)})
		}
	}
	return violations, nil
}

// GateDAG checks the dependency graph for orphan deps, cycles, and wave-order
// violations.
//
// OrphanDeps/DetectCycle/WaveViolations each rebuild their own id→task map. We
// deliberately do not hoist a shared byID here (Stage 06 F3): benchmarking a
// 200-task DAG put DetectCycle at ~26µs with only 7 allocs, so the extra map
// builds are noise against a once-per-`check` invocation. Sharing the map would
// add *With API surface for no measurable win, so the public funcs stay
// self-contained.
func GateDAG(c CheckCtx) (violations, warnings []Violation) {
	if c.Doc == nil {
		return nil, nil
	}
	dag := make([]DagTask, len(c.Doc.Tasks))
	for i, t := range c.Doc.Tasks {
		st := TaskPending
		if c.State != nil {
			if ts, ok := c.State.Tasks[t.ID]; ok {
				st = ts.Status
			}
		}
		dag[i] = DagTask{ID: t.ID, Wave: t.Wave, Depends: ParseDepends(t.Meta["depends"]), Status: st}
	}
	for _, o := range OrphanDeps(dag) {
		violations = append(violations, Violation{Gate: "dag", Location: "tasks.md", Message: fmt.Sprintf("%s depends on missing task '%s'", o.Task, o.Dep)})
	}
	if cyc := DetectCycle(dag); cyc != nil {
		violations = append(violations, Violation{Gate: "dag", Location: "tasks.md", Message: fmt.Sprintf("dependency cycle: %s", strings.Join(cyc, " → "))})
	}
	for _, w := range WaveViolations(dag) {
		violations = append(violations, Violation{Gate: "dag", Location: "tasks.md", Message: fmt.Sprintf("%s depends on '%s' which is in a later wave", w.Task, w.Dep)})
	}
	return violations, nil
}

// GateSync checks that tasks.md checkboxes/annotations agree with state.json.
func GateSync(c CheckCtx) (violations, warnings []Violation) {
	if c.Doc == nil || c.State == nil {
		return nil, nil
	}
	for _, t := range c.Doc.Tasks {
		ts, ok := c.State.Tasks[t.ID]
		if !ok {
			continue
		}
		checkboxComplete := t.Checked && ts.Status == TaskComplete
		checkboxNotComplete := !t.Checked && ts.Status != TaskComplete
		if !checkboxComplete && !checkboxNotComplete {
			cbStr := "[ ]"
			if t.Checked {
				cbStr = "[x]"
			}
			violations = append(violations, Violation{Gate: "sync", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: checkbox/state drift (checkbox=%s, state=%s)", t.ID, cbStr, ts.Status)})
		}
		annotBlocked := t.Annotation != nil && t.Annotation.Kind == AnnotBlocked
		if annotBlocked != (ts.Status == TaskBlocked) {
			violations = append(violations, Violation{Gate: "sync", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: blocked-annotation/state drift (state=%s)", t.ID, ts.Status)})
		}
	}
	return violations, nil
}

// GateTraceability checks the two-way mapping between requirements and tasks.
// Unreferenced requirements are warnings unless cfg.Gates.Traceability=="error".
func GateTraceability(c CheckCtx) (violations, warnings []Violation) {
	if c.Doc == nil {
		return nil, nil
	}
	referenced := make(map[int]bool)
	for _, t := range c.Doc.Tasks {
		if _, ok := t.Meta["requirements"]; ok {
			for _, n := range ParseRequirements(t.Meta["requirements"]) {
				referenced[n] = true
			}
		}
	}
	if c.ReqMd == nil {
		return nil, nil
	}
	asError := c.Cfg.Gates.Traceability == "error"
	reqNums := RequirementNumbers(*c.ReqMd)
	for n := range reqNums {
		if !referenced[n] {
			v := Violation{Gate: "traceability", Location: "requirements.md", Message: fmt.Sprintf("requirement %d not referenced by any task", n)}
			if asError {
				violations = append(violations, v)
			} else {
				warnings = append(warnings, v)
			}
		}
	}
	for _, t := range c.Doc.Tasks {
		if _, ok := t.Meta["requirements"]; !ok {
			continue
		}
		for _, n := range ParseRequirements(t.Meta["requirements"]) {
			if !reqNums[n] {
				violations = append(violations, Violation{Gate: "traceability", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: references requirement %d which is not defined in requirements.md", t.ID, n)})
			}
		}
	}
	return violations, warnings
}

// GateEvidence checks that every state-complete task carries evidence and (for
// non-read-only roles) a verified record.
func GateEvidence(c CheckCtx) (violations, warnings []Violation) {
	if c.State == nil {
		return nil, nil
	}
	for _, t := range c.State.Tasks {
		if t.Status != TaskComplete {
			continue
		}
		if t.Evidence == nil || strings.TrimSpace(*t.Evidence) == "" {
			violations = append(violations, Violation{Gate: "evidence", Location: "state.json", Message: fmt.Sprintf("%s: complete without evidence", t.ID)})
			continue
		}
		if !IsReadonlyRole(t.Role) && (t.Verification == nil || !t.Verification.Verified) {
			violations = append(violations, Violation{Gate: "evidence", Location: "state.json", Message: fmt.Sprintf("%s: complete without a verified record (role '%s') — run `specd verify %s %s`", t.ID, t.Role, c.Slug, t.ID)})
		}
	}
	return violations, warnings
}

// GateAcceptance enforces, for tasks that declare an `acceptance:` mapping, that
// every mapped criterion is (a) defined in requirements.md and (b) recorded as a
// pass in state.json once the task is complete. It is enforcement-only — specd
// never judges whether a criterion is "met"; it records and gates on the
// operator-supplied pass/fail evidence (`specd verify --criterion`). Severity is
// driven by cfg.Gates.Acceptance: "off"/"" disables the gate (byte-identical to
// pre-gate behaviour), "warn" demotes the completion findings to warnings,
// "error" fails the check. A criterion id that is mapped but undefined in
// requirements.md is always an error (a broken reference, not a severity knob).
func GateAcceptance(c CheckCtx) (violations, warnings []Violation) {
	mode := c.Cfg.Gates.Acceptance
	if mode == "" || mode == "off" {
		return nil, nil
	}
	if c.Doc == nil {
		return nil, nil
	}
	asError := mode == "error"
	var validCrit map[string]bool
	if c.ReqMd != nil {
		validCrit = map[string]bool{}
		for _, cr := range ExtractCriteria(*c.ReqMd) {
			validCrit[cr.ID] = true
		}
	}
	for _, t := range c.Doc.Tasks {
		amap := ParseAcceptanceMap(t.Meta["acceptance"])
		if len(amap) == 0 {
			continue
		}
		isComplete := false
		if c.State != nil {
			if ts, ok := c.State.Tasks[t.ID]; ok && ts.Status == TaskComplete {
				isComplete = true
			}
		}
		ids := sortedKeys(amap)
		for _, critID := range ids {
			if validCrit != nil && !validCrit[critID] {
				violations = append(violations, Violation{Gate: "acceptance", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s maps acceptance to criterion %s which is not defined in requirements.md", t.ID, critID)})
				continue
			}
			if !isComplete {
				continue
			}
			rec, ok := c.State.Acceptance[critID]
			if !ok || rec.Status != "pass" {
				v := Violation{Gate: "acceptance", Location: "state.json", Message: fmt.Sprintf("%s complete but acceptance criterion %s has no recorded pass — run `specd verify %s --criterion %s --status pass --evidence \"...\"`", t.ID, critID, c.Slug, critID)}
				if asError {
					violations = append(violations, v)
				} else {
					warnings = append(warnings, v)
				}
			}
		}
	}
	return violations, warnings
}

// GateScope flags verify-time changed files that fall outside a task's declared
// `files:` contract. cfg.Gates.Scope drives it: "off"/""/"*" disables it (the
// default — no behavioural change), "warn"/"error" set the severity. A task whose
// `files` contract is "*" (or empty) opts out individually. Evidence comes from
// the VerificationRecord's ChangedFiles; tasks without a verify record are
// skipped (nothing to scope). Matching is glob-based (path.Match per declared
// pattern) so contracts like `internal/core/*.go` work.
func GateScope(c CheckCtx) (violations, warnings []Violation) {
	mode := c.Cfg.Gates.Scope
	if mode == "" || mode == "off" || mode == "*" {
		return nil, nil
	}
	if c.Doc == nil || c.State == nil {
		return nil, nil
	}
	asError := mode == "error"
	for _, t := range c.Doc.Tasks {
		ts, ok := c.State.Tasks[t.ID]
		if !ok || ts.Verification == nil || len(ts.Verification.ChangedFiles) == 0 {
			continue
		}
		patterns := parseFilesContract(t.Meta["files"])
		if len(patterns) == 0 || containsStr(patterns, "*") {
			continue
		}
		for _, f := range ts.Verification.ChangedFiles {
			if !matchesAnyGlob(f, patterns) {
				v := Violation{Gate: "scope", Location: "state.json", Message: fmt.Sprintf("%s changed '%s' outside its declared files contract (%s)", t.ID, f, strings.Join(patterns, ", "))}
				if asError {
					violations = append(violations, v)
				} else {
					warnings = append(warnings, v)
				}
			}
		}
	}
	return violations, warnings
}

// GateContextBudget is the opt-in context-budget gate. It is a no-op unless
// cfg.Gates.ContextBudget names a severity ("warn"/"error"; "off"/""/"*" disable
// it — the default, so the core pipeline is unchanged). When enabled it builds
// the active spec's context manifest through the shared engine and reports when
// the required-item token estimate exceeds the derived budget, naming the
// heaviest required items so the offender is actionable.
func GateContextBudget(c CheckCtx) (violations, warnings []Violation) {
	mode := c.Cfg.Gates.ContextBudget
	if mode == "" || mode == "off" || mode == "*" {
		return nil, nil
	}
	if c.State == nil {
		return nil, nil
	}
	m := buildCheckContextManifest(c)
	if m.Budget <= 0 || m.EstimatedTokens <= m.Budget {
		return nil, nil
	}
	v := Violation{
		Gate:     "context-budget",
		Location: "context-manifest",
		Message: fmt.Sprintf("required context ~%d tokens exceeds budget %d; heaviest: %s",
			m.EstimatedTokens, m.Budget, strings.Join(heaviestRequiredItems(m, 3), ", ")),
	}
	if mode == "error" {
		return []Violation{v}, nil
	}
	return nil, []Violation{v}
}

// buildCheckContextManifest assembles the context manifest for the spec under
// check, scoped to the next runnable task when one exists. The injected reader
// is the only IO, mirroring the other artifact-reading gates.
// GateModeCapability is the opt-in mode-capability gate. It is a no-op unless
// cfg.Gates.ModeCapability names a severity ("warn"/"error"; "off"/""/"*"
// disable it — the default, so Base projects stay clean). When enabled it flags
// a spec recorded as orchestrated while the project lacks orchestration
// capability (orchestration.enabled absent/false), pointing at the one enabling
// command. This catches a spec that opted into orchestration in a project that
// was never (or no longer is) orchestration-capable.
func GateModeCapability(c CheckCtx) (violations, warnings []Violation) {
	mode := c.Cfg.Gates.ModeCapability
	if mode == "" || mode == "off" || mode == "*" {
		return nil, nil
	}
	if c.State == nil || c.State.EffectiveMode() != ModeOrchestrated {
		return nil, nil
	}
	if c.Cfg.Orchestration.Enabled {
		return nil, nil
	}
	v := Violation{
		Gate:     "mode-capability",
		Location: "state.json",
		Message:  "spec is orchestrated but project has no orchestration capability — enable it with `specd init --orchestration session` (or manual|planning), or switch the spec back with `specd mode <slug> --set base`",
	}
	if mode == "error" {
		return []Violation{v}, nil
	}
	return nil, []Violation{v}
}

func buildCheckContextManifest(c CheckCtx) contextpkg.MissionContextManifest {
	req := contextpkg.ContextRequest{
		Slug:         c.Slug,
		Status:       c.State.Status,
		Mode:         contextpkg.ContextModeMission,
		HostBudget:   c.Cfg.Gates.MaxContextTokens,
		ReadArtifact: specArtifactReader(c.Root, c.Slug),
	}
	if next := NextRunnable(DagTasksFromState(c.State)); next.Kind == NextTask && c.Doc != nil {
		v := ResolveTaskView(*c.Doc, c.State, next.ID)
		req.TaskID = next.ID
		req.Role = v.Role
		req.Files = SplitCSV(v.Meta["files"])
		req.Requirements = v.Requirements
	}
	return contextpkg.BuildContextManifest(req)
}

// heaviestRequiredItems returns up to n required items, heaviest token hint
// first, formatted "kind path (~tokens)" for actionable gate output. Ties break
// on the item's manifest order so the result is deterministic.
func heaviestRequiredItems(m contextpkg.MissionContextManifest, n int) []string {
	required := make([]contextpkg.MissionContextItem, 0, len(m.Items))
	for _, it := range m.Items {
		if it.Required {
			required = append(required, it)
		}
	}
	sort.SliceStable(required, func(i, j int) bool {
		if required[i].TokenHint != required[j].TokenHint {
			return required[i].TokenHint > required[j].TokenHint
		}
		return required[i].Order < required[j].Order
	})
	if len(required) > n {
		required = required[:n]
	}
	out := make([]string, 0, len(required))
	for _, it := range required {
		ref := it.Path
		if ref == "" {
			ref = it.Command
		}
		out = append(out, fmt.Sprintf("%s %s (~%d)", it.Kind, ref, it.TokenHint))
	}
	return out
}

// runCustomGates executes each configured external custom gate and folds its
// findings into the check result. A gate that errors (non-zero exit, timeout,
// invalid JSON) is itself reported as a violation so a broken gate fails loudly
// rather than silently passing. Per-gate Severity decides whether its reported
// findings are violations or warnings (default "error").
func runCustomGates(c CheckCtx) (violations, warnings []Violation) {
	gates := c.Cfg.Gates.Custom
	if len(gates) == 0 || c.State == nil {
		return nil, nil
	}
	shell := strings.TrimSpace(customGateShell())
	if shell == "" {
		shell = "sh"
	}
	input := BuildCustomGateInput(c.Root, c.State)
	for _, g := range gates {
		name := g.Name
		if name == "" {
			name = "custom"
		}
		gateID := "custom:" + name
		if strings.TrimSpace(g.Command) == "" {
			violations = append(violations, Violation{Gate: gateID, Location: ".specd/config.json", Message: fmt.Sprintf("custom gate %q has no command", name)})
			continue
		}
		out, err := RunCustomGate(context.Background(), c.Root, shell, g.Command, input, customGateTimeout(), g.Sandbox)
		if err != nil {
			violations = append(violations, Violation{Gate: gateID, Location: ".specd/config.json", Message: err.Error()})
			continue
		}
		asError := g.Severity != "warn"
		for _, f := range out.Violations {
			v := Violation{Gate: gateID, Location: f.Location, Message: f.Message}
			if asError {
				violations = append(violations, v)
			} else {
				warnings = append(warnings, v)
			}
		}
		for _, f := range out.Warnings {
			warnings = append(warnings, Violation{Gate: gateID, Location: f.Location, Message: f.Message})
		}
	}
	return violations, warnings
}

// customGateTimeout is the per-gate wall-clock budget, overridable via
// SPECD_CUSTOM_GATE_TIMEOUT_MS (default 30s).
func customGateTimeout() time.Duration {
	return time.Duration(EnvInt("SPECD_CUSTOM_GATE_TIMEOUT_MS", 30_000, 1, 0)) * time.Millisecond
}

// customGateShell mirrors the verify shell selection so custom gates run under
// the same interpreter as verify commands.
func customGateShell() string {
	if s := strings.TrimSpace(os.Getenv("SPECD_VERIFY_SHELL")); s != "" {
		return s
	}
	return "sh"
}

// parseFilesContract splits a `files:` metadata value into individual path
// globs, tolerating comma- or whitespace-separated lists.
func parseFilesContract(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == ';'
	})
	var out []string
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// matchesAnyGlob reports whether path p matches any of the declared globs. A
// pattern that ends in "/" or is a bare directory also matches files beneath it.
func matchesAnyGlob(p string, patterns []string) bool {
	for _, pat := range patterns {
		if pat == p {
			return true
		}
		if ok, _ := path.Match(pat, p); ok {
			return true
		}
		// Directory-prefix contract: "internal/core" matches "internal/core/x.go".
		dir := strings.TrimSuffix(pat, "/")
		if dir != "" && (strings.HasPrefix(p, dir+"/")) {
			return true
		}
		// Recursive glob convenience: "dir/**" matches anything under dir.
		if strings.HasSuffix(pat, "/**") {
			base := strings.TrimSuffix(pat, "/**")
			if base == "" || strings.HasPrefix(p, base+"/") {
				return true
			}
		}
	}
	return false
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
