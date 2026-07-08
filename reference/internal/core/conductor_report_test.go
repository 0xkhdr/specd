package core

import (
	"reflect"
	"testing"
)

// TestConductorRejectionReportClusters asserts rejection clustering is a pure,
// deterministic count over the ledger: exact-string reasons grouped, sorted by
// descending count then reason ascending, with non-reject events ignored.
func TestConductorRejectionReportClusters(t *testing.T) {
	events := []ConductorEvent{
		{Action: "accept", Reason: ""},
		{Action: "reject", Reason: "flaky test"},
		{Action: "reject", Reason: "wrong file"},
		{Action: "reject", Reason: "flaky test"},
		{Action: "step", Reason: "ignored"},
		{Action: "reject", Reason: "flaky test"},
		{Action: "reject", Reason: "wrong file"},
	}
	got := ConductorRejectionReport(events)
	want := []RejectionCluster{
		{Reason: "flaky test", Count: 3},
		{Reason: "wrong file", Count: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clusters = %+v, want %+v", got, want)
	}

	// Determinism: a second call over the same events yields the identical order.
	if !reflect.DeepEqual(ConductorRejectionReport(events), got) {
		t.Fatalf("clustering is not deterministic")
	}

	// Ties on count break by reason ascending.
	tie := ConductorRejectionReport([]ConductorEvent{
		{Action: "reject", Reason: "zebra"},
		{Action: "reject", Reason: "alpha"},
	})
	if tie[0].Reason != "alpha" || tie[1].Reason != "zebra" {
		t.Fatalf("tie order = %+v, want alpha before zebra", tie)
	}

	// No rejections → empty (non-nil) slice.
	if got := ConductorRejectionReport([]ConductorEvent{{Action: "accept"}}); len(got) != 0 {
		t.Fatalf("want no clusters, got %+v", got)
	}
}
