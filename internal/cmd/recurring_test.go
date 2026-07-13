package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestRecurringRecord(t *testing.T) {
	root := t.TempDir()
	flags := map[string]string{"check": "api-health", "head": strings.Repeat("a", 40), "release": "rel-1", "config": "prod-v1", "verdict": "pass", "observed-at": "2026-01-01T00:00:00Z"}
	if err := runRecurring(root, []string{"record", "demo"}, flags); err != nil {
		t.Fatal(err)
	}
	records, err := core.LoadRecurringResults(core.RecurringResultsPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].CheckID != "api-health" {
		t.Fatalf("records = %+v", records)
	}
}
