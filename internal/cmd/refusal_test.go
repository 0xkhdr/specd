package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestTypedRefusalUnknownCommandFailsClosed(t *testing.T) {
	err := RefuseUnknownCommand("nope")

	// The dispatcher classifies on this sentinel to exit 2. Adopting the typed
	// shape must not change that.
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatal("unknown-command refusal no longer matches ErrUnknownCommand")
	}

	refusal, ok := core.AsRefusal(err)
	if !ok {
		t.Fatalf("refusal is untyped: %v", err)
	}
	if refusal.Code != "UNKNOWN_COMMAND" {
		t.Fatalf("code=%q", refusal.Code)
	}
	if refusal.AuthorityConsumed {
		t.Fatal("refusal before authority issue reports authority_consumed true")
	}
	if refusal.RecoveryCommand == "" || refusal.ActorRequired != core.RefusalActorAgent {
		t.Fatalf("refusal leaves recovery unstated: %#v", refusal)
	}
}

// TestTypedRefusalDispatchSitesAreTyped enumerates the refusals reachable
// through dispatch and asserts each returns the one structured shape. R4.2 is
// about coverage: a single untyped path is where an agent starts improvising.
func TestTypedRefusalDispatchSitesAreTyped(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		code string
		verb string
		args []string
		flag map[string]string
	}{
		{name: "unknown-command", code: "UNKNOWN_COMMAND", verb: "definitely-not-a-verb"},
		{name: "traversal-slug", code: "SPEC_INVALID", verb: "check", args: []string{"../../escape"}},
		{name: "flag-enum", code: "FLAG_VALUE_INVALID", verb: "link", args: []string{"a", "b"}, flag: map[string]string{"kind": "not-a-kind"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(root, tc.verb, tc.args, tc.flag)
			refusal, ok := core.AsRefusal(err)
			if !ok {
				t.Fatalf("untyped refusal: %v", err)
			}
			if refusal.Code != tc.code {
				t.Fatalf("code=%q want %q", refusal.Code, tc.code)
			}
			if refusal.Blocker == "" || refusal.ActorRequired == "" || refusal.RecoveryCommand == "" {
				t.Fatalf("incomplete refusal: %#v", refusal)
			}
			// None of these ran an operation, so none burned a packet.
			if refusal.AuthorityConsumed {
				t.Fatalf("%s reports authority_consumed before authority issue", tc.name)
			}
		})
	}
}

func TestTypedRefusalReachesRunUnchanged(t *testing.T) {
	err := Run(t.TempDir(), "definitely-not-a-verb", nil, nil)
	if err == nil {
		t.Fatal("unknown verb did not fail closed")
	}
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("Run error lost the sentinel: %v", err)
	}
	if _, ok := core.AsRefusal(err); !ok {
		t.Fatalf("Run returned an untyped refusal: %v", err)
	}
}

func TestRefusalRecoveryContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		rec  *core.EvidenceRecord
		code string
	}{
		{name: "missing", code: "EVIDENCE_MISSING"},
		{name: "failing", rec: &core.EvidenceRecord{TaskID: "T1", ExitCode: 1, GitHead: "abc"}, code: "EVIDENCE_FAILING"},
		{name: "stale", rec: &core.EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: "old-head"}, code: "EVIDENCE_STALE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newDemoSpec(t)
			if tc.rec != nil {
				if err := core.AppendEvidence(core.EvidencePath(root, "demo"), *tc.rec); err != nil {
					t.Fatal(err)
				}
			}
			before, err := core.LoadState(core.StatePath(root, "demo"))
			if err != nil {
				t.Fatal(err)
			}
			err = runTaskComplete(root, []string{"demo", "T1"}, nil)
			refusal, ok := core.AsRefusal(err)
			if !ok || refusal.Code != tc.code {
				t.Fatalf("complete-task refusal = %#v, %v; want %s", refusal, err, tc.code)
			}
			if refusal.Category != "evidence" || refusal.Entity != "demo/T1" || refusal.Observed == "" || refusal.Expected == "" || refusal.StateChanged || !refusal.Retryable {
				t.Fatalf("incomplete or untruthful refusal: %#v", refusal)
			}
			if len(refusal.RecoveryOperations) != 1 || refusal.RecoveryOperations[0].Operation != "verify.task" || refusal.RecoveryOperations[0].Command != "specd verify demo T1" {
				t.Fatalf("illegal recovery: %#v", refusal.RecoveryOperations)
			}
			after, loadErr := core.LoadState(core.StatePath(root, "demo"))
			if loadErr != nil || after.Revision != before.Revision {
				t.Fatalf("refused completion changed state: before=%d after=%d err=%v", before.Revision, after.Revision, loadErr)
			}
		})
	}

	t.Run("checkpoint-effect", func(t *testing.T) {
		root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
		writeBrainSingleTask(t, root)
		spec, err := loadSpec(root, "demo")
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		missionID := orchestration.MissionID("demo", 1, "T1")
		acpPath := filepath.Join(core.SpecdDir(root), "specs", "demo", "acp.jsonl")
		if err := orchestration.AppendDispatch(acpPath, orchestration.ACPEvent{Time: now, TaskID: "T1", MissionID: missionID}); err != nil {
			t.Fatal(err)
		}
		session := orchestration.Session{ID: "demo"}
		dispatcher := sessionDispatcher{
			root: root, slug: "demo", tasks: spec.Tasks, config: loadSpecConfig(root), acpPath: acpPath,
			checkpointPath: orchestration.CheckpointPath(root, "demo"), now: now, session: &session,
		}
		err = dispatcher.Dispatch(core.FrontierTask{ID: "T1"})
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "DISPATCH_LEDGER_FAILED" {
			t.Fatalf("dispatch refusal = %#v, %v", refusal, err)
		}
		if !refusal.StateChanged || refusal.CheckpointID != missionID || refusal.Retryable || refusal.ActorRequired != core.RefusalActorOperator {
			t.Fatalf("checkpoint effect not disclosed: %#v", refusal)
		}
		if len(refusal.RecoveryOperations) != 1 || refusal.RecoveryOperations[0].Operation != "brain.resume" || refusal.RecoveryOperations[0].InPlace {
			t.Fatalf("checkpoint recovery = %#v", refusal.RecoveryOperations)
		}
		checkpoint, exists, loadErr := orchestration.LoadCheckpoint(orchestration.CheckpointPath(root, "demo"))
		if loadErr != nil || !exists || checkpoint.MissionID != missionID {
			t.Fatalf("checkpoint missing after disclosed mutation: %+v exists=%v err=%v", checkpoint, exists, loadErr)
		}
	})

	t.Run("failing-eval-is-not-missing", func(t *testing.T) {
		root := newDemoSpec(t)
		tasks := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance | evidence |\n|---|---|---|---|---|---|---|\n| T1 | scout | spec.md | - | true | ok | review/audit |\n"
		if err := os.WriteFile(filepath.Join(core.SpecdDir(root), "specs", "demo", "tasks.md"), []byte(tasks), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGit(t, root, "init")
		mustGit(t, root, "add", ".")
		mustGit(t, root, "commit", "-m", "baseline", "--no-gpg-sign")
		head := gitHead(root)
		if err := core.AppendEvidence(core.EvidencePath(root, "demo"), core.EvidenceRecord{TaskID: "T1", Command: "true", ExitCode: 0, GitHead: head}); err != nil {
			t.Fatal(err)
		}
		if err := core.AppendEval(core.EvalStorePath(root, "demo"), core.EvidenceEnvelopeV1{
			SchemaVersion: core.EvalSchemaVersion, EvidenceID: "failed-audit", EvidenceClass: core.EvidenceReview,
			SpecSlug: "demo", TaskID: "T1", RunID: "run-1", Attempt: 1, SubjectRevision: head,
			Producer: "auditor", ProducerVersion: "1", ConfigDigest: "config", CheckID: "audit", Verdict: core.EvalFail,
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), Actor: "auditor",
			ArtifactRef: "review.md", ArtifactDigest: "artifact",
		}); err != nil {
			t.Fatal(err)
		}
		err := runTaskComplete(root, []string{"demo", "T1"}, nil)
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "EVIDENCE_FAILING" {
			t.Fatalf("failing eval refusal = %#v, %v", refusal, err)
		}
		if strings.Contains(refusal.Detail, "remove the declaration") || len(refusal.RecoveryOperations) != 1 || refusal.RecoveryOperations[0].Operation != "eval.import" {
			t.Fatalf("failing eval advertised bypass or wrong recovery: %#v", refusal)
		}
	})
}
