package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

var validImpacts = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}

func RunMidreq(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	input := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if len(args.Pos) > 1 {
		input = args.Pos[1]
	}
	if slug == "" || input == "" {
		return usageExit("usage: specd midreq <slug> \"<verbatim input>\" --impact <low|medium|high|critical>")
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	impact := args.Str("impact")
	if !validImpacts[impact] {
		return usageExit("--impact must be one of: low, medium, high, critical")
	}
	interpretation := args.Str("interpretation")
	if interpretation == "" {
		interpretation = "TODO"
	}
	changes := args.Str("changes")
	if changes == "" {
		changes = "TODO"
	}
	gated := impact == "high" || impact == "critical"

	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		state, err := core.LoadState(root, slug)
		if err != nil || state == nil {
			return specdExit(err), err
		}
		state.Turn++
		if gated {
			state.Gate = core.GateAwaitingApproval
		}
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}
		stamp := core.Clock().UTC().Format("2006-01-02T15:04")
		entry := fmt.Sprintf("\n## Turn %d — %s — impact: %s\n**User input (verbatim):** \"%s\"\n**Interpretation:** %s\n**Impact:** %s\n**Changes made:** %s\n**Notes / open questions:** TODO\n",
			state.Turn, stamp, impact, input, interpretation, impact, changes)
		path := core.ArtifactPath(root, slug, "mid-requirements.md")
		if err := core.AppendFile(path, entry); err != nil {
			return specdExit(err), err
		}
		fmt.Printf("midreq: logged Turn %d (impact: %s)\n", state.Turn, impact)
		if gated {
			fmt.Println("⛔ gate set to awaiting-approval — stop, present the revised plan, wait for approval.")
		}
		return 0, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}
