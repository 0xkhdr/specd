package gates

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestCoreGates(t *testing.T) {
	tasks := []core.TaskRow{
		{ID: "T1", Role: "builder", Files: "a.go", Verify: "go test ./..."},
		{ID: "T2", Role: "builder", Files: "b.go", DependsOn: []string{"T1"}, Verify: "go test ./..."},
	}
	ctx := CheckCtx{
		Tasks:    tasks,
		Status:   map[string]core.TaskRunStatus{"T1": core.TaskComplete},
		Evidence: map[string]core.EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "abc"}},
	}
	if findings := CoreRegistry().Run(ctx); HasErrors(findings) {
		t.Fatalf("valid tasks produced errors: %#v", findings)
	}

	ctx.Evidence = map[string]core.EvidenceRecord{}
	findings := CoreRegistry().Run(ctx)
	if !HasErrors(findings) {
		t.Fatalf("missing evidence should fail")
	}
}
