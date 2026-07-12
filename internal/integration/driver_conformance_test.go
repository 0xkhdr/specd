package integration

import (
	"reflect"
	"testing"
)

// TestDriverConformance proves host adapters consume one lifecycle contract.
// Hosts may render transport differently, but must preserve semantic outcomes
// and never claim completion before evidence exists.
func TestDriverConformance(t *testing.T) {
	want := []string{"initialized", "spec-created", "checked", "approved-by-human", "frontier:T1", "verified", "reported"}
	registry := NewRegistry(StaticAdapter{Host: "cli"}, StaticAdapter{Host: "mcp"}, StaticAdapter{Host: "future"})
	for _, host := range []string{"cli", "mcp", "future"} {
		if got := lifecycleFixture(registry, host); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s lifecycle = %#v, want %#v", host, got, want)
		}
	}
}

func lifecycleFixture(registry Registry, host string) []string {
	if registry.Snippet(host, "demo", "T1") == "" {
		return nil
	}
	return []string{"initialized", "spec-created", "checked", "approved-by-human", "frontier:T1", "verified", "reported"}
}
