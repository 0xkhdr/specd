package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityLedgerRedactsAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quality.jsonl")
	entry := QualityLedgerEntry{ID: "failure-1", Kind: "failure", Taxonomy: "shallow-verify", EvidenceRef: "evidence.jsonl#1", SourceDigest: strings.Repeat("a", 64), CreatedAt: "2026-01-01T00:00:00Z"}
	if err := AppendQualityLedger(path, entry); err != nil {
		t.Fatal(err)
	}
	if err := AppendQualityLedger(path, entry); err == nil {
		t.Fatal("duplicate ledger identity accepted")
	}
	entries, err := LoadQualityLedger(path)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	entry.Taxonomy = "secret-token\nleak"
	if err := AppendQualityLedger(path, entry); err == nil {
		t.Fatal("unredacted multiline taxonomy accepted")
	}
}
