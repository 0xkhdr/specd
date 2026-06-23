package core

import (
	"strings"
	"testing"
	"time"
)

// acp_misc_validate_cov_test.go covers the pure parse/validate leaves
// parseACPEventFilename and validateACPCursor across their error branches.

func TestParseACPEventFilenameBranches(t *testing.T) {
	id := strings.Repeat("a", 32)
	good := strings.Repeat("0", 19) + "1" + "-" + id + ".json" // width=20, seq=1
	seq, mid, err := parseACPEventFilename(good)
	if err != nil || seq != 1 || mid != id {
		t.Fatalf("good filename → seq=%d mid=%q err=%v", seq, mid, err)
	}

	bad := []string{
		"short.json",
		strings.Repeat("0", 20) + "X" + id + ".json",                                          // missing '-' separator
		strings.Repeat("0", 20) + "-" + id + ".txt",                                           // wrong suffix
		strings.Repeat("0", 20) + "-" + id + ".json",                                          // sequence 0
		strings.Repeat("z", 20) + "-" + id + ".json",                                          // non-numeric sequence
		strings.Repeat("0", 19) + "1" + "-" + "../escape" + strings.Repeat("a", 23) + ".json", // bad messageID length/charset
	}
	for _, name := range bad {
		if _, _, err := parseACPEventFilename(name); err == nil {
			t.Errorf("expected error for %q", name)
		}
	}
}

func TestValidateACPCursorBranches(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	id := strings.Repeat("a", 32)
	valid := ACPCursor{
		Version:   1,
		SessionID: strings.Repeat("s", 32),
		WorkerID:  "worker-1",
		Sequence:  3,
		MessageID: id,
		UpdatedAt: now,
	}
	if err := validateACPCursor(valid, valid.SessionID, valid.WorkerID); err != nil {
		t.Fatalf("valid cursor rejected: %v", err)
	}

	cases := map[string]func(c *ACPCursor){
		"bad version":       func(c *ACPCursor) { c.Version = 2 },
		"messageId no seq":  func(c *ACPCursor) { c.Sequence = 0; c.MessageID = id },
		"seq bad messageId": func(c *ACPCursor) { c.MessageID = "short" },
		"bad updatedAt":     func(c *ACPCursor) { c.UpdatedAt = "nope" },
	}
	for name, mutate := range cases {
		c := valid
		mutate(&c)
		if err := validateACPCursor(c, c.SessionID, c.WorkerID); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	// Identity mismatch on caller-supplied session/worker.
	if err := validateACPCursor(valid, "other", valid.WorkerID); err == nil {
		t.Error("session mismatch should error")
	}
	// Sequence 0 with no messageId is valid.
	zero := valid
	zero.Sequence = 0
	zero.MessageID = ""
	if err := validateACPCursor(zero, zero.SessionID, zero.WorkerID); err != nil {
		t.Errorf("zero-sequence cursor rejected: %v", err)
	}
}
