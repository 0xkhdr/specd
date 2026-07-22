package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// reopenCLISpec is a demo spec whose only task is already complete, inside a
// real git repository so the reopen can pin a fresh baseline.
func reopenCLISpec(t *testing.T) string {
	t.Helper()
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)
	writeTasks(t, root, "demo", "| ✅ T1 | scout | spec.md | - | printf ok | reopen fixture |")
	if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte("# spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// reopenRevision is the state revision a reopen must be previewed against.
func reopenRevision(t *testing.T, root string) string {
	t.Helper()
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	return strconv.FormatInt(state.Revision, 10)
}

func reopenCLI(t *testing.T, root string, flags map[string]string) (core.ReopenPlan, error) {
	t.Helper()
	out, err := captureStdout(t, func() error {
		return Run(root, "reopen", []string{"demo", "task", "T1"}, flags)
	})
	if err != nil {
		return core.ReopenPlan{}, err
	}
	var plan core.ReopenPlan
	if jsonErr := json.Unmarshal([]byte(out), &plan); jsonErr != nil {
		t.Fatalf("reopen json: %v (out=%q)", jsonErr, out)
	}
	return plan, nil
}

func TestTaskReopenAttemptBindingCLICreatesAttempt(t *testing.T) {
	root := reopenCLISpec(t)
	plan, err := reopenCLI(t, root, map[string]string{"reason": "rounding defect found in review", "expect-revision": reopenRevision(t, root)})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !plan.Eligible || plan.Attempt.Attempt != 2 || plan.EventID == "" {
		t.Fatalf("plan = %+v, want an eligible second attempt", plan)
	}
	if !core.HeadPinned(plan.Attempt.Baseline) || plan.Attempt.AuthorityDigest == "" {
		t.Fatalf("attempt = %+v, want a fresh baseline and authority digest", plan.Attempt)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil || state.Revision != plan.NewRevision {
		t.Fatalf("state = %+v, err %v, want the attempt committed at revision %d", state, err, plan.NewRevision)
	}
	// tasks.md is never rewritten by a reopen.
	raw, err := os.ReadFile(filepath.Join(core.SpecdDir(root), "specs", "demo", "tasks.md"))
	if err != nil || !strings.Contains(string(raw), "✅ T1") {
		t.Fatalf("tasks.md = %q, err %v, want the marker untouched", raw, err)
	}
}

func TestTaskReopenAttemptBindingCLIRefusesPriorAttemptEvidence(t *testing.T) {
	root := reopenCLISpec(t)
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := reopenCLI(t, root, map[string]string{"reason": "defect found after completion", "expect-revision": reopenRevision(t, root)}); err != nil {
		t.Fatalf("reopen: %v", err)
	}

	t.Run("prior-evidence", func(t *testing.T) {
		// The attempt-1 record is still on disk with the same command, files, and
		// git HEAD; it simply no longer counts, so completion sees no record.
		err := Run(root, "complete-task", []string{"demo", "T1"}, nil)
		if err == nil || !strings.Contains(err.Error(), "no task verify record") {
			t.Fatalf("complete-task = %v, want prior-attempt evidence refused", err)
		}
		records, loadErr := core.LoadEvidenceRecords(core.EvidencePath(root, "demo"))
		if loadErr != nil || len(records) != 1 || records[0].ExitCode != 0 {
			t.Fatalf("history = %+v, err %v, want the prior attempt preserved verbatim", records, loadErr)
		}
	})

	t.Run("re-verified-attempt-completes", func(t *testing.T) {
		if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
			t.Fatalf("verify: %v", err)
		}
		if err := Run(root, "complete-task", []string{"demo", "T1"}, nil); err != nil {
			t.Fatalf("complete-task: %v", err)
		}
	})
}

func TestTaskReopenAttemptBindingCLIRefusesLiveLease(t *testing.T) {
	root := reopenCLISpec(t)
	path := filepath.Join(core.SpecdDir(root), "specs", "demo", "session.json")
	session := orchestration.Session{Leases: []orchestration.Lease{{LeaseID: "lease-7", TaskID: "T1", WorkerID: "worker-9"}}}
	if err := orchestration.SaveSessionCAS(root, path, 0, session); err != nil {
		t.Fatal(err)
	}
	flags := map[string]string{"reason": "defect found after completion", "expect-revision": reopenRevision(t, root)}

	t.Run("live-lease", func(t *testing.T) {
		_, err := reopenCLI(t, root, flags)
		if err == nil || !strings.Contains(err.Error(), "--revoke-lease lease-7") {
			t.Fatalf("reopen = %v, want a refusal naming the exact revoke recovery", err)
		}
		if events, readErr := core.ReadWorkflowEvents(core.WorkflowEventPath(root, "demo")); readErr != nil || len(events) != 0 {
			t.Fatalf("ledger = %d events, err %v, want a refusal to mutate nothing", len(events), readErr)
		}
	})

	t.Run("authorized-revocation", func(t *testing.T) {
		flags["revoke-lease"] = "lease-7"
		plan, err := reopenCLI(t, root, flags)
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		if !plan.Eligible || plan.Attempt.Attempt != 2 {
			t.Fatalf("plan = %+v, want the attempt created once the lease is revoked", plan)
		}
		current, err := orchestration.LoadSession(path)
		if err != nil || len(current.Leases) != 0 {
			t.Fatalf("session = %+v, err %v, want the lease surrendered in the same transaction", current, err)
		}
	})
}

func TestTaskReopenAttemptBindingCLIRefusesMalformedInvocations(t *testing.T) {
	root := reopenCLISpec(t)
	rev := reopenRevision(t, root)
	cases := map[string]struct {
		args  []string
		flags map[string]string
	}{
		"missing-reason":          {[]string{"demo", "task", "T1"}, map[string]string{"expect-revision": rev}},
		"missing-expect-revision": {[]string{"demo", "task", "T1"}, map[string]string{"reason": "x"}},
		"negative-revision":       {[]string{"demo", "task", "T1"}, map[string]string{"reason": "x", "expect-revision": "-1"}},
		"unknown-entity-kind":     {[]string{"demo", "spec"}, map[string]string{"reason": "x", "expect-revision": rev}},
		"missing-task":            {[]string{"demo", "task"}, map[string]string{"reason": "x", "expect-revision": rev}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Run(root, "reopen", tc.args, tc.flags); err == nil {
				t.Fatal("malformed reopen must fail closed")
			}
		})
	}
	t.Run("stale-revision", func(t *testing.T) {
		err := Run(root, "reopen", []string{"demo", "task", "T1"}, map[string]string{"reason": "x", "expect-revision": "99"})
		if err == nil || !strings.Contains(err.Error(), "REOPEN_REVISION_STALE") {
			t.Fatalf("reopen = %v, want a stale-revision refusal", err)
		}
	})
}
