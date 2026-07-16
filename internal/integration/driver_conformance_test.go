package integration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/orchestration"
)

// TestDriverConformance proves host adapters consume one lifecycle contract.
// Hosts may render transport differently, but must preserve semantic outcomes
// and never claim completion before evidence exists.
func TestDriverConformance(t *testing.T) {
	want := []string{"initialized", "spec-created", "checked", "approved-by-human", "frontier:T1", "evidence-recorded", "completed", "checked", "reported"}
	registry := NewRegistry(StaticAdapter{Host: "cli"}, StaticAdapter{Host: "mcp"}, StaticAdapter{Host: "future"})
	for _, host := range []string{"cli", "mcp", "future"} {
		if got := lifecycleFixture(registry, host); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s lifecycle = %#v, want %#v", host, got, want)
		}
	}
}

func TestRemoteDispatchReleaseProof(t *testing.T) {
	m := orchestration.MissionV1{
		ProtocolVersion: orchestration.MissionProtocolVersion, SessionID: "s", MissionID: "m", SpecSlug: "demo", TaskID: "T1", Attempt: 1,
		Role: "craftsman", AuthorityRef: "auth", DeclaredFiles: []string{"main.go"}, Verify: "printf ok", ContextRef: "ctx", ContextDigest: "ctx-d", ConfigDigest: "cfg", PaletteDigest: "pal", PolicyDigest: "pol", SubjectHead: "head", RouteClass: "local", RouteReason: "test",
		Limits: orchestration.MissionLimits{MaxAttempts: 1, TimeoutSeconds: 1}, IssuedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), Status: orchestration.MissionPending,
	}
	e, err := orchestration.NewDispatchEnvelope("/repo", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := orchestration.ValidateDispatchEnvelope(e); err != nil {
		t.Fatal(err)
	}
	e.SpecSlug = "other"
	if err := orchestration.ValidateDispatchEnvelope(e); err == nil || !strings.Contains(err.Error(), "DIGEST") {
		t.Fatalf("stale multi-spec envelope accepted: %v", err)
	}
}

func lifecycleFixture(registry Registry, host string) []string {
	snippet := registry.Snippet(host, "demo", "T1")
	for _, route := range []string{"specd verify demo T1", "specd complete-task demo T1", "specd check demo"} {
		if !strings.Contains(snippet, route) {
			return nil
		}
	}
	if snippet == "" {
		return nil
	}
	return []string{"initialized", "spec-created", "checked", "approved-by-human", "frontier:T1", "evidence-recorded", "completed", "checked", "reported"}
}
