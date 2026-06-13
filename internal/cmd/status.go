package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunStatus(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	jsonOut := args.Bool("json")
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}

	if slug == "" {
		specs := core.ListSpecs(root)
		if jsonOut {
			type row struct {
				Spec     string          `json:"spec"`
				Status   core.SpecStatus `json:"status"`
				Phase    core.Phase      `json:"phase"`
				Gate     core.Gate       `json:"gate"`
				Pending  int             `json:"pending"`
				Running  int             `json:"running"`
				Complete int             `json:"complete"`
				Blocked  int             `json:"blocked"`
				Total    int             `json:"total"`
			}
			rows := make([]row, 0, len(specs))
			for _, s := range specs {
				st, err := core.LoadState(root, s)
				if err != nil || st == nil {
					continue
				}
				c := core.CountTasks(st)
				rows = append(rows, row{Spec: s, Status: st.Status, Phase: st.Phase, Gate: st.Gate,
					Pending: c.Pending, Running: c.Running, Complete: c.Complete, Blocked: c.Blocked, Total: c.Total})
			}
			b, _ := json.MarshalIndent(rows, "", "  ")
			fmt.Println(string(b))
			return 0
		}
		if len(specs) == 0 {
			fmt.Println("no specs yet. Run `specd new <slug>`.")
			return 0
		}
		for _, s := range specs {
			st, err := core.LoadState(root, s)
			if err != nil || st == nil {
				continue
			}
			c := core.CountTasks(st)
			gate := ""
			if st.Gate != core.GateNone {
				gate = fmt.Sprintf("  ⛔ %s", st.Gate)
			}
			fmt.Printf("%s  [%s]  %d/%d done · next: %s%s\n", s, st.Status, c.Complete, c.Total, core.NextSummary(st), gate)
		}
		return 0
	}

	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State
	c := core.CountTasks(state)

	if jsonOut {
		type fullState struct {
			*core.State
			Counts core.Counts     `json:"counts"`
			Next   core.NextResult `json:"next"`
		}
		out := fullState{State: state, Counts: c, Next: core.NextRunnable(core.DagTasksFromState(state))}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	fmt.Printf("# %s (%s)\n", state.Title, state.Spec)
	fmt.Printf("status: %s · phase: %s · gate: %s · turn: %d\n", state.Status, state.Phase, state.Gate, state.Turn)
	fmt.Printf("tasks: %d complete · %d running · %d pending · %d blocked · %d total\n", c.Complete, c.Running, c.Pending, c.Blocked, c.Total)
	fmt.Println()
	fmt.Println(core.WaveGraph(state))
	if len(state.Blockers) > 0 {
		fmt.Println()
		fmt.Println("Blockers:")
		for _, b := range state.Blockers {
			fmt.Printf("  ⚠ %s: %s (since %s)\n", b.Task, b.Reason, b.Since)
		}
	}
	fmt.Println()
	fmt.Printf("Next: %s\n", core.NextSummary(state))
	if state.Gate != core.GateNone {
		fmt.Printf("\n⛔ GATE: %s — stop and get approval.\n", state.Gate)
	}
	return 0
}
