package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// runReview scaffolds review_report.md for a spec (spec 09 R1): the spec slug,
// the git HEAD under review, a per-task section (id/files/acceptance), and the
// verdict/reviewer/findings fields the auditor fills. It refuses to overwrite an
// existing report — at any HEAD — unless --force (R5.1), so an auditor's notes
// are never clobbered by a re-scaffold. With --restamp (R5.2), it preserves the
// human-authored body byte-for-byte while updating the git HEAD pin.
func runReview(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("review")
	}
	// An undeclared flag is a typo, and silently ignoring it is destructive here:
	// `--restmap` would fall through to scaffold mode and overwrite the very
	// findings the author meant to restamp. Fail closed against the palette.
	// ponytail: scoped to this verb because dispatch has no global flag check;
	// lift it into dispatch when another verb needs the same guard.
	if declared, ok := core.CommandByName("review"); ok {
		known := make(map[string]bool, len(declared.Flags))
		for _, flag := range declared.Flags {
			known[flag.Name] = true
		}
		names := make([]string, 0, len(flags))
		for name := range flags {
			if !known[name] {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		if len(names) > 0 {
			return fmt.Errorf("%w: unknown flag(s) for review: --%s; %s", ErrUsage, strings.Join(names, ", --"), declared.Usage)
		}
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

	// Scaffold mode: any existing report blocks a re-scaffold (R5.1). A stale
	// report is not safe to replace — it holds the auditor's findings just as a
	// current one does, and moving HEAD is not operator authorization to destroy
	// them. --restamp carries that body forward; --force is the explicit
	// destructive action.
	if existing, readErr := os.ReadFile(path); readErr == nil && !flagEnabled(flags, "force") {
		existingHead := core.ReviewReportHead(string(existing))
		if existingHead == "" {
			existingHead = "unresolved"
		}
		return fmt.Errorf("review report already exists (HEAD %s); pass --restamp to update it to HEAD %s preserving findings, or --force to overwrite", existingHead, head)
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
