package core

import (
	"os"
	"testing"
)

// TestDeliveryLedger pins the append/replay ledger contract (spec 08 R6.2):
// round-trip append/read, torn-final-line tolerance, a reproducible content-address
// release identity, idempotent release freezing, and monotonic deployment attempts.
func TestDeliveryLedger(t *testing.T) {
	t.Run("release_round_trip", func(t *testing.T) {
		path := tempLedger(t, "releases.jsonl")
		r := validReleaseCandidate()
		if err := AppendRelease(path, r); err != nil {
			t.Fatalf("append release: %v", err)
		}
		got, err := ReadReleases(path)
		if err != nil {
			t.Fatalf("read releases: %v", err)
		}
		if len(got) != 1 || got[0].ReleaseID != r.ReleaseID {
			t.Fatalf("round-trip mismatch: %+v", got)
		}
	})

	t.Run("deployment_round_trip", func(t *testing.T) {
		path := tempLedger(t, "deployments.jsonl")
		d := validDeployment()
		if err := AppendDeployment(path, d); err != nil {
			t.Fatalf("append deployment: %v", err)
		}
		got, err := ReadDeployments(path)
		if err != nil {
			t.Fatalf("read deployments: %v", err)
		}
		if len(got) != 1 || got[0].DeploymentID != d.DeploymentID {
			t.Fatalf("round-trip mismatch: %+v", got)
		}
	})

	t.Run("torn_final_line_dropped", func(t *testing.T) {
		path := tempLedger(t, "deployments.jsonl")
		if err := AppendDeployment(path, validDeployment()); err != nil {
			t.Fatalf("append: %v", err)
		}
		// Simulate a crash mid-append: a partial JSON line with no newline.
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(`{"schema":"DeploymentV1","deployment_i`); err != nil {
			t.Fatal(err)
		}
		f.Close()
		got, err := ReadDeployments(path)
		if err != nil {
			t.Fatalf("torn line must not error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected prior complete record only, got %d", len(got))
		}
	})

	t.Run("corrupt_middle_line_errors", func(t *testing.T) {
		path := tempLedger(t, "deployments.jsonl")
		if err := AppendFile(path, "{not json}\n"); err != nil {
			t.Fatal(err)
		}
		if err := AppendDeployment(path, validDeployment()); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadDeployments(path); err == nil {
			t.Fatal("corrupt non-final line must fail closed")
		}
	})

	t.Run("release_id_reproducible", func(t *testing.T) {
		r := validReleaseCandidate()
		id1 := ReleaseCandidateID(r)
		r.CreatedAt = "2030-01-01T00:00:00Z" // metadata, not identity
		if ReleaseCandidateID(r) != id1 {
			t.Fatal("created_at must not change release identity")
		}
		r.ArtifactDigest = "sha256:different"
		if ReleaseCandidateID(r) == id1 {
			t.Fatal("artifact digest change must change release identity")
		}
	})

	t.Run("freeze_is_idempotent", func(t *testing.T) {
		root := t.TempDir()
		r := validReleaseCandidate()
		if _, err := FreezeReleaseCandidate(root, "demo", r); err != nil {
			t.Fatalf("freeze: %v", err)
		}
		if _, err := FreezeReleaseCandidate(root, "demo", r); err != nil {
			t.Fatalf("re-freeze: %v", err)
		}
		got, err := ReadReleases(ReleaseLedgerPath(root, "demo"))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("immutable candidate duplicated: %d records", len(got))
		}
	})

	t.Run("deployment_attempts_monotonic", func(t *testing.T) {
		root := t.TempDir()
		d := validDeployment()
		first, err := AppendDeploymentAttempt(root, "demo", d)
		if err != nil {
			t.Fatalf("append attempt 1: %v", err)
		}
		second, err := AppendDeploymentAttempt(root, "demo", d)
		if err != nil {
			t.Fatalf("append attempt 2: %v", err)
		}
		if first.Attempt != 1 || second.Attempt != 2 {
			t.Fatalf("attempts not monotonic: %d, %d", first.Attempt, second.Attempt)
		}
		other := validDeployment()
		other.DeploymentID = "dep-2"
		third, err := AppendDeploymentAttempt(root, "demo", other)
		if err != nil {
			t.Fatalf("append other deployment: %v", err)
		}
		if third.Attempt != 1 {
			t.Fatalf("a distinct deployment_id starts at attempt 1, got %d", third.Attempt)
		}
	})
}

func tempLedger(t *testing.T, name string) string {
	t.Helper()
	return t.TempDir() + string(os.PathSeparator) + name
}
