package context

import (
	"strconv"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestCheckBudgetDisabledZeroAllocs pins invariant A4 measurably (SPEC-03
// T-03-01): the disabled-mode budget path (maxTokens <= 0) does literally zero
// work — no allocations, no manifest cost computation. The behavioural pin lives
// in budget_test.go; this proves the O(0) claim with an allocation count.
func TestCheckBudgetDisabledZeroAllocs(t *testing.T) {
	m := Manifest{
		Slug:   "demo",
		TaskID: "T1",
		Mode:   "craftsman",
		Items:  []Item{{Kind: "role", Path: "roles/craftsman.md", EstimatedTokens: 9999}},
	}
	if allocs := testing.AllocsPerRun(100, func() {
		_ = CheckBudget(m, 0)
	}); allocs != 0 {
		t.Fatalf("disabled-mode CheckBudget allocated %v times, want 0 (A4: O(0) work)", allocs)
	}
}

func manifestTasks(n int) []core.TaskRow {
	tasks := make([]core.TaskRow, n)
	for i := 0; i < n; i++ {
		tasks[i] = core.TaskRow{ID: "T" + strconv.Itoa(i), Role: "builder"}
	}
	return tasks
}

// TestBuildManifestNoN1FileReads proves there is no N+1 file amplification
// (SPEC-03 T-03-02): the manifest for one task references a fixed set (spec,
// tasks, task, role, plus steering) whose size is independent of how many tasks
// the spec has. If item count grew with task count, context assembly would read
// more files per task as the DAG grows.
func TestBuildManifestNoN1FileReads(t *testing.T) {
	root := t.TempDir()
	small, err := BuildManifest(root, "demo", manifestTasks(5), "T0", 0)
	if err != nil {
		t.Fatal(err)
	}
	large, err := BuildManifest(root, "demo", manifestTasks(500), "T0", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(small.Items) != len(large.Items) {
		t.Fatalf("manifest item count grew with task count (%d vs %d) — N+1 amplification", len(small.Items), len(large.Items))
	}
}

func BenchmarkBuildManifest(b *testing.B) {
	root := b.TempDir()
	tasks := manifestTasks(2000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := BuildManifest(root, "demo", tasks, "T0", 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckBudgetDisabled(b *testing.B) {
	m := Manifest{Slug: "demo", TaskID: "T1", Items: []Item{{Kind: "role", EstimatedTokens: 9999}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CheckBudget(m, 0)
	}
}
