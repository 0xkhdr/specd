package gates

import "testing"

func TestRegistryOrder(t *testing.T) {
	names := CoreRegistry().Names()
	want := []string{"task-ids", "dependencies", "dag", "roles", "files", "verify", "evidence", "context-budget", "ears", "approval", "sync", "design", "criteria", "review", "task-trace", "coverage", "evidence-policy", "intake", "governance", "memory-lint", "quality-declaration"}
	if len(names) != len(want) {
		t.Fatalf("len = %d, want %d", len(names), len(want))
	}
	for i := range names {
		if names[i] != want[i] {
			t.Fatalf("name[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

type namedGate string

func (g namedGate) Name() string           { return string(g) }
func (g namedGate) Run(CheckCtx) []Finding { return nil }

func TestRegistryProductionRequiredGate(t *testing.T) {
	names := CoreRegistryWith(namedGate("security")).Names()
	if names[len(names)-1] != "security" {
		t.Fatalf("names=%v", names)
	}
}
