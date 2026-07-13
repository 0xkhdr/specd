package verify

import (
	"context"
	"strings"
	"testing"
)

func TestAdapterProductionCapabilities(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "ci"} {
		a := SandboxAdapterV1{SchemaVersion: SandboxAdapterSchemaV1, Name: platform + "-sandbox", Platform: platform, Capabilities: append([]string(nil), RequiredSandboxCapabilities...)}
		if err := a.Validate(true); err != nil {
			t.Fatalf("%s adapter rejected: %v", platform, err)
		}
	}
	a := SandboxAdapterV1{SchemaVersion: SandboxAdapterSchemaV1, Name: "weak", Platform: "linux", Capabilities: []string{CapabilityNetworkIsolation}}
	if err := a.Validate(true); err == nil || !strings.Contains(err.Error(), CapabilityCredentialIsolation) {
		t.Fatalf("missing capability error = %v", err)
	}
}

func TestAdapterUnknownAndDuplicateCapabilitiesFailClosed(t *testing.T) {
	base := SandboxAdapterV1{SchemaVersion: SandboxAdapterSchemaV1, Name: "sandbox", Platform: "ci", Capabilities: append([]string(nil), RequiredSandboxCapabilities...)}
	for name, mutate := range map[string]func(*SandboxAdapterV1){
		"schema":    func(a *SandboxAdapterV1) { a.SchemaVersion = "sandbox/v2" },
		"platform":  func(a *SandboxAdapterV1) { a.Platform = "windows" },
		"unknown":   func(a *SandboxAdapterV1) { a.Capabilities = append(a.Capabilities, "host.root") },
		"duplicate": func(a *SandboxAdapterV1) { a.Capabilities = append(a.Capabilities, a.Capabilities[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			a := base
			a.Capabilities = append([]string(nil), base.Capabilities...)
			mutate(&a)
			if err := a.Validate(true); err == nil {
				t.Fatal("invalid adapter accepted")
			}
		})
	}
}

func TestAdapterRefusedBeforeShell(t *testing.T) {
	dir := t.TempDir()
	_, err := Run(context.Background(), Options{Command: "touch ran", Dir: dir, RequireSandbox: true, Adapter: &SandboxAdapterV1{
		SchemaVersion: SandboxAdapterSchemaV1, Name: "weak", Platform: "ci", Binary: "/bin/sh", Capabilities: []string{CapabilityNetworkIsolation},
	}})
	if err == nil || !strings.Contains(err.Error(), "missing required capability") {
		t.Fatalf("Run error = %v", err)
	}
}
