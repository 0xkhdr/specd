package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
	"github.com/0xkhdr/specd/internal/version"
)

func runVerify(root string, args []string, flags map[string]string) error {
	if _, ok := flags["criterion"]; ok {
		return runVerifyCriterion(root, args, flags)
	}
	if len(args) != 2 {
		return errors.New("usage: specd verify <slug> <task>")
	}
	slug, taskID := args[0], args[1]
	if err := requireTaskGate(root, slug); err != nil {
		return err
	}
	annotations, err := parseAnnotations(flags)
	if err != nil {
		return err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return err
	}
	var task core.TaskRow
	for _, candidate := range spec.Tasks {
		if candidate.ID == taskID {
			task = candidate
			break
		}
	}
	if task.ID == "" {
		return fmt.Errorf("task %s not found", taskID)
	}
	// Escalation ratchet (spec 06 R2): once a task has failed verify N times in a
	// row, block further attempts until a human clears it. This is not a bypass —
	// the override only resets the counter; the task still needs a passing verify.
	if count, err := taskFailCount(root, slug, taskID); err != nil {
		return err
	} else if core.IsEscalated(count, escalationMaxFails(root)) {
		return fmt.Errorf("task %s is escalated after %d consecutive verify failures; clear it with `specd task %s --override --reason <text>` before re-attempting", taskID, count, taskID)
	}
	run := func() (verifyexec.Result, error) {
		cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == "error" {
				return verifyexec.Result{ExitCode: 2}, fmt.Errorf("load config: %s", diagnostic.Message)
			}
		}
		return verifyexec.Run(context.Background(), verifyexec.Options{
			Command:        task.Verify,
			Dir:            root,
			Sandbox:        flagEnabled(flags, "sandbox"),
			RequireSandbox: cfg.Security.RequiresVerifySandbox(),
			SandboxBinary:  flags["sandbox-binary"],
			TimeoutSecs:    verifyTimeoutSecs(root),
		})
	}
	var result verifyexec.Result
	if flagEnabled(flags, "revert-on-fail") {
		result, err = withRevertOnFail(root, run)
	} else {
		result, err = run()
	}
	head := gitHead(root)
	if !core.HeadPinned(head) {
		fmt.Fprintf(os.Stderr, "warning: git HEAD unresolved (%q); this evidence cannot pin to a commit and will not count toward `complete-task`\n", head)
	}
	record := core.EvidenceRecord{TaskID: taskID, Command: task.Verify, ExitCode: result.ExitCode, GitHead: head, Telemetry: annotations}
	if appendErr := core.AppendEvidence(core.EvidencePath(root, slug), record); appendErr != nil && err == nil {
		err = appendErr
	}
	// Allocate this attempt's run/attempt identity through the shared core
	// allocator (spec 07 R2.1/R2.2): a manual verify accrues an attempt on the
	// task's run chain, monotonic through the fail/fail/pass loop. The ledger is
	// additive — an allocation failure never blocks the verify record above.
	runRecord, allocErr := core.AllocateRun(root, slug, taskID, head, "", "", core.TelemetrySourceWorker)
	if allocErr != nil && err == nil {
		err = allocErr
	}
	if result.Stdout != "" {
		fmt.Fprint(os.Stdout, core.TruncateEvidenceOutput(result.Stdout))
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, core.TruncateEvidenceOutput(result.Stderr))
	}
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("verify failed with exit code %d", result.ExitCode)
	}
	// Evidence-envelope stamping (spec R2.1): a passing verify on a task that
	// declares test/<check-id> re-records the same exit-0 + pinned-HEAD fact as
	// a class-tagged envelope in the eval store, closing the verify →
	// complete-task loop for test/* without an external import. Non-test
	// classes are never stamped (R2.2) and an unpinned HEAD stamps nothing —
	// stamping only adds evidence, it never weakens the completion gate.
	var outstanding []core.EvidenceRequirement
	if allocErr == nil && core.HeadPinned(head) {
		if contract, contractErr := core.ParseQualityContract(task); contractErr != nil {
			fmt.Fprintf(os.Stderr, "warning: evidence declaration not stamped: %v\n", contractErr)
		} else {
			storedRecords, loadErr := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
			if loadErr != nil {
				return loadErr
			}
			if len(storedRecords) == 0 {
				return errors.New("passing verify record was not persisted")
			}
			storedRecord := storedRecords[len(storedRecords)-1]
			recordRaw, _ := json.Marshal(storedRecord)
			stamp := core.VerifyStamp{
				SpecSlug:        slug,
				TaskID:          taskID,
				RunID:           runRecord.RunID,
				Attempt:         runRecord.Attempt,
				SubjectRevision: head,
				ProducerVersion: version.Get().Version,
				ConfigDigest:    core.Digest([]byte(task.Verify)),
				ArtifactRef:     ".specd/specs/" + slug + "/evidence.jsonl#" + taskID,
				ArtifactDigest:  core.Digest(recordRaw),
				CreatedAt:       core.Clock(),
			}
			for _, envelope := range core.BuildVerifyEnvelopes(contract, stamp) {
				if stampErr := core.AppendEval(core.EvalStorePath(root, slug), envelope); stampErr != nil {
					return stampErr
				}
			}
			// Spec R2.4: name what this verify run could not satisfy. After
			// stamping, any remaining requirement needs an external producer,
			// so an unconditional complete-task hint would mislead.
			evals, evalErr := core.LoadEvals(core.EvalStorePath(root, slug))
			if evalErr != nil {
				return evalErr
			}
			st := core.EvaluateQuality(contract, evals, core.FreshnessSubject{Revision: head})
			outstanding = append(append(outstanding, st.Missing...), st.Stale...)
		}
	}
	if len(outstanding) > 0 {
		fmt.Fprintf(os.Stdout, "evidence recorded for %s %s; task not complete; outstanding evidence contract %s cannot be satisfied by `specd verify` (producer: `specd eval import`)",
			slug, taskID, core.FormatRequirements(outstanding))
		for _, requirement := range outstanding {
			fmt.Fprintf(os.Stdout, "; import %s/%s with `specd eval import %s <file> --task %s --check %s`", requirement.EvidenceClass, requirement.CheckID, slug, taskID, requirement.CheckID)
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintf(os.Stdout, "evidence recorded for %s %s; task not complete; run `specd complete-task %s %s`\n", slug, taskID, slug, taskID)
	return nil
}

func requireTaskGate(root, slug string) error {
	state, err := core.LoadState(filepath.Join(root, ".specd", "specs", slug, "state.json"))
	if err != nil {
		return err
	}
	switch state.Status {
	case core.StatusTasks, core.StatusComplete:
		return nil
	default:
		if state.Records != nil {
			if _, ok := state.Records["approval:requirements"]; ok {
				if _, ok := state.Records["approval:design"]; ok {
					return nil
				}
			}
		}
		return fmt.Errorf("missing approval: requirements and design gates must be approved before task execution")
	}
}

func withRevertOnFail(root string, run func() (verifyexec.Result, error)) (verifyexec.Result, error) {
	before := gitDiff(root)
	result, err := run()
	if err == nil && result.ExitCode == 0 {
		return result, nil
	}
	after := gitDiff(root)
	if after != "" {
		_ = gitApply(root, after, true)
	}
	if before != "" {
		_ = gitApply(root, before, false)
	}
	return result, err
}

// runVerifyCriterion records a per-acceptance-criterion evidence record. It
// never runs a command and never writes a task verify record — a criterion
// record is operator-supplied and can never substitute for a task's passing
// verify (spec 04 R1, R7). Unknown criterion ids fail closed (exit 2, R2).
func runVerifyCriterion(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return fmt.Errorf("%w: specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>", ErrUsage)
	}
	slug := args[0]
	if err := requireTaskGate(root, slug); err != nil {
		return err
	}
	id := flags["criterion"]
	status := flags["status"]
	if status != core.CriterionStatusPass && status != core.CriterionStatusFail {
		return fmt.Errorf("%w: --status must be pass or fail", ErrUsage)
	}
	evidence := strings.TrimSpace(flags["evidence"])
	if evidence == "" {
		return fmt.Errorf("%w: --evidence <text-or-path> required", ErrUsage)
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	reqDoc, err := os.ReadFile(filepath.Join(dir, "requirements.md"))
	if err != nil {
		return fmt.Errorf("read requirements.md: %w", err)
	}
	if !gates.HasCriterion(string(reqDoc), id) {
		return fmt.Errorf("%w: unknown criterion %q — not an acceptance criterion in approved requirements.md", ErrUsage, id)
	}
	head := gitHead(root)
	if !core.HeadPinned(head) {
		fmt.Fprintf(os.Stderr, "warning: git HEAD unresolved (%q); this criterion record cannot pin to a commit\n", head)
	}
	rec := core.CriterionRecord{Criterion: id, Status: status, Evidence: evidence, GitHead: head}
	path := core.CriteriaPath(root, slug)
	if _, err := core.WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, core.AppendCriterion(path, rec)
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "recorded criterion %s = %s for %s\n", id, status, slug)
	return nil
}
