package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunWaves(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd waves <slug> [--json]")
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
				wout[i].Tasks = append(wout[i].Tasks, struct {
					ID      string          `json:"id"`
					Status  core.TaskStatus `json:"status"`
					Depends []string        `json:"depends"`
				}{t.ID, t.Status, t.Depends})
			}
		}
		out := map[string]interface{}{
			"waves":        wout,
			"criticalPath": core.CriticalPath(tasks),
			"blockers":     state.Blockers,
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	fmt.Println(core.WaveGraph(state))
	if len(state.Blockers) > 0 {
		fmt.Println("\nBlockers gating downstream waves:")
		for _, b := range state.Blockers {
			fmt.Printf("  ⚠ %s: %s\n", b.Task, b.Reason)
		}
	}
	return 0
}
