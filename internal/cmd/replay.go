package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunReplay prints a spec's deterministic, audit-derived event timeline. It is
// strictly read-only: it loads state and renders, never mutating anything. Text
// by default; a typed JSON array under SPECD_JSON.
func RunReplay(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd replay <slug>")
	if !ok {
		return code
	}
	if sessionID := args.Str("acp-session"); sessionID != "" {
		store, err := core.NewACPStore(root)
		if err != nil {
			return specdExit(err)
		}
		events, err := store.ReplaySessionEvents(sessionID)
		if err != nil {
			return specdExit(err)
		}
		timeline := core.ReplaySessionTimeline(events)
		if core.IsJSONMode() {
			if timeline == nil {
				timeline = []core.SessionTimelineEvent{}
			}
			if err := core.PrintJSON(timeline); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		fmt.Printf("acp replay — %s (%d event%s)\n", sessionID, len(timeline), plural(len(timeline)))
		for _, event := range timeline {
			fmt.Printf(" %s\n", core.FormatSessionTimelineEvent(event))
		}
		return core.ExitOK
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	state, err := core.LoadState(root, slug)
	if err != nil {
		return specdExit(err)
	}
	if state == nil {
		return specdExit(core.NotFoundError(fmt.Sprintf("no state for spec '%s'", slug)))
	}

	events := core.ReplayTimeline(state)

	if core.IsJSONMode() {
		if events == nil {
			events = []core.TimelineEvent{}
		}
		if err := core.PrintJSON(events); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	fmt.Printf("replay — %s (%d event%s)\n", slug, len(events), plural(len(events)))
	for _, e := range events {
		at := e.At
		if at == "" {
			at = "(no timestamp)"
		}
		task := ""
		if e.Task != "" {
			task = " " + e.Task
		}
		fmt.Printf("  %s  %-14s%s  %s\n", at, e.Kind, task, e.Detail)
	}
	return core.ExitOK
}
