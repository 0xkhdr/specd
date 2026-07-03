package contextpkg

import (
	"github.com/0xkhdr/specd/internal/spec"
	"path/filepath"
	"strings"
)

// ContextMode selects which delivery surface a manifest is built for. The engine
// emits the same item shape for every mode; the mode only widens the inputs a
// surface is expected to supply (e.g. a host budget on a mission).
type ContextMode string

const (
	ContextModeBriefing ContextMode = "briefing" // specd context (single agent / human)
	ContextModeDispatch ContextMode = "dispatch" // specd dispatch (parallel fan-out)
	ContextModeMission  ContextMode = "mission"  // Brain -> Pinky orchestrated mission
)

// ContextRequest is the single input to the shared context engine. It is the
// superset of what the three context-delivery surfaces need so that "what to
// load" is derived exactly once. All IO is injected via ReadArtifact, keeping
// the engine pure and deterministic.
type ContextRequest struct {
	Slug           string
	Status         spec.SpecStatus // empty => phase inferred from TaskID prefix
	TaskID         string
	Role           string
	Files          []string
	Mode           ContextMode
	HostBudget     int      // MCP-negotiated cap; <=0 means "unset"
	ContextCommand string   // the "specd context <slug>" briefing command, if any
	Requirements   []int    // requirement ids the task covers (for targeted slicing)
	DesignHeadings []string // design sections the task names (for targeted slicing)
	// ReadArtifact injects the only IO the engine performs: it returns the raw
	// markdown for a spec artifact ("requirements.md", "design.md", "tasks.md",
	// "memory.md") and ok=false when absent. A nil reader is valid: the engine
	// then falls back to conservative default token hints and whole-file modes,
	// reproducing the pre-measurement output byte-for-byte.
	ReadArtifact func(name string) (content string, ok bool)
}

// Default token hints used when no measurable artifact content is available.
// These mirror the historical hardcoded values so a reader-less request is
// byte-identical to the pre-measurement manifest.
const (
	ctxHintRole       = 800
	ctxHintSkill      = 1200
	ctxHintPhaseSkill = 1600
	ctxHintContext    = 1800
	ctxHintScopeFile  = 1200
	ctxHintArtifact   = 1200
)

const missionContextStrategy = "Load required items in order. Keep optional/reference items collapsed unless needed. Stop expanding optional items before the soft ceiling; never skip required role, skill, context, or scoped files."

// BuildContextManifest is the single source of truth for "what to load" across
// every surface. Order: role -> skill -> phase-skill -> spec-context -> scoped
// files -> phase-filtered source artifacts. Source artifacts are measured (and,
// where a selector matches, delivered as read-targeted slices) via the injected
// reader; everything else uses default hints. The engine performs no IO of its
// own and is total.
func BuildContextManifest(req ContextRequest) MissionContextManifest {
	items := make([]MissionContextItem, 0, 8+len(req.Files))
	add := func(kind, path, command, mode string, required bool, hint int, rationale string, selector *ContextSelector) {
		items = append(items, MissionContextItem{
			Order:     len(items) + 1,
			Kind:      kind,
			Path:      filepath.ToSlash(path),
			Command:   command,
			Mode:      mode,
			Required:  required,
			TokenHint: hint,
			Rationale: rationale,
			Selector:  selector,
		})
	}

	role := req.Role
	if role == "" || !spec.RoleAllowsPhase(role, effectivePhase(req)) {
		role = defaultContextRole(req)
	}
	req.Role = role
	add("role", filepath.Join(".specd", "roles", role+".md"), "", "read-full", true, ctxHintRole, "role authority and constraints", nil)
	if req.Mode == ContextModeMission || role == "pinky" || req.ContextCommand != "" {
		add("skill", filepath.Join(".specd", "skills", "specd-pinky", "SKILL.md"), "", "read-full", true, ctxHintSkill, "Pinky lease/report lifecycle", nil)
	}
	if req.Mode == ContextModeBriefing && req.ContextCommand == "" {
		addSteeringItems(req, add)
	}
	if skill := contextPhaseSkillPath(req.TaskID, req.Files); skill != "" {
		add("phase-skill", skill, "", "read-full", true, ctxHintPhaseSkill, "phase-scoped workflow; do not load unrelated stage guidance", nil)
	}
	if req.Mode == ContextModeBriefing && req.ContextCommand == "" {
		if req.Slug != "" {
			add("handshake-policy", "", "specd handshake policy "+req.Slug+" --json", "run-command", true, 120, "binding config and mode policy; do not inline help schema", nil)
		}
		add("command-schema", "", "specd help --json", "run-command", false, 120, "cache command schema digest when host has no cached schema", nil)
	}
	if req.ContextCommand != "" {
		add("spec-context", "", req.ContextCommand, "run-command", true, ctxHintContext, "phase briefing, load list, blockers, approvals", nil)
	}
	for _, file := range req.Files {
		add("scope-file", file, "", "read-targeted", true, ctxHintScopeFile, "mission-declared file in scope", nil)
	}
	addSourceArtifacts(req, add)

	est := sumRequiredHints(items)
	budget := deriveContextBudget(req)
	return MissionContextManifest{
		Version:          ManifestVersion,
		SoftTokenCeiling: missionContextSoftCeiling,
		Strategy:         missionContextStrategy,
		Role:             role,
		Items:            items,
		EstimatedTokens:  est,
		Budget:           budget,
		OverBudget:       est > budget,
		BudgetActions:    budgetActions(req, est, budget),
	}
}

