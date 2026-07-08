package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
)

// runSubmit is the terminal verb (spec 08). It refuses unless every gate is
// green and every task is complete (R1), then generates the deterministic PR
// summary — the same generator as `report --pr`, one implementation (R2) — and
// streams it on stdin to the operator-configured submit.command through the
// existing sandboxed exec path. With no command configured it prints the summary
// and exits 0 (dry-run, R3). A successful run appends a submission record to the
// spec ledger (R4); a same-HEAD resubmission is refused without --resubmit (R5).
func runSubmit(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd submit <spec> [--resubmit]")
	}
	slug := args[0]

	spec, err := loadSpec(root, slug)
	if err != nil {
		return err
	}
	gateFailures := gateFailureMessages(gates.CoreRegistry().Run(buildCheckCtx(root, slug, spec, "")))
	model, err := reportModel(root, slug)
	if err != nil {
		return err
	}
	if blockers := core.SubmitBlockers(model, gateFailures); len(blockers) > 0 {
		fmt.Fprintf(os.Stderr, "submit refused: %d blocker(s)\n", len(blockers))
		for _, b := range blockers {
			fmt.Fprintf(os.Stderr, "  - %s\n", b)
		}
		return errors.New("submit blocked by unmet gates or incomplete tasks")
	}

	summary := core.PRSummary(model)
	hash := core.SummaryHash(summary)
	head := gitHead(root)

	submissions, err := core.LoadSubmissions(core.SubmissionsPath(root, slug))
	if err != nil {
		return err
	}
	if core.AlreadySubmittedAt(submissions, head) && !flagEnabled(flags, "resubmit") {
		return fmt.Errorf("already submitted at HEAD %s; pass --resubmit to submit again", head)
	}

	cfg := loadSpecConfig(root)
	if cfg.Submit.Command == "" {
		// Dry-run default: no operator command configured (R3). Print the summary
		// to stdout and exit 0; nothing is recorded because nothing was submitted.
		fmt.Fprint(os.Stdout, summary)
		return nil
	}

	if !core.HeadPinned(head) {
		fmt.Fprintf(os.Stderr, "warning: git HEAD unresolved (%q); this submission cannot pin to a commit\n", head)
	}

	timeout := cfg.Submit.TimeoutSecs
	if timeout <= 0 {
		timeout = core.SubmitDefaultTimeoutSecs
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	result, runErr := verifyexec.Run(ctx, verifyexec.Options{
		Command: cfg.Submit.Command,
		Dir:     root,
		Stdin:   summary,
	})
	if result.Stdout != "" {
		fmt.Fprint(os.Stdout, result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	rec := core.SubmissionRecord{GitHead: head, SummaryHash: hash, Command: cfg.Submit.Command, Exit: result.ExitCode}
	if _, appendErr := core.WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, core.AppendSubmission(core.SubmissionsPath(root, slug), rec)
	}); appendErr != nil && runErr == nil {
		runErr = appendErr
	}
	if runErr != nil {
		return runErr
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("submit command failed with exit code %d", result.ExitCode)
	}
	fmt.Fprintf(os.Stdout, "submitted %s at %s (summary %s)\n", slug, head, hash[:12])
	return nil
}

// gateFailureMessages renders the error-severity findings of a gate run as
// stable "<gate>: <message>" strings for the submit precondition (spec 08 R1).
func gateFailureMessages(findings []gates.Finding) []string {
	var msgs []string
	for _, f := range findings {
		if f.Severity == gates.Error {
			msgs = append(msgs, fmt.Sprintf("gate %s: %s", f.Gate, f.Message))
		}
	}
	return msgs
}
