package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestDriftByteStable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "drift.json"), []byte(`{"schema_version":1,"invariants":[{"id":"INV-1","path":"config/app.json","evidence_task":"T1","severity":"high"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AppendEvidence(core.EvidencePath(root, "demo"), core.EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}); err != nil {
		t.Fatal(err)
	}

	first, err := captureStdout(t, func() error { return runDrift(root, []string{"demo"}, map[string]string{"json": "true"}) })
	if err != nil {
		t.Fatal(err)
	}
	second, err := captureStdout(t, func() error { return runDrift(root, []string{"demo"}, map[string]string{"json": "true"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(first), []byte(second)) {
		t.Fatalf("output changed:\n%s\n%s", first, second)
	}
	want := `{"source":"invariant:INV-1","path":"config/app.json","severity":"high","status":"holds","last_passing_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","suggested_command":"specd new demo-drift"}` + "\n"
	if first != want {
		t.Fatalf("output = %q, want %q", first, want)
	}
}