// addSourceArtifacts appends the phase-relevant source-of-truth artifacts. Each
// is measured against its real content; when the task names a slice selector for
// it (requirement ids, design headings, the task row) and the selector matches,
// the artifact is delivered as a read-targeted slice and its hint measures the
// slice instead of the whole file.
func addSourceArtifacts(req ContextRequest, add func(kind, path, command, mode string, required bool, hint int, rationale string, selector *ContextSelector)) {
	const wholeRationale = "source of truth; expand only if required by contract or context command"
	for _, name := range statusSourceArtifacts(req.Status) {
		path := filepath.Join(".specd", "specs", req.Slug, name)
		mode := "reference-if-needed"
		hint := ctxHintArtifact
		rationale := wholeRationale
		var selector *ContextSelector
		if content, ok := readArtifactContent(req, name); ok {
			if slice, sliced, srat, sel := sliceArtifact(name, content, req); sliced {
				mode = "read-targeted"
				rationale = srat
				hint = EstimateTokensString(slice)
				selector = sel
			} else {
				hint = EstimateTokensString(content)
			}
		}
		add("source-artifact", path, "", mode, false, hint, rationale, selector)
	}
}

// sliceArtifact attempts to reduce an artifact to the minimal block the task
// needs, using the T2 slicers. It returns (slice, true, rationale) only when a
// selector matches; otherwise the caller delivers the whole artifact.
func sliceArtifact(name, content string, req ContextRequest) (slice string, ok bool, rationale string, selector *ContextSelector) {
	switch name {
	case "tasks.md":
		if req.TaskID == "" {
			return "", false, "", nil
		}
		if s, found := TaskSlice(content, req.TaskID); found {
			return s, true, "the task's row only, not the whole task list", &ContextSelector{Artifact: name, TaskID: req.TaskID}
		}
	case "requirements.md":
		if len(req.Requirements) == 0 {
			return "", false, "", nil
		}
		if s, found := CoveredRequirements(content, req.Requirements); found {
			return s, true, "only the requirements this task covers", &ContextSelector{Artifact: name, Requirements: append([]int(nil), req.Requirements...)}
		}
	case "design.md":
		if len(req.DesignHeadings) == 0 {
			return "", false, "", nil
		}
		if s, found := DesignSection(content, req.DesignHeadings); found {
			return s, true, "only the design sections this task names", &ContextSelector{Artifact: name, DesignHeadings: append([]string(nil), req.DesignHeadings...)}
		}
	}
	return "", false, "", nil
}

func addSteeringItems(req ContextRequest, add func(kind, path, command, mode string, required bool, hint int, rationale string, selector *ContextSelector)) {
	steering := []string{"reasoning", "workflow", "product", "tech", "structure"}
	for _, name := range steering {
		mode := "reference-if-needed"
		required := false
		if req.Mode == ContextModeBriefing || name == "reasoning" || name == "workflow" {
			mode = "read-full"
			required = true
		}
		add("steering", filepath.Join(".specd", "steering", name+".md"), "", mode, required, 500, "always-on steering constitution", nil)
	}
}

func readArtifactContent(req ContextRequest, name string) (string, bool) {
	if req.ReadArtifact == nil {
		return "", false
	}
	return req.ReadArtifact(name)
}

