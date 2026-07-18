package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// stampableDemo builds a completable demo spec whose single task declares the
// given evidence cell (empty means no declaration column at all).
func stampableDemo(t *testing.T, evidence string) string {
	t.Helper()
	root := newDemoSpec(t)
	if evidence != "" {
		body := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance | evidence |\n|---|---|---|---|---|---|---|\n| T1 | scout | spec.md | - | true | ok | " + evidence + " |\n"
		path := filepath.Join(core.SpecdDir(root), "specs", "demo", "tasks.md")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write tasks: %v", err)
		}
	}
	gitInitRepo(t, root)
	advanceToExecuting(t, root)
	return root
}

func TestVerifyStampsDeclaredTestEvidence(t *testing.T) {
	root := stampableDemo(t, "test/unit-demo")
	out, err := captureStdout(t, func() error { return Run(root, "verify", []string{"demo", "T1"}, nil) })
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(out, "run `specd complete-task demo T1`") {
		t.Fatalf("satisfied contract must keep the complete-task hint: %q", out)
	}
	evals, evalErr := core.LoadEvals(core.EvalStorePath(root, "demo"))
	if evalErr != nil {
		t.Fatalf("load evals: %v", evalErr)
	}
	if len(evals) != 1 {
		t.Fatalf("want one stamped envelope, got %d", len(evals))
	}
	envelope := evals[0]
	if envelope.Producer != core.VerifyProducer || envelope.EvidenceClass != core.EvidenceTest || envelope.CheckID != "unit-demo" {
		t.Fatalf("envelope identity = %+v", envelope)
	}
	if envelope.Verdict != core.EvalPass {
		t.Fatalf("verdict = %q", envelope.Verdict)
	}
	if envelope.ArtifactRef != ".specd/specs/demo/evidence.jsonl#T1" {
		t.Fatalf("artifact_ref = %q, want verify record", envelope.ArtifactRef)
	}
	head := gitHead(root)
	if !core.HeadPinned(head) || envelope.SubjectRevision != head {
		t.Fatalf("subject_revision %q does not pin verify HEAD %q", envelope.SubjectRevision, head)
	}
	if err := core.ValidateEvidenceEnvelope(envelope); err != nil {
		t.Fatalf("stamped envelope invalid: %v", err)
	}
	// The stamped envelope closes the loop: completion succeeds end-to-end
	// with no external `specd eval import`.
	if _, err := captureStdout(t, func() error { return Run(root, "complete-task", []string{"demo", "T1"}, nil) }); err != nil {
		t.Fatalf("complete-task after stamped verify: %v", err)
	}
}

func TestVerifyStampsNothingForNonTestClass(t *testing.T) {
	root := stampableDemo(t, "output_eval/rubric-demo")
	out, err := captureStdout(t, func() error { return Run(root, "verify", []string{"demo", "T1"}, nil) })
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Spec R2.4: this verify run cannot satisfy the declared non-test contract,
	// so the unconditional complete-task hint is suppressed and the outstanding
	// contract and its producer are named instead.
	if strings.Contains(out, "run `specd complete-task") {
		t.Fatalf("unsatisfiable contract still printed the complete-task hint: %q", out)
	}
	for _, want := range []string{"outstanding evidence contract output_eval/rubric-demo", "specd eval import demo <file> --task T1 --check rubric-demo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("verify output missing %q: %q", want, out)
		}
	}
	evals, err := core.LoadEvals(core.EvalStorePath(root, "demo"))
	if err != nil {
		t.Fatalf("load evals: %v", err)
	}
	if len(evals) != 0 {
		t.Fatalf("non-test class must not be stamped by verify, got %d records", len(evals))
	}
}

func TestVerifyStampsNothingWithoutDeclaration(t *testing.T) {
	root := stampableDemo(t, "")
	if _, err := captureStdout(t, func() error { return Run(root, "verify", []string{"demo", "T1"}, nil) }); err != nil {
		t.Fatalf("verify: %v", err)
	}
	evals, err := core.LoadEvals(core.EvalStorePath(root, "demo"))
	if err != nil {
		t.Fatalf("load evals: %v", err)
	}
	if len(evals) != 0 {
		t.Fatalf("undeclared task must not be stamped, got %d records", len(evals))
	}
}
