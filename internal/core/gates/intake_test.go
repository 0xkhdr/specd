package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestIntakeReadiness(t *testing.T) {
	ctx := CheckCtx{Provenance: &core.ProvenanceV1{
		SchemaVersion:  core.ProvenanceSchemaV1,
		SourceType:     core.SourceIncident,
		SourceRef:      "INC-42",
		RequiredFields: []string{"source_ref", "systems", "affected_specs", "severity", "risk", "owner", "prior_links"},
	}}
	findings := intakeReadiness(ctx)
	if len(findings) != 6 {
		t.Fatalf("findings=%v, want six missing fields", findings)
	}
	for _, field := range []string{"systems", "affected_specs", "severity", "risk", "owner", "prior_links"} {
		if !containsFinding(findings, field) {
			t.Errorf("missing finding for %s: %v", field, findings)
		}
	}

	ctx.Provenance.Systems = []string{"api"}
	ctx.Provenance.AffectedSpecs = []string{"payments"}
	ctx.Provenance.Severity = "high"
	ctx.Provenance.Risk = "customer-impact"
	ctx.Provenance.Owner = "sre"
	ctx.Provenance.PriorLinks = []core.ProvenanceLink{{To: "payments"}}
	if got := intakeReadiness(ctx); len(got) != 0 {
		t.Fatalf("complete intake findings=%v", got)
	}
}

func TestIntakeUnknownSentinel(t *testing.T) {
	configured := &core.ProvenanceV1{
		SourceType: core.SourceDrift, SourceRef: "unknown", Systems: []string{"unknown"},
		RequiredFields: []string{"source_ref", "systems"},
	}
	first := intakeReadiness(CheckCtx{Provenance: configured})
	second := intakeReadiness(CheckCtx{Provenance: configured})
	if len(first) != 2 || len(second) != 2 || first[0].Message != second[0].Message || first[1].Message != second[1].Message {
		t.Fatalf("unknown sentinel must fail deterministically: first=%v second=%v", first, second)
	}
	if got := intakeReadiness(CheckCtx{}); len(got) != 0 {
		t.Fatalf("unconfigured feature changed behavior: %v", got)
	}
	if got := intakeReadiness(CheckCtx{Provenance: &core.ProvenanceV1{SourceType: core.SourceFeature}}); len(got) != 0 {
		t.Fatalf("feature intake without policy changed behavior: %v", got)
	}
}

func containsFinding(findings []Finding, value string) bool {
	for _, finding := range findings {
		if strings.Contains(finding.Message, value) {
			return true
		}
	}
	return false
}