// statusSourceArtifacts returns the phase-relevant source artifacts in load
// order. An empty/unknown status keeps the historical full set so callers that
// do not carry a status (e.g. a mission) stay equivalent to the pre-filter path.
func statusSourceArtifacts(status spec.SpecStatus) []string {
	switch status {
	case spec.StatusRequirements:
		return []string{"requirements.md"}
	case spec.StatusDesign:
		return []string{"requirements.md", "design.md"}
	case spec.StatusTasks:
		return []string{"requirements.md", "design.md", "tasks.md"}
	case spec.StatusExecuting, spec.StatusBlocked:
		return []string{"requirements.md", "design.md", "tasks.md"}
	case spec.StatusVerifying:
		return []string{"requirements.md", "tasks.md"}
	case spec.StatusComplete:
		return []string{"tasks.md"}
	default:
		return []string{"requirements.md", "design.md", "tasks.md"}
	}
}

// contextPhaseSkillPath picks the one stage skill to load. Authoring tasks
// ("A…") map to the skill for the artifact they touch; every other task is an
// execution task. Mirrors the historical mission behaviour.
func contextPhaseSkillPath(taskID string, files []string) string {
	if strings.HasPrefix(taskID, "A") {
		for _, file := range files {
			switch filepath.Base(file) {
			case "requirements.md":
				return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-requirements", "SKILL.md"))
			case "design.md":
				return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-design", "SKILL.md"))
			case "tasks.md":
				return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-tasks", "SKILL.md"))
			}
		}
		return ""
	}
	return filepath.ToSlash(filepath.Join(".specd", "skills", "specd-execute", "SKILL.md"))
}

// sumRequiredHints totals the token hints of required items — the floor a host
// must spend before it can do anything, and the figure the budget gate checks.
func sumRequiredHints(items []MissionContextItem) int {
	sum := 0
	for _, item := range items {
		if item.Required {
			sum += item.TokenHint
		}
	}
	return sum
}

// defaultContextRole picks a stable role when caller did not supply one.
func defaultContextRole(req ContextRequest) string {
	switch effectivePhase(req) {
	case spec.PhaseAnalyze, spec.PhasePlan:
		return "architect"
	case spec.PhaseVerify:
		return "validator"
	case spec.PhaseReflect:
		return "documenter"
	default:
		return "craftsman"
	}
}

// deriveContextBudget computes the effective soft ceiling from phase, role, and
// declared file count, then caps it to a host-negotiated budget when present.
// The result is clamped to the manifest's hard bounds. Planning phases default
// higher; low-budget roles default lower; multi-file work scales with file count.
func budgetActions(req ContextRequest, est, budget int) []string {
	if est <= budget {
		return nil
	}
	if req.Mode == ContextModeMission {
		return []string{"run `specd brain compact` for the active session when available", "prefer targeted manifest selectors before broad loading"}
	}
	return []string{"load only read-full and read-targeted items first", "use targeted selectors", "ask the user before broad loading optional references"}
}

func deriveContextBudget(req ContextRequest) int {
	base := missionContextSoftCeiling
	switch effectivePhase(req) {
	case spec.PhaseAnalyze, spec.PhasePlan:
		base = 16000
	case spec.PhaseExecute:
		base = missionContextSoftCeiling
	case spec.PhaseVerify, spec.PhaseReflect:
		base = 9000
	}
	switch spec.RoleBudgetTier(req.Role) {
	case "minimal":
		base = base * 3 / 5
	case "focused":
		// keep
	case "medium":
		base = base * 6 / 5
	case "large":
		base = base * 4 / 3
	}
	base += len(req.Files) * 1500
	if req.HostBudget > 0 && req.HostBudget < base {
		base = req.HostBudget
	}
	return clampContextBudget(base)
}

// effectivePhase resolves the phase from an explicit status, falling back to the
// task-id convention (authoring tasks plan; everything else executes).
func effectivePhase(req ContextRequest) spec.Phase {
	if req.Status != "" {
		return spec.PhaseForStatus(req.Status)
	}
	if strings.HasPrefix(req.TaskID, "A") {
		return spec.PhasePlan
	}
	return spec.PhaseExecute
}

func clampContextBudget(v int) int {
	if v < MinSoftCeiling {
		return MinSoftCeiling
	}
	if v > MaxSoftCeiling {
		return MaxSoftCeiling
	}
	return v
}
