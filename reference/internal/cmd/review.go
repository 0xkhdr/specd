package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const reviewUsage = "usage: specd review <slug> [checklist] [--force] [--json]"

// RunReview implements `specd review`. The bare form scaffolds review_report.md
// (mandatory sections + verdict skeleton) and prints the read-only reviewer
// brief; `review checklist` deterministically extracts a human review checklist
// from design.md + tasks.md (extraction only, zero interpretation).
func RunReview(args cli.Args) int {
	if len(args.Pos) >= 2 && args.Pos[1] == "checklist" {
		return runReviewChecklist(args)
	}
	root, slug, code, ok := requireRootAndSlug(args, reviewUsage)
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	path := core.ArtifactPath(root, slug, "review_report.md")
	if _, err := os.Stat(path); err == nil && !args.Bool("force") {
		return specdExit(core.GateError(fmt.Sprintf("%s already exists (pass --force to overwrite the scaffold)", path)))
	}
	if err := core.AtomicWrite(path, core.ScaffoldReviewReport(slug)); err != nil {
		return specdExit(err)
	}
	fmt.Printf("wrote review scaffold to %s\n", path)
	fmt.Println(reviewBrief)
	return core.ExitOK
}

// reviewBrief is the read-only, adversarial reviewer role brief printed after the
// scaffold. It never authorizes edits — a reviewer demands the artifact.
const reviewBrief = `
reviewer role — read-only, adversarial:
  - Demand the artifact: cite file:line for every claim; do not trust prose.
  - Bugs: correctness, edge cases, error handling, concurrency.
  - Security: injected input, secrets, unsafe exec, path handling.
  - Hallucinated Dependencies: verify every imported/declared package exists.
  - Verdict must be 'approve' or 'revise'. Approve only when Bugs/Security/
    Hallucinated Dependencies are clear. Human approval stays final.`

func runReviewChecklist(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, reviewUsage)
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	designMd := ""
	if d := core.ReadArtifact(root, slug, "design.md"); d != nil {
		designMd = *d
	}
	var doc *core.ParsedTasks
	if raw := core.ReadArtifact(root, slug, "tasks.md"); raw != nil && strings.TrimSpace(*raw) != "" {
		if parsed, err := core.ParseTasks(*raw); err == nil {
			doc = &parsed
		}
	}
	items := core.ReviewChecklist(designMd, doc)
	if args.Bool("json") {
		if err := core.PrintJSON(map[string]interface{}{"spec": slug, "checklist": items}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	fmt.Printf("=== REVIEW CHECKLIST: %s ===\n", slug)
	for _, it := range items {
		fmt.Println(it)
	}
	return core.ExitOK
}
