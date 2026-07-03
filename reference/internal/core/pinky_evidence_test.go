package core

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// evidenceFixture writes the demo spec, claims a craftsman lease for "wkr" on T1,
// and injects a passing specd verification record (the truth the reconciler must
// bind to). It returns that record so a test can derive a matching report or
// mutate a copy to forge one.
func evidenceFixture(t *testing.T) (root, sessionID string, cfg OrchestrationCfg, rec *VerificationRecord) {
	t.Helper()
	root = writePinkySpec(t)
	sessionID = strings.Repeat("7", 32)
	cfg = DefaultConfig.Orchestration
	clock := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return clock })
	t.Cleanup(restore)

	claimDemoLease(t, root, sessionID, "wkr", 1, cfg)

	head := "deadbeefcafefeed"
	rec = &VerificationRecord{
		Command:      "go test ./internal/core",
		ExitCode:     0,
		Verified:     true,
		RanAt:        NowISO(),
		GitHead:      &head,
		ChangedFiles: []string{"internal/core/demo.go"},
	}
	setTaskVerification(t, root, "T1", rec)
	return root, sessionID, cfg, rec
}

func setTaskVerification(t *testing.T, root, id string, rec *VerificationRecord) {
	t.Helper()
	loaded, err := LoadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	ts := loaded.State.Tasks[id]
	ts.Verification = rec
	loaded.State.Tasks[id] = ts
	if err := SaveState(root, "demo", loaded.State); err != nil {
		t.Fatal(err)
	}
}

// setTaskRole rewrites the role in tasks.md (the authoritative source LoadSpec
// re-syncs task state from) so the change survives a reload.
func setTaskRole(t *testing.T, root, role string) {
	t.Helper()
	path := ArtifactPath(root, "demo", "tasks.md")
	raw := ReadOrDefault(path, "")
	updated := strings.Replace(raw, "- role: craftsman", "- role: "+role, 1)
	if updated == raw {
		t.Fatalf("role line not found in tasks.md")
	}
	if err := AtomicWrite(path, updated); err != nil {
		t.Fatal(err)
	}
}

func validEvidenceReport(sessionID string, rec *VerificationRecord) PinkyTerminalReport {
	return PinkyTerminalReport{
		SessionID:       sessionID,
		WorkerID:        "wkr",
		Spec:            "demo",
		TaskID:          "T1",
		Attempt:         1,
		VerificationRef: VerificationRef(rec),
		Summary:         "implemented demo",
		ChangedFiles:    append([]string{}, rec.ChangedFiles...),
		GitHead:         *rec.GitHead,
		DurationMs:      120,
	}
}

func taskStatus(t *testing.T, root, id string) TaskStatus {
	t.Helper()
	loaded, err := LoadSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	return loaded.State.Tasks[id].Status
}

func TestPinkyEvidenceCompletesThroughIntegrityPath(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	report := validEvidenceReport(sessionID, rec)

	res, err := ReconcilePinkyEvidence(root, report, cfg)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Completion.Status != TaskComplete || res.Completion.AlreadyComplete {
		t.Fatalf("completion = %#v, want fresh complete", res.Completion)
	}
	if got := taskStatus(t, root, "T1"); got != TaskComplete {
		t.Fatalf("task status = %s, want complete", got)
	}

	// A duplicate report re-records nothing and repeats no transition (V3).
	dup, err := ReconcilePinkyEvidence(root, report, cfg)
	if err != nil {
		t.Fatalf("duplicate reconcile: %v", err)
	}
	if !dup.Completion.AlreadyComplete {
		t.Fatalf("duplicate completion = %#v, want already-complete", dup.Completion)
	}
	if dup.Event.MessageID != res.Event.MessageID {
		t.Fatalf("duplicate evidence event differs: %s vs %s", dup.Event.MessageID, res.Event.MessageID)
	}
}

func TestPinkyEvidenceRejectsForgedRef(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	report := validEvidenceReport(sessionID, rec)
	report.VerificationRef = strings.Repeat("0", 32)

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("forged verificationRef accepted")
	}
	if got := taskStatus(t, root, "T1"); got == TaskComplete {
		t.Fatal("task completed on forged evidence")
	}
}

func TestPinkyEvidenceRejectsStaleGitHead(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	report := validEvidenceReport(sessionID, rec)
	report.GitHead = "0000000000000000"

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("stale git head accepted")
	}
	if got := taskStatus(t, root, "T1"); got == TaskComplete {
		t.Fatal("task completed on stale head")
	}
}

func TestPinkyEvidenceRejectsChangedVerifyCommand(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	// The recorded command no longer matches the task's verify line.
	stale := *rec
	stale.Command = "go test ./changed"
	setTaskVerification(t, root, "T1", &stale)
	report := validEvidenceReport(sessionID, &stale)

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("stale verify command accepted")
	}
	if got := taskStatus(t, root, "T1"); got == TaskComplete {
		t.Fatal("task completed on stale command")
	}
}

func TestPinkyEvidenceRejectsUndeclaredFilesWhenScopeError(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	if err := AtomicWrite(filepath.Join(root, ".specd", "config.yml"), "gates:\n  scope: error\n"); err != nil {
		t.Fatal(err)
	}
	scoped := *rec
	scoped.ChangedFiles = []string{"internal/core/demo.go", "cmd/evil.go"}
	setTaskVerification(t, root, "T1", &scoped)
	report := validEvidenceReport(sessionID, &scoped)

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("undeclared changed file accepted under scope=error")
	}
	if got := taskStatus(t, root, "T1"); got == TaskComplete {
		t.Fatal("task completed despite scope violation")
	}
}

func TestPinkyEvidenceRejectsReadOnlyRole(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	setTaskRole(t, root, "auditor")
	report := validEvidenceReport(sessionID, rec)

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("read-only role submitted verified completion evidence")
	}
	if got := taskStatus(t, root, "T1"); got == TaskComplete {
		t.Fatal("read-only task completed via evidence path")
	}
}

func TestPinkyEvidenceRejectsMissingVerification(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	setTaskVerification(t, root, "T1", nil)
	report := validEvidenceReport(sessionID, rec)

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("completion accepted with no verification record")
	}
}

func TestPinkyEvidenceRejectsWrongWorker(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	report := validEvidenceReport(sessionID, rec)
	report.WorkerID = "intruder" // holds no lease

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("non-lease-owner submitted evidence")
	}
	if got := taskStatus(t, root, "T1"); got == TaskComplete {
		t.Fatal("task completed for non-owner")
	}
}

func TestPinkyEvidenceRejectsMismatchedChangedFiles(t *testing.T) {
	root, sessionID, cfg, rec := evidenceFixture(t)
	report := validEvidenceReport(sessionID, rec)
	report.ChangedFiles = []string{filepath.ToSlash("internal/core/other.go")}

	if _, err := ReconcilePinkyEvidence(root, report, cfg); err == nil {
		t.Fatal("divergent changed-file claim accepted")
	}
	if got := taskStatus(t, root, "T1"); got == TaskComplete {
		t.Fatal("task completed on mismatched file claim")
	}
}
