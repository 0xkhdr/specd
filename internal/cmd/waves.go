package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunWaves implements `specd waves`: it loads the spec's task DAG and prints
// the wave-by-wave task graph, the critical path, and any blockers, as JSON
// (--json) or as the human-readable wave graph.
func RunWaves(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd waves <slug> [--json]")
	if !ok {
		return code
	}
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State

	if args.Bool("json") {
		tasks := core.DagTasksFromState(state)
		type waveJSON struct {
			Wave  int `json:"wave"`
			Tasks []struct {
				ID      string          `json:"id"`
				Status  core.TaskStatus `json:"status"`
				Depends []string        `json:"depends"`
			} `json:"tasks"`
		}
		waves := core.GroupWaves(tasks)
		wout := make([]waveJSON, len(waves))
		for i, w := range waves {
			wout[i].Wave = w.Wave
			for _, t := range w.Tasks {
				depends := t.Depends
				if depends == nil {
					depends = []string{}
				}
				wout[i].Tasks = append(wout[i].Tasks, struct {
					ID      string          `json:"id"`
					Status  core.TaskStatus `json:"status"`
					Depends []string        `json:"depends"`
				}{t.ID, t.Status, depends})
			}
		}
		critical := core.CriticalPath(tasks)
		if critical == nil {
			critical = []string{}
		}
		blockers := state.Blockers
		if blockers == nil {
			blockers = []core.Blocker{}
		}
		out := map[string]interface{}{
			"waves":        wout,
			"criticalPath": critical,
			"blockers":     blockers,
		}
		if err := core.PrintJSON(out); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	fmt.Println(core.WaveGraph(state))
	if len(state.Blockers) > 0 {
		fmt.Println("\nBlockers gating downstream waves:")
		for _, b := range state.Blockers {
			fmt.Printf("  ⚠ %s: %s\n", b.Task, b.Reason)
		}
	}
	return core.ExitOK
}
