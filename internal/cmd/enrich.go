package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunEnrich drives the AI-enrichment contract that complements `specd boot`.
// boot fills the deterministic skeleton; enrich hands an agent a brief, accepts
// the authored steering sections, and gates their freshness. The binary itself
// performs zero inference — every sub-verb is deterministic.
//
//	specd enrich plan   [--json]                       emit the enrichment brief
//	specd enrich apply  --target <k> [--content-file p] merge agent-authored markdown
//	specd enrich status [--json]                       enrich-freshness gate
func RunEnrich(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}

	verb := "plan"
	if len(args.Pos) > 0 {
		verb = args.Pos[0]
	}
	switch verb {
	case "plan":
		return runEnrichPlan(root, args.Bool("json"))
	case "apply":
		return runEnrichApply(root, args)
	case "status":
		return runEnrichStatus(root, args.Bool("json"))
	default:
		return usageExit("usage: specd enrich [plan|apply|status]  (see `specd help enrich`)")
	}
}

func runEnrichPlan(root string, jsonOut bool) int {
	if _, err := core.CheckBootFreshness(root); err != nil {
		// boot.json absent → can't build a meaningful brief.
		return specdExit(err)
	}
	brief := core.BuildEnrichBrief(root)

	if jsonOut {
		b, _ := json.MarshalIndent(brief, "", "  ")
		fmt.Println(string(b))
		return core.ExitOK
	}

	core.Header("specd enrich — brief")
	fmt.Printf("project: %s\n", brief.ProjectName)
	fmt.Printf("stacks:  %s\n", strings.Join(brief.Stacks, ", "))
	if brief.BootNote != "" {
		core.Warn(brief.BootNote)
	}
	fmt.Println("\nEvidence to read:")
	for _, e := range brief.Evidence {
		note := ""
		if e.Note != "" {
			note = "  — " + e.Note
		}
		fmt.Printf("  • [%s] %s%s\n", e.Kind, e.Path, note)
	}
	fmt.Println("\nTargets:")
	for _, t := range brief.Targets {
		fmt.Printf("  • %s (%s)  [%s]\n", t.File, t.Target, t.State)
		fmt.Printf("      sections: %s\n", strings.Join(t.Sections, ", "))
		fmt.Printf("      %s\n", t.Instructions)
	}
	fmt.Println("\n" + brief.ApplyHint)
	return core.ExitOK
}

func runEnrichApply(root string, args cli.Args) int {
	target := args.Str("target")
	if target == "" {
		return usageExit("usage: specd enrich apply --target <" + strings.Join(core.EnrichTargetKeys(), "|") + "> [--content-file <path>]")
	}

	body, err := readEnrichContent(args.Str("content-file"))
	if err != nil {
		return specdExit(core.UsageError(err.Error()))
	}
	if applyErr := core.ApplyEnrichSection(root, target, body); applyErr != nil {
		return specdExit(applyErr)
	}

	path, _ := core.EnrichTargetPath(root, target)
	fmt.Printf("✓ enriched %s — verify with `specd enrich status`\n", strings.TrimPrefix(path, root+"/"))
	return core.ExitOK
}

// readEnrichContent reads the authored markdown from a file, or from stdin when
// no --content-file is given (the common agent path: pipe the section in).
func readEnrichContent(file string) (string, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read content file: %v", err)
		}
		return string(b), nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %v", err)
	}
	if strings.TrimSpace(string(b)) == "" {
		return "", fmt.Errorf("no content on stdin and no --content-file given")
	}
	return string(b), nil
}

// runEnrichStatus implements `specd enrich status` and `specd check --enrich`.
func runEnrichStatus(root string, jsonOut bool) int {
	res, err := core.CheckEnrichFreshness(root)
	if err != nil {
		return specdExit(err)
	}
	if jsonOut {
		b, _ := json.MarshalIndent(map[string]interface{}{
			"gate": "enrich-freshness", "ok": !res.Stale, "issues": res.Issues,
		}, "", "  ")
		fmt.Println(string(b))
		if res.Stale {
			return core.ExitGate
		}
		return core.ExitOK
	}
	if !res.Stale {
		fmt.Println("✓ enrich-freshness: steering enrichment matches the repository.")
		return core.ExitOK
	}
	for _, iss := range res.Issues {
		fmt.Fprintf(os.Stderr, "fail  enrich.json: %s (enrich-freshness)\n", iss)
	}
	fmt.Fprintf(os.Stderr, "\n✗ enrichment is stale or incomplete — run `specd enrich plan`.\n")
	return core.ExitGate
}
