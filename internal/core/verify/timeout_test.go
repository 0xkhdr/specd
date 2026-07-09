package verify

import (
	"context"
	"strings"
	"testing"
)

// TestTimeoutRecordsFailingResult pins gap 4.2: a verify that outruns its deadline
// is reported as a failing Result (exit 124) with no error, so the caller records
// failing evidence instead of the pipeline hanging or crashing.
func TestTimeoutRecordsFailingResult(t *testing.T) {
	result, err := Run(context.Background(), Options{
		Command:     "sleep 5",
		Dir:         t.TempDir(),
		TimeoutSecs: 1,
	})
	if err != nil {
		t.Fatalf("Run returned error %v, want nil (timeout is failing evidence, not a crash)", err)
	}
	if result.ExitCode != TimeoutExitCode {
		t.Fatalf("timed-out verify exit = %d, want %d", result.ExitCode, TimeoutExitCode)
	}
	if !strings.Contains(result.Stderr, "timed out") {
		t.Errorf("timed-out verify stderr = %q, want a timeout note", result.Stderr)
	}
}

// TestNoTimeoutWhenUnset confirms an unset (zero) timeout leaves fast commands
// untouched — the bound is opt-in and never bites a normal verify.
func TestNoTimeoutWhenUnset(t *testing.T) {
	result, err := Run(context.Background(), Options{Command: "printf ok", Dir: t.TempDir()})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("Run(printf ok) = (%+v, %v), want exit 0 no error", result, err)
	}
}
