package runner_test

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/runner"
)

func TestShRunnerUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh-based runner test is POSIX-only")
	}
	r := runner.NewShRunner()
	if r.Name() != "none" {
		t.Fatalf("default runner Name = %q, want none", r.Name())
	}
	base := runner.RunSpec{Root: t.TempDir(), Shell: "sh", Env: nil, Timeout: 5 * time.Second}

	t.Run("success_captures_stdout_and_exit_0", func(t *testing.T) {
		s := base
		s.Command = "printf hello"
		res := r.Run(context.Background(), s)
		if res.ExitCode != 0 || res.TimedOut {
			t.Fatalf("exit=%d timedOut=%v", res.ExitCode, res.TimedOut)
		}
		if res.Stdout != "hello" {
			t.Errorf("stdout = %q, want hello", res.Stdout)
		}
	})

	t.Run("non_zero_exit_is_preserved", func(t *testing.T) {
		s := base
		s.Command = "exit 3"
		res := r.Run(context.Background(), s)
		if res.ExitCode != 3 || res.TimedOut {
			t.Errorf("exit=%d timedOut=%v, want 3/false", res.ExitCode, res.TimedOut)
		}
	})

	t.Run("stderr_captured_separately", func(t *testing.T) {
		s := base
		s.Command = "printf oops 1>&2; exit 1"
		res := r.Run(context.Background(), s)
		if !strings.Contains(res.Stderr, "oops") || res.Stdout != "" {
			t.Errorf("stdout=%q stderr=%q", res.Stdout, res.Stderr)
		}
	})

	t.Run("timeout_yields_124_timedout", func(t *testing.T) {
		s := base
		s.Command = "sleep 5"
		s.Timeout = 50 * time.Millisecond
		res := r.Run(context.Background(), s)
		if !res.TimedOut || res.ExitCode != 124 {
			t.Errorf("exit=%d timedOut=%v, want 124/true", res.ExitCode, res.TimedOut)
		}
	})
}

func TestRecordSandboxField(t *testing.T) {
	// Default (none) record omits sandbox → byte-compatible with legacy records.
	rec := core.VerificationRecord{Command: "go test", Verified: true, RanAt: "2026-01-01T00:00:00Z"}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "sandbox") {
		t.Errorf("empty sandbox must be omitted: %s", out)
	}

	// A sandboxed record carries the backend name and round-trips.
	rec.Sandbox = "bwrap"
	out, _ = json.Marshal(rec)
	if !strings.Contains(string(out), "\"sandbox\":\"bwrap\"") {
		t.Errorf("sandbox not serialized: %s", out)
	}
	var back core.VerificationRecord
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.Sandbox != "bwrap" {
		t.Errorf("sandbox round-trip = %q", back.Sandbox)
	}

	// Legacy record without the field still parses.
	var legacy core.VerificationRecord
	if err := json.Unmarshal([]byte(`{"command":"x","verified":true,"ranAt":"t"}`), &legacy); err != nil {
		t.Fatalf("legacy parse: %v", err)
	}
	if legacy.Sandbox != "" {
		t.Errorf("legacy sandbox should be empty, got %q", legacy.Sandbox)
	}
}

func TestRevertRecord(t *testing.T) {
	rec := core.VerificationRecord{Command: "go test", Verified: false, RanAt: "t"}
	out, _ := json.Marshal(rec)
	if strings.Contains(string(out), "reverted") || strings.Contains(string(out), "stashRef") {
		t.Errorf("empty revert fields must be omitted: %s", out)
	}
	rec.Reverted = true
	rec.StashRef = "deadbeef"
	out, _ = json.Marshal(rec)
	var back core.VerificationRecord
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Reverted || back.StashRef != "deadbeef" {
		t.Errorf("revert fields lost: %+v", back)
	}
	var legacy core.VerificationRecord
	if err := json.Unmarshal([]byte(`{"command":"x","verified":true,"ranAt":"t"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Reverted || legacy.StashRef != "" {
		t.Errorf("legacy record gained revert fields: %+v", legacy)
	}
}
