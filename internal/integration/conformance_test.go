package integration

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestRequestModeGuideConformance(t *testing.T) {
	general := NewRegistry().ModeSnippet("codex", core.RequestModeGeneral, "", "", core.AssuranceAdvisory)
	if strings.Contains(general, "`specd ") || !strings.Contains(general, "Request mode: general") {
		t.Fatalf("general adapter guide = %q", general)
	}
	managed := NewRegistry().ModeSnippet("codex", core.RequestModeManaged, "demo", "T1", core.AssuranceAdvisory)
	if !strings.Contains(managed, "Run `specd handshake bootstrap demo --json` first") || !strings.Contains(managed, "not enforced") {
		t.Fatalf("managed adapter guide = %q", managed)
	}
}

func TestAdapterConformance(t *testing.T) {
	adapter := StaticAdapter{Host: "codex"}
	first := adapter.Install()
	second := adapter.Install()
	if first != second {
		t.Fatalf("Install not idempotent: %+v != %+v", first, second)
	}
	if first.Owner != "specd" || first.Host != "codex" {
		t.Fatalf("Install ownership not recorded: %+v", first)
	}
	if got := NewRegistry(adapter).Snippet("codex", "demo", "T1"); got == "" {
		t.Fatalf("empty adapter snippet")
	}
}
