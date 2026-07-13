package integration

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core/verify"
)

func TestSandboxConformanceEquivalentPolicyOutcome(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "ci"} {
		a := verify.SandboxAdapterV1{SchemaVersion: verify.SandboxAdapterSchemaV1, Name: platform, Platform: platform, Capabilities: append([]string(nil), verify.RequiredSandboxCapabilities...)}
		if err := a.Validate(true); err != nil {
			t.Errorf("%s outcome = %v, want accepted", platform, err)
		}
		a.Capabilities = a.Capabilities[:len(a.Capabilities)-1]
		if err := a.Validate(true); err == nil {
			t.Errorf("%s incomplete adapter accepted", platform)
		}
	}
}
