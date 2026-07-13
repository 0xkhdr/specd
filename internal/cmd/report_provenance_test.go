package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestProvenanceHistory(t *testing.T) {
	root := newHistoryDemo(t)
	raw := []byte(`{"schema_version":1,"source_type":"incident","source_ref":"INC-42","systems":["api"],"severity":"high","owner":"sre","prior_links":["payments"]}`)
	if err := os.WriteFile(core.ProvenancePath(root, "demo"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"history": ""})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"provenance", "source_type=incident", "source_ref=INC-42", "systems=api", "severity=high", "owner=sre", "prior_links=payments"} {
		if !strings.Contains(first, want) {
			t.Fatalf("history missing %q:\n%s", want, first)
		}
	}
	second, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"history": ""})
	})
	if err != nil || first != second {
		t.Fatalf("provenance history not deterministic: err=%v", err)
	}
}
