package gates

import "testing"

func TestRegistryOrder(t *testing.T) {
	names := CoreRegistry().Names()
	want := []string{"task-ids", "dependencies", "dag", "roles", "files", "verify", "evidence", "context-budget", "ears", "approval", "sync", "design"}
	if len(names) != len(want) {
		t.Fatalf("len = %d, want %d", len(names), len(want))
	}
	for i := range names {
		if names[i] != want[i] {
			t.Fatalf("name[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}
