package core

import (
	"os"
	"testing"
)

// seedSpecWithFiles writes a spec whose single task declares a files contract,
// plus its state, so correlation can match frame paths against it.
func seedSpecWithFiles(t *testing.T, root, slug, files string, status SpecStatus) {
	t.Helper()
	dir := SpecDir(root, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := "# Tasks — " + slug + "\n\n## Wave 1\n\n- [ ] T1 — Charge\n" +
		"  - why: charge the card\n  - role: craftsman\n  - files: " + files + "\n" +
		"  - contract: charges the card\n  - acceptance: card charged\n  - verify: true\n" +
		"  - depends: none\n  - requirements: 1\n"
	if err := AtomicWrite(ArtifactPath(root, slug, "tasks.md"), tasks); err != nil {
		t.Fatal(err)
	}
	st := InitialState(slug, slug)
	st.Status = status
	if err := SaveState(root, slug, &st); err != nil {
		t.Fatal(err)
	}
}

// TestCorrelateFileMatchMedium: a frame matching a task contract with no deploy
// history is medium confidence.
func TestCorrelateFileMatchMedium(t *testing.T) {
	root := t.TempDir()
	seedSpecWithFiles(t, root, "billing", "internal/svc/*.go", StatusExecuting)

	p := ErrorPayload{Severity: "error", Message: "boom", Environment: "prod", Frames: []StackFrame{{File: "internal/svc/charge.go"}}}
	c, err := CorrelatePayload(root, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Spec != "billing" || c.Confidence != "medium" || c.Impact != "high" {
		t.Fatalf("correlation = %+v, want billing/medium/high", c)
	}
	if len(c.MatchedFiles) != 1 || c.MatchedFiles[0] != "internal/svc/charge.go" {
		t.Errorf("matched files = %v", c.MatchedFiles)
	}
}

// TestCorrelateFileMatchPlusDeployHigh: a file match on a spec that has a
// recorded deploy to the same env is high confidence.
func TestCorrelateFileMatchPlusDeployHigh(t *testing.T) {
	root := t.TempDir()
	seedSpecWithFiles(t, root, "billing", "internal/svc/*.go", StatusComplete)
	st, _ := LoadState(root, "billing")
	st.Deploy = &DeployRecord{Env: "prod", Outcome: "succeeded", Time: NowISO()}
	_ = SaveState(root, "billing", st)

	p := ErrorPayload{Severity: "critical", Message: "panic", Environment: "prod", Frames: []StackFrame{{File: "internal/svc/charge.go"}}}
	c, err := CorrelatePayload(root, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Confidence != "high" || c.Impact != "critical" {
		t.Fatalf("correlation = %+v, want high/critical", c)
	}
}

// TestCorrelateDeployFallbackLow: no file matches, but a recent deploy to the
// env attributes the error at low confidence.
func TestCorrelateDeployFallbackLow(t *testing.T) {
	root := t.TempDir()
	seedSpecWithFiles(t, root, "billing", "internal/other/*.go", StatusComplete)
	st, _ := LoadState(root, "billing")
	st.Deploy = &DeployRecord{Env: "staging", Outcome: "succeeded", Time: NowISO()}
	_ = SaveState(root, "billing", st)

	p := ErrorPayload{Severity: "warning", Message: "slow", Environment: "staging"}
	c, err := CorrelatePayload(root, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Spec != "billing" || c.Confidence != "low" {
		t.Fatalf("correlation = %+v, want billing/low", c)
	}
}

// TestCorrelateNoMatchErrors: nothing correlates → error asking for --spec.
func TestCorrelateNoMatchErrors(t *testing.T) {
	root := t.TempDir()
	seedSpecWithFiles(t, root, "billing", "internal/other/*.go", StatusExecuting)
	p := ErrorPayload{Severity: "error", Message: "x", Environment: "prod"}
	if _, err := CorrelatePayload(root, p, ""); err == nil {
		t.Fatal("expected no-correlation error")
	}
}

// TestCorrelateForcedSpec: --spec pins attribution even with no match (low).
func TestCorrelateForcedSpec(t *testing.T) {
	root := t.TempDir()
	seedSpecWithFiles(t, root, "billing", "internal/other/*.go", StatusExecuting)
	p := ErrorPayload{Severity: "error", Message: "x"}
	c, err := CorrelatePayload(root, p, "billing")
	if err != nil {
		t.Fatal(err)
	}
	if c.Spec != "billing" || c.Confidence != "low" {
		t.Fatalf("forced correlation = %+v", c)
	}
}

// TestLoadDeployPlanAndEnvs covers the on-disk loader and env listing.
func TestLoadDeployPlanAndEnvs(t *testing.T) {
	root := t.TempDir()
	dir := DeployPlanPath(root, "staging")
	if err := os.MkdirAll(dirOf(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"steps":[{"name":"a","command":"true","timeoutSeconds":1}]}`
	if err := os.WriteFile(dir, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := LoadDeployPlan(root, "staging")
	if err != nil || len(plan.Steps) != 1 {
		t.Fatalf("LoadDeployPlan: %v %+v", err, plan)
	}
	if envs := SortedDeployEnvs(root); len(envs) != 1 || envs[0] != "staging" {
		t.Errorf("SortedDeployEnvs = %v, want [staging]", envs)
	}
	// Missing env → not-found.
	if _, err := LoadDeployPlan(root, "prod"); err == nil {
		t.Error("missing plan should error")
	}
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
