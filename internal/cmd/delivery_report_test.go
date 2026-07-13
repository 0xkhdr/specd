package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestDeliveryReportStable(t *testing.T) {
	records := []core.DeploymentV1{{Schema: core.DeploymentSchemaV1, DeploymentID: "dep-b", Attempt: 1, ReleaseID: "rel-b", Environment: core.EnvironmentProduction, Status: core.StatusFailed, Adapter: "runtime", AdapterTrustSource: core.AdapterTrustSignedRuntime},
		{Schema: core.DeploymentSchemaV1, DeploymentID: "dep-a", Attempt: 1, ReleaseID: "rel-a", Environment: core.EnvironmentStaging, Status: core.StatusHealthy, Adapter: "ci", AdapterTrustSource: core.AdapterTrustAttestedCI}}
	first := renderDeliveryReport(records)
	second := renderDeliveryReport(records)
	if first != second {
		t.Fatal("delivery report not byte-stable")
	}
	if !strings.Contains(first, "source=signed_runtime") || !strings.Contains(first, "adapter=runtime") {
		t.Fatalf("source not labeled separately: %s", first)
	}
	if strings.Index(first, "dep-a") > strings.Index(first, "dep-b") {
		t.Fatalf("report not sorted: %s", first)
	}
	metrics := core.RenderPrometheus(core.PrometheusMetrics{Slug: "demo", TasksByStatus: map[string]int{}, DeliveryBySource: map[string]int{"signed_runtime": 1, "attested_ci": 1}})
	if !strings.Contains(metrics, `source="signed_runtime"`) || strings.Index(metrics, `source="attested_ci"`) > strings.Index(metrics, `source="signed_runtime"`) {
		t.Fatalf("delivery source metrics unstable: %s", metrics)
	}
}
