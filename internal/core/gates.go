package core

import (
	"fmt"
	"strings"
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
