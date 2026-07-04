package integration

import "testing"

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
