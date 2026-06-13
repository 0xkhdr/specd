package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunCheck(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd check <slug> [--json]")
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	jsonOut := args.Bool("json")

	var violations, warnings []core.Violation

	// Gate 1: EARS
	reqMd := core.ReadArtifact(root, slug, "requirements.md")
	if reqMd != nil {
		for _, iss := range core.LintEars(*reqMd) {
			violations = append(violations, core.Violation{Gate: "ears", Location: fmt.Sprintf("requirements.md:%d", iss.Line), Message: iss.Message})
		}
	} else {
		violations = append(violations, core.Violation{Gate: "ears", Location: "requirements.md", Message: "requirements.md missing"})
	}

	// Gate 2: design
	violations = append(violations, core.DesignGate(core.ReadArtifact(root, slug, "design.md"))...)

	// Gate 3+4: parse tasks, schema, DAG
	tasksMdRaw := core.ReadArtifact(root, slug, "tasks.md")
	tasksMd := ""
	if tasksMdRaw != nil {
		tasksMd = *tasksMdRaw
	}
	var doc *core.ParsedTasks
	if strings.TrimSpace(tasksMd) != "" {
		parsed, parseErr := core.ParseTasks(tasksMd)
		if parseErr != nil {
			if se, ok := core.IsSpecdError(parseErr); ok {
				violations = append(violations, core.Violation{Gate: "task-schema", Location: "tasks.md", Message: se.Message})
			} else {
				return specdExit(parseErr)
			}
		} else {
			doc = &parsed
		}
	}

	state, err := core.LoadState(root, slug)
	if err != nil {
		return specdExit(err)
	}

	if doc != nil {
		if len(doc.Tasks) == 0 {
			violations = append(violations, core.Violation{Gate: "task-schema", Location: "tasks.md", Message: "no tasks defined"})
		}
		cfg := core.LoadConfig(root)

		// Schema: role valid, verify command
		for _, t := range doc.Tasks {
			role := t.Meta["role"]
			validRole := false
			for _, r := range core.ValidRoles {
				if r == role {
					validRole = true
					break
				}
			}
			if !validRole {
				violations = append(violations, core.Violation{Gate: "task-schema", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: invalid role '%s'", t.ID, role)})
			}
			verify := strings.TrimSpace(t.Meta["verify"])
			isNA := verify == "" || strings.ToUpper(verify[:min3(len(verify), 3)]) == "N/A"
			if isNA {
				isReadonly := false
				for _, r := range core.ReadonlyRoles {
					if r == role {
						isReadonly = true
						break
					}
				}
				if !isReadonly {
					violations = append(violations, core.Violation{Gate: "task-schema", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: verify N/A only allowed for read-only roles (got '%s')", t.ID, role)})
				}
			}
		}

		// DAG
		dag := make([]core.DagTask, len(doc.Tasks))
		for i, t := range doc.Tasks {
			st := core.TaskPending
			if state != nil {
				if ts, ok := state.Tasks[t.ID]; ok {
					st = ts.Status
				}
			}
			dag[i] = core.DagTask{ID: t.ID, Wave: t.Wave, Depends: core.ParseDepends(t.Meta["depends"]), Status: st}
		}
		for _, o := range core.OrphanDeps(dag) {
			violations = append(violations, core.Violation{Gate: "dag", Location: "tasks.md", Message: fmt.Sprintf("%s depends on missing task '%s'", o.Task, o.Dep)})
		}
		if cyc := core.DetectCycle(dag); cyc != nil {
			violations = append(violations, core.Violation{Gate: "dag", Location: "tasks.md", Message: fmt.Sprintf("dependency cycle: %s", strings.Join(cyc, " → "))})
		}
		for _, w := range core.WaveViolations(dag) {
			violations = append(violations, core.Violation{Gate: "dag", Location: "tasks.md", Message: fmt.Sprintf("%s depends on '%s' which is in a later wave", w.Task, w.Dep)})
		}

		// Gate 6: sync
		if state != nil {
			for _, t := range doc.Tasks {
				ts, ok := state.Tasks[t.ID]
				if !ok {
					continue
				}
				checkboxComplete := t.Checked && ts.Status == core.TaskComplete
				checkboxNotComplete := !t.Checked && ts.Status != core.TaskComplete
				if !checkboxComplete && !checkboxNotComplete {
					cbStr := "[ ]"
					if t.Checked {
						cbStr = "[x]"
					}
					violations = append(violations, core.Violation{Gate: "sync", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: checkbox/state drift (checkbox=%s, state=%s)", t.ID, cbStr, ts.Status)})
				}
				annotBlocked := t.Annotation != nil && t.Annotation.Kind == core.AnnotBlocked
				if annotBlocked != (ts.Status == core.TaskBlocked) {
					violations = append(violations, core.Violation{Gate: "sync", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: blocked-annotation/state drift (state=%s)", t.ID, ts.Status)})
				}
			}
		}

		// Gate 7: traceability
		referenced := make(map[int]bool)
		for _, t := range doc.Tasks {
			if _, ok := t.Meta["requirements"]; ok {
				for _, n := range core.ParseRequirements(t.Meta["requirements"]) {
					referenced[n] = true
				}
			}
		}
		if reqMd != nil {
			forwardSink := &warnings
			if cfg.Gates.Traceability == "error" {
				forwardSink = &violations
			}
			reqNums := core.RequirementNumbers(*reqMd)
			for n := range reqNums {
				if !referenced[n] {
					*forwardSink = append(*forwardSink, core.Violation{Gate: "traceability", Location: "requirements.md", Message: fmt.Sprintf("requirement %d not referenced by any task", n)})
				}
			}
			for _, t := range doc.Tasks {
				if _, ok := t.Meta["requirements"]; !ok {
					continue
				}
				for _, n := range core.ParseRequirements(t.Meta["requirements"]) {
					if !reqNums[n] {
						violations = append(violations, core.Violation{Gate: "traceability", Location: fmt.Sprintf("tasks.md:%d", t.Line), Message: fmt.Sprintf("%s: references requirement %d which is not defined in requirements.md", t.ID, n)})
					}
				}
			}
		}
	}

	// Gate 5: evidence
	if state != nil {
		for _, t := range state.Tasks {
			if t.Status != core.TaskComplete {
				continue
			}
			if t.Evidence == nil || strings.TrimSpace(*t.Evidence) == "" {
				violations = append(violations, core.Violation{Gate: "evidence", Location: "state.json", Message: fmt.Sprintf("%s: complete without evidence", t.ID)})
				continue
			}
			isReadonly := false
			for _, r := range core.ReadonlyRoles {
				if r == t.Role {
					isReadonly = true
					break
				}
			}
			if !isReadonly && (t.Verification == nil || !t.Verification.Verified) {
				violations = append(violations, core.Violation{Gate: "evidence", Location: "state.json", Message: fmt.Sprintf("%s: complete without a verified record (role '%s') — run `specd verify %s %s`", t.ID, t.Role, slug, t.ID)})
			}
		}
	}

	if jsonOut {
		out := map[string]interface{}{"ok": len(violations) == 0, "violations": violations, "warnings": warnings}
		if violations == nil {
			out["violations"] = []core.Violation{}
		}
		if warnings == nil {
			out["warnings"] = []core.Violation{}
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		if len(violations) == 0 {
			return 0
		}
		return 1
	}

	for _, w := range warnings {
		fmt.Printf("warn  %s: %s (%s)\n", w.Location, w.Message, w.Gate)
	}
	if len(violations) == 0 {
		warnNote := ""
		if len(warnings) > 0 {
			warnNote = fmt.Sprintf(" (%d warning(s))", len(warnings))
		}
		fmt.Printf("✓ check passed — all gates green for '%s'%s\n", slug, warnNote)
		return 0
	}
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "fail  %s: %s (%s)\n", v.Location, v.Message, v.Gate)
	}
	fmt.Fprintf(os.Stderr, "\n✗ %d violation(s) across gates.\n", len(violations))
	return 1
}

func min3(a, b int) int {
	if a < b {
		return a
	}
	return b
}
