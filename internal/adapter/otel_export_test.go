package adapter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestOTelTraceExportPreservesCorrelationAndExcludesRawData(t *testing.T) {
	events := []orchestration.ObservableEvent{{RunID: "run-1", EventID: "event-1", Seq: 1, Tool: "specd.check", ArgClass: "metadata", ResultClass: "pass", Paths: []string{"internal/core/state.go"}, Time: "2026-07-13T00:00:00Z", Actor: "validator", Correlation: "corr-1"}}
	records, err := ExportOTel(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].TraceID != "run-1" || records[0].SpanID != "event-1" || records[0].Attributes["specd.correlation_id"] != "corr-1" {
		t.Fatalf("correlation lost: %+v", records)
	}
	raw, _ := json.Marshal(records)
	for _, forbidden := range []string{"source_content", "prompt", "secret", "internal/core/state.go"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("export leaked %q: %s", forbidden, raw)
		}
	}
}

func TestOTelTraceExportRejectsInvalidTrace(t *testing.T) {
	_, err := ExportOTel([]orchestration.ObservableEvent{{RunID: "run-1", EventID: "event-1", Seq: 1}})
	if err == nil {
		t.Fatal("invalid trace exported")
	}
}
