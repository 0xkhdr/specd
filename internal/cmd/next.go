package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	speccontext "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
)

func runNext(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("next")
	}
	spec, err := loadSpec(root, args[0])
	if err != nil {
		return err
	}
	if flagEnabled(flags, "waves") {
		waves, err := core.ProjectWaves(spec.Tasks)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(waves)
	}
	if err := requireTaskGate(root, args[0]); err != nil {
		// Machine callers (--json / --dispatch) get an empty frontier plus the
		// gate reason rather than a bare error, so a dispatch loop can read the
		// blocker without parsing stderr.
		if flagEnabled(flags, "json") || flagEnabled(flags, "dispatch") {
			return writeJSON(map[string]any{"items": []any{}, "reason": err.Error()})
		}
		return err
	}
	escalated, err := escalatedCounts(root, args[0], spec.Tasks)
	if err != nil {
		return err
	}
	frontier, err := core.FrontierExcluding(spec.Tasks, taskStatus(spec.Tasks), escalatedBoolSet(escalated))
	if err != nil {
		return err
	}
	if flagEnabled(flags, "dispatch") {
		if len(frontier) == 0 {
			return writeJSON(map[string]any{"items": nil})
		}
		config, _ := core.LoadConfig(configPaths(root), getenv())
		manifest, err := speccontext.BuildMachineManifest(root, args[0], spec.Tasks, frontier[0].ID, "dispatch", "execute", contextBudget(root), core.BootstrapHandshake(config))
		if err != nil {
			return err
		}
		return writeJSON(map[string]any{"items": manifest})
	}
	if flagEnabled(flags, "json") {
		return writeJSON(frontier)
	}
	for _, task := range frontier {
		fmt.Fprintln(os.Stdout, task.ID)
	}
	return nil
}
