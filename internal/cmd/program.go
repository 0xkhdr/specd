package cmd

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunProgram(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	sub := ""
	if len(args.Pos) > 0 {
		sub = args.Pos[0]
	}
	if sub == "link" || sub == "unlink" {
		return programMutate(root, sub, args)
	}
	return programRender(root, args.Bool("json"))
}

func programMutate(root, sub string, args cli.Args) int {
	spec := ""
	if len(args.Pos) > 1 {
		spec = args.Pos[1]
	}
	dep := args.Str("on")
	if spec == "" || dep == "" {
		return usageExit(fmt.Sprintf("usage: specd program %s <spec> --on <dep>", sub))
	}
	if spec == dep {
		return usageExit("a spec cannot depend on itself")
	}
	if !core.SpecExists(root, spec) {
		return specdExit(core.NotFoundError(fmt.Sprintf("spec '%s' not found under .specd/specs/", spec)))
	}
	if !core.SpecExists(root, dep) {
		return specdExit(core.NotFoundError(fmt.Sprintf("spec '%s' not found under .specd/specs/", dep)))
	}

	manifest, err := core.LoadProgram(root)
	if err != nil {
		return specdExit(err)
	}

	current := make(map[string]bool)
	for _, d := range manifest.DependsOn[spec] {
		current[d] = true
	}

	if sub == "link" {
		current[dep] = true
		deps := make([]string, 0, len(current))
		for d := range current {
			deps = append(deps, d)
		}
		manifest.DependsOn[spec] = deps
		g, err := core.BuildProgram(root, &manifest)
		if err != nil {
			return specdExit(err)
		}
		if g.Cycle != nil {
			return specdExit(core.GateError(fmt.Sprintf("linking %s → %s would create a cycle: %s", spec, dep, strings.Join(g.Cycle, " → "))))
		}
		if err := core.SaveProgram(root, manifest); err != nil {
			return specdExit(err)
		}
		fmt.Printf("linked: %s now depends on %s\n", spec, dep)
	} else {
		delete(current, dep)
		deps := make([]string, 0, len(current))
		for d := range current {
			deps = append(deps, d)
		}
		manifest.DependsOn[spec] = deps
		if err := core.SaveProgram(root, manifest); err != nil {
			return specdExit(err)
		}
		fmt.Printf("unlinked: %s no longer depends on %s\n", spec, dep)
	}
	return core.ExitOK
}

func programRender(root string, jsonOut bool) int {
	g, err := core.BuildProgram(root, nil)
	if err != nil {
		return specdExit(err)
	}
	frontier := core.RunnableFrontier(g.Dag)
	frontierSet := make(map[string]bool)
	for _, f := range frontier {
		frontierSet[f.ID] = true
	}
	frontierIDs := make([]string, len(frontier))
	for i, f := range frontier {
		frontierIDs[i] = f.ID
	}
	next := core.NextRunnable(g.Dag)

	if jsonOut {
		type specOut struct {
			core.SpecNode
			Runnable bool `json:"runnable"`
		}
		specs := make([]specOut, len(g.Specs))
		for i, s := range g.Specs {
			specs[i] = specOut{s, frontierSet[s.Slug]}
		}
		waves := core.GroupWaves(g.Dag)
		type waveOut struct {
			Wave  int      `json:"wave"`
			Specs []string `json:"specs"`
		}
		wout := make([]waveOut, len(waves))
		for i, w := range waves {
			ids := make([]string, len(w.Tasks))
			for j, t := range w.Tasks {
				ids[j] = t.ID
			}
			wout[i] = waveOut{w.Wave, ids}
		}
		critical := core.CriticalPath(g.Dag)
		if critical == nil {
			critical = []string{}
		}
		cycle := g.Cycle
		if cycle == nil {
			cycle = []string{}
		}
		orphans := g.Orphans
		if orphans == nil {
			orphans = []struct{ Spec, Dep string }{}
		}
		out := map[string]interface{}{
			"kind": "program", "count": len(g.Specs), "specs": specs,
			"frontier": frontierIDs, "waves": wout,
			"criticalPath": critical,
			"next":         next, "cycle": cycle, "orphans": orphans,
		}
		if err := core.PrintJSON(out); err != nil {
			return specdExit(err)
		}
		if g.Cycle != nil {
			return core.ExitGate
		}
		return core.ExitOK
	}

	if len(g.Specs) == 0 {
		fmt.Println("no specs yet. Run `specd new <slug>`.")
		return core.ExitOK
	}
	fmt.Printf("# Program — %d spec(s)\n", len(g.Specs))
	fmt.Println("legend: ✓ complete · ▶ runnable · ✗ blocked · · waiting")
	fmt.Println()
	for _, w := range core.GroupWaves(g.Dag) {
		fmt.Printf("Wave %d:\n", w.Wave)
		for _, t := range w.Tasks {
			s := findSpecNode(g.Specs, t.ID)
			if s == nil {
				continue
			}
			mark := "·"
			switch {
			case s.Complete:
				mark = "✓"
			case frontierSet[s.Slug]:
				mark = "▶"
			case s.Status == core.StatusBlocked:
				mark = "✗"
			}
			deps := ""
			if len(s.DependsOn) > 0 {
				deps = "  ← " + strings.Join(s.DependsOn, ", ")
			}
			fmt.Printf("  %s %s  [%s]%s\n", mark, s.Slug, s.Status, deps)
		}
	}
	fmt.Println()
	switch {
	case len(frontierIDs) > 0:
		fmt.Printf("Runnable now: %s\n", strings.Join(frontierIDs, ", "))
	case next.Kind == core.NextAllComplete:
		fmt.Println("✓ all specs complete.")
	case next.Kind == core.NextAllBlocked:
		fmt.Printf("⚠ all remaining specs blocked: %s\n", strings.Join(next.Blocked, ", "))
	case next.Kind == core.NextWaiting:
		fmt.Printf("… waiting on incomplete specs: %s\n", strings.Join(next.Blocking, ", "))
	}
	cp := core.CriticalPath(g.Dag)
	if len(cp) > 1 {
		fmt.Printf("Critical path: %s\n", strings.Join(cp, " → "))
	}
	if len(g.Orphans) > 0 {
		fmt.Println()
		for _, o := range g.Orphans {
			fmt.Printf("⚠ %s depends on unknown spec '%s'\n", o.Spec, o.Dep)
		}
	}
	if g.Cycle != nil {
		fmt.Println()
		fmt.Printf("⛔ dependency cycle: %s\n", strings.Join(g.Cycle, " → "))
		return core.ExitGate
	}
	return core.ExitOK
}

func findSpecNode(specs []core.SpecNode, slug string) *core.SpecNode {
	for i := range specs {
		if specs[i].Slug == slug {
			return &specs[i]
		}
	}
	return nil
}
