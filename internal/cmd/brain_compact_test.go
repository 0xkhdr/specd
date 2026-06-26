package cmd_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/obs"
	"github.com/0xkhdr/specd/internal/testharness"
)

// TestBrainCompactClearLedger is the CLI smoke test for the compact/clear/ledger
// subcommands (T3/T9): compact and clear each append a compacted ledger entry and
// write a summary file; ledger --json reports them as machine-parseable output
// with zero LLM calls.
func TestBrainCompactClearLedger(t *testing.T) {
	h := testharness.New(t)
	slug := h.Spec("compact-spec").
		Req("compact", "As an operator I can shed context.", "THE SYSTEM SHALL shed context.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).
		Build()
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID := strings.Repeat("a", 32)
	host.StartSpec(slug, sessionID)

	// compact with an explicit reason.
	compact := h.RunExpect(core.ExitOK, "brain", "compact", slug, "--session", sessionID, "--reason", "phase-complete", "--json")
	var outcome core.CompactionOutcome
	if err := json.Unmarshal([]byte(compact.Stdout), &outcome); err != nil {
		t.Fatalf("compact JSON: %v\n%s", err, compact.Stdout)
	}
	if !outcome.Entry.Compacted || outcome.Entry.Reason != "phase-complete" {
		t.Fatalf("compact entry = %+v, want compacted phase-complete", outcome.Entry)
	}
	if !strings.Contains(outcome.SummaryFile, ".specd/runtime/sessions/") ||
		!strings.Contains(outcome.SummaryFile, "compact-") ||
		!strings.HasSuffix(outcome.SummaryFile, ".md") {
		t.Fatalf("summary file = %q, want .specd/sessions/.../compact-*.md", outcome.SummaryFile)
	}

	// clear is compact with reason manual-clear.
	clear := h.RunExpect(core.ExitOK, "brain", "clear", slug, "--session", sessionID, "--json")
	var clearOut core.CompactionOutcome
	if err := json.Unmarshal([]byte(clear.Stdout), &clearOut); err != nil {
		t.Fatalf("clear JSON: %v\n%s", err, clear.Stdout)
	}
	if clearOut.Entry.Reason != "manual-clear" {
		t.Fatalf("clear reason = %q, want manual-clear", clearOut.Entry.Reason)
	}

	// ledger --json must be valid JSON listing both compaction entries.
	ledger := h.RunExpect(core.ExitOK, "brain", "ledger", slug, "--session", sessionID, "--json")
	var report struct {
		Session            string                    `json:"session"`
		PeakTokens         int                       `json:"peakTokens"`
		LastCompactionStep uint64                    `json:"lastCompactionStep"`
		Ledger             []core.ContextLedgerEntry `json:"ledger"`
	}
	if err := json.Unmarshal([]byte(ledger.Stdout), &report); err != nil {
		t.Fatalf("ledger JSON: %v\n%s", err, ledger.Stdout)
	}
	compacted := 0
	for _, e := range report.Ledger {
		if e.Compacted {
			compacted++
		}
	}
	if compacted != 2 {
		t.Fatalf("ledger has %d compacted entries, want 2", compacted)
	}

	// ledger human form prints the summary header.
	human := h.RunExpect(core.ExitOK, "brain", "ledger", slug, "--session", sessionID)
	if !strings.Contains(human.Stdout, "brain ledger") || !strings.Contains(human.Stdout, "peak tokens") {
		t.Fatalf("ledger human output missing header: %s", human.Stdout)
	}

	// T8: the context.compact event is on the obs timeline (replay-visible).
	timeline, err := obs.ReadTimeline(h.Root, sessionID)
	if err != nil {
		t.Fatalf("ReadTimeline: %v", err)
	}
	compactEvents := 0
	for _, e := range timeline {
		if e.Event == "context.compact" {
			compactEvents++
		}
	}
	if compactEvents != 2 {
		t.Fatalf("timeline has %d context.compact events, want 2", compactEvents)
	}
}

// TestBrainCompactRequiresRunningSession guards the status check: compacting a
// non-running session is refused.
func TestBrainCompactUnknownSession(t *testing.T) {
	h := testharness.New(t)
	slug := h.Spec("compact-missing").
		Req("compact", "As an operator I see clear errors.", "THE SYSTEM SHALL report missing sessions.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		Build()
	res := h.Run("brain", "compact", slug, "--session", strings.Repeat("b", 32))
	if res.Code == core.ExitOK {
		t.Fatalf("compact on missing session unexpectedly succeeded: %s", res.Out())
	}
}
