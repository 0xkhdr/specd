package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestTaskTraceEmptyRequirementSet pins R3.1: an unparseable requirements.md
// with tasks that cite R-ids is reported against requirements.md, not blamed on
// tasks.md — and a genuinely-unknown reference against a parseable requirements
// doc keeps the original unknown-reference message.
func TestTaskTraceEmptyRequirementSet(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1", Refs: []string{"R1.1"}}}

	t.Run("empty_set_names_requirements_md", func(t *testing.T) {
		// Non-empty but unparseable requirements.md (old template shape).
		doc := "## Requirement R1 — x\n\n- **R1.1** When a, the system shall b.\n"
		findings := taskTrace(CheckCtx{Tasks: tasks, RequirementsDoc: doc})
		if len(findings) != 1 {
			t.Fatalf("findings = %+v, want exactly one", findings)
		}
		if !strings.Contains(findings[0].Message, "requirements.md") {
			t.Fatalf("finding must name requirements.md, got %q", findings[0].Message)
		}
		if strings.Contains(findings[0].Message, "references unknown requirement") {
			t.Fatalf("must not blame tasks.md, got %q", findings[0].Message)
		}
	})

	t.Run("parseable_set_keeps_unknown_reference_message", func(t *testing.T) {
		doc := "## R1 — x\n\n- R1.1: When a, the system shall b.\n"
		bad := []core.TaskRow{{ID: "T1", Refs: []string{"R9.9"}}}
		findings := taskTrace(CheckCtx{Tasks: bad, RequirementsDoc: doc})
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "references unknown requirement R9.9") {
			t.Fatalf("findings = %+v, want unknown-reference message", findings)
		}
	})
}
