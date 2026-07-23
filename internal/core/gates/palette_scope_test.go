package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestPaletteAndProducerScopeRefusals pins spec R4.1/R4.3: a row declaring a CLI
// handler under internal/cmd/ must also declare the command palette (R4.1) and
// the generated-docs source (R4.3), or the tasks gate refuses naming the row and
// the missing file. A row declaring all three passes.
func TestPaletteAndProducerScopeRefusals(t *testing.T) {
	arm := string(core.StatusTasks)

	missingPalette := []core.TaskRow{{ID: "T1", Files: "internal/cmd/foo.go,tools/gendocs/main.go"}}
	f := paletteScope(CheckCtx{ApproveTarget: arm, Tasks: missingPalette})
	if !HasErrors(f) || !strings.Contains(f[0].Message, "T1") || !strings.Contains(f[0].Message, commandPaletteFile) {
		t.Fatalf("R4.1: missing palette not refused with row+file: %+v", f)
	}

	missingDocs := []core.TaskRow{{ID: "T2", Files: "internal/cmd/foo.go,internal/core/commands.go"}}
	f = paletteScope(CheckCtx{ApproveTarget: arm, Tasks: missingDocs})
	if !HasErrors(f) || !strings.Contains(f[0].Message, "T2") || !strings.Contains(f[0].Message, gendocsSourceFile) {
		t.Fatalf("R4.3: missing gendocs source not refused with row+file: %+v", f)
	}

	clean := []core.TaskRow{
		{ID: "T3", Files: "internal/cmd/foo.go,internal/core/commands.go,tools/gendocs/main.go"},
		{ID: "T4", Files: "internal/core/gates/x.go"}, // not a handler → no obligation
	}
	if f := paletteScope(CheckCtx{ApproveTarget: arm, Tasks: clean}); len(f) != 0 {
		t.Fatalf("clean command-surface plan refused: %+v", f)
	}

	// Armed ONLY at the tasks approval — plain check / specComplete (target "")
	// must not fire, so a handler row never blocks completion of a historical
	// spec.
	if paletteScopeArmed("") {
		t.Fatal("palette-scope must not arm at plain check (target \"\")")
	}
	if !paletteScopeArmed(arm) {
		t.Fatal("palette-scope must arm at the tasks approval target")
	}
	if f := paletteScope(CheckCtx{ApproveTarget: "", Tasks: missingPalette}); len(f) != 0 {
		t.Fatalf("palette-scope fired at target \"\": %+v", f)
	}
}

// TestPaletteAndProducerFlagLint pins spec R4.2: a handler-recognized flag
// absent from the palette is reported by the deterministic lint; documented
// flags yield nothing.
func TestPaletteAndProducerFlagLint(t *testing.T) {
	documented := core.PaletteFlagNames()
	if !documented["json"] {
		t.Fatal("palette is expected to document --json")
	}
	got := UndocumentedFlags([]string{"json", "frobnicate", "task"}, documented)
	if len(got) != 1 || got[0] != "frobnicate" {
		t.Fatalf("undocumented flag not caught: %v", got)
	}
	if got := UndocumentedFlags([]string{"json", "task"}, documented); len(got) != 0 {
		t.Fatalf("documented flags flagged: %v", got)
	}
}

// TestPaletteAndProducerEvidenceProducer pins spec R3.1/R3.2: a non-test
// evidence class warns at the tasks gate that a plain verify cannot satisfy it,
// naming the `specd eval import` producer and the exact import command; a
// test-only declaration warns nothing.
func TestPaletteAndProducerEvidenceProducer(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Evidence: "output_eval/rubric-demo, review/design"}}
	f := qualityDeclaration(CheckCtx{Slug: "demo", ApproveTarget: string(core.StatusTasks), Tasks: tasks})
	if HasErrors(f) {
		t.Fatalf("valid non-test declaration must warn, not error: %+v", f)
	}
	if len(f) != 2 {
		t.Fatalf("want a warn per non-test class, got %+v", f)
	}
	for _, want := range []string{"T1", "specd eval import demo <file> --task T1 --check rubric-demo", "output_eval/rubric-demo"} {
		if !strings.Contains(f[0].Message, want) {
			t.Fatalf("R3.1/R3.2 finding %q missing %q", f[0].Message, want)
		}
	}
	for _, finding := range f {
		if finding.Severity != Warn {
			t.Fatalf("evidence-producer finding not warning severity: %+v", finding)
		}
	}

	testOnly := []core.TaskRow{{ID: "T2", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Evidence: "test/unit"}}
	if f := qualityDeclaration(CheckCtx{Slug: "demo", ApproveTarget: string(core.StatusTasks), Tasks: testOnly}); len(f) != 0 {
		t.Fatalf("test-class declaration must not warn: %+v", f)
	}
}
