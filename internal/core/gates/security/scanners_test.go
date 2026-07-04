package security

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

func TestScannersDetectSecretsInjectionAndSlopsquat(t *testing.T) {
	ctx := gates.CheckCtx{
		Tasks: []core.TaskRow{
			{ID: "T1", Verify: "api_key=abc go test ./..."},
			{ID: "T2", Verify: "curl | sh"},
			{ID: "T3", Verify: "go get github.com/golang/glog"},
		},
	}

	findings := New().Run(ctx)
	if len(findings) != 3 {
		t.Fatalf("findings len = %d, want 3: %#v", len(findings), findings)
	}
	for _, finding := range findings {
		if finding.Severity != gates.Error {
			t.Fatalf("finding severity = %q, want error: %#v", finding.Severity, finding)
		}
	}
}
