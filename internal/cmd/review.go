package cmd

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

// runReview scaffolds review_report.md for a spec (spec 09 R1): the spec slug,
// the git HEAD under review, a per-task section (id/files/acceptance), and the
// verdict/reviewer/findings fields the auditor fills. It refuses to overwrite a
// report already written for the current HEAD unless --force (R5.1) — an auditor's
// in-progress notes are not clobbered by a re-scaffold. With --restamp (R5.2), it
// preserves human findings while updating the git HEAD pin.
func runReview(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("review")
	}
	slug := args[0]
	spec, err := loadSpec(root, slug)
	if err != nil {
		return err
	}
	head := gitHead(root)
	if !core.HeadPinned(head) {
		fmt.Fprintf(os.Stderr, "warning: git HEAD unresolved (%q); the review cannot pin to a commit and the review gate will treat it as stale\n", head)
	}
	path := core.ReviewReportPath(root, slug)

	// Handle --restamp mode: update existing report to new HEAD while preserving body
	if flagEnabled(flags, "restamp") {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("--restamp requires existing review report: %w", readErr)
		}
		restamped, err := core.RestampReviewReport(string(existing), head)
		if err != nil {
			return fmt.Errorf("failed to restamp review report: %v", err)
		}
		if _, err := core.WithSpecLock(root, func() (struct{}, error) {
			return struct{}{}, core.AtomicWrite(path, restamped)
		}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "restamped %s to HEAD %s\n", path, head)
		return nil
	}

	// Scaffold mode: create new or update stale report (R5.1)
	if existing, readErr := os.ReadFile(path); readErr == nil && !flagEnabled(flags, "force") {
		// Only a report already written for the current HEAD blocks a re-scaffold;
		// a stale report from an older commit is safe to replace. The guard keys on
		// the HEAD line alone so it protects an in-progress report whose verdict is
		// not yet filled (R5.1).
		if core.HeadPinned(head) && core.ReviewReportHead(string(existing)) == head {
			return fmt.Errorf("review report already exists for HEAD %s; pass --force to overwrite or --restamp to preserve findings", head)
		}
	}

	content := core.RenderReviewScaffold(slug, head, spec.Tasks)
	if _, err := core.WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, core.AtomicWrite(path, content)
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "scaffolded %s\n", path)
	return nil
}
