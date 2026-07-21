package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

// authorDemoSpec replaces the scaffold stubs with real requirements + design so
// the W4 EARS and design-stub gates pass. Sections carry content (design gate
// rejects empty sections) and requirements are EARS-shaped (no shape warnings).
func authorDemoSpec(t *testing.T, root, slug string) {
	t.Helper()
	dir := filepath.Join(root, ".specd", "specs", slug)
	reqs := "# Requirements — " + slug + "\n\n- **R1** When a user runs check, the system shall validate the spec.\n"
	design := "# Design — " + slug + "\n\n## Modules\nThe check module runs gates.\n\n## On-disk contracts\nstate.json holds status.\n\n## Invariants\nOutput is deterministic.\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.md"), []byte(reqs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "design.md"), []byte(design), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNextGatedOnApproval(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "new", []string{"demo"}, nil); err != nil {
		t.Fatalf("new: %v", err)
	}
	authorDemoSpec(t, root, "demo")
	writeTasks(t, root, "demo", "| T1 | scout | requirements.md | - | printf ok | approval-gated fixture |")

	// In the requirements (perceive) phase the coarse phase gate fails closed:
	// next/verify are execution verbs with no approved DAG to act on, so they
	// exit 2 before the handler runs (spec 03 R2).
	if err := Run(root, "next", []string{"demo"}, map[string]string{"json": "1"}); err == nil {
		t.Fatalf("next in perceive phase succeeded (want phase-gate rejection)")
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err == nil {
		t.Fatalf("verify before approval succeeded")
	}

	// First approval advances exactly requirements→design and records approval
	// of requirements. Execution stays blocked until design is approved.
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve next: %v", err)
	}
	out, err := captureStdout(t, func() error { return Run(root, "next", []string{"demo"}, map[string]string{"json": "1"}) })
	if err != nil {
		t.Fatalf("next after requirements approval: %v", err)
	}
	if strings.Contains(out, `"id": "T1"`) {
		t.Fatalf("next exposed task before design approval: %s", out)
	}
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve design: %v", err)
	}

	out, err = captureStdout(t, func() error {
		return Run(root, "next", []string{"demo"}, map[string]string{"json": "1"})
	})
	if err != nil {
		t.Fatalf("next after approval: %v", err)
	}
	if !strings.Contains(out, `"id": "T1"`) {
		t.Fatalf("next after approval = %s", out)
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify after approval: %v", err)
	}
}

func TestReadinessApprovalParity(t *testing.T) {
	t.Run("green_same_revision_plan", func(t *testing.T) {
		root := newDemoSpec(t)
		out, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "true"})
		})
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		var envelope core.ReadinessEnvelope
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatalf("readiness envelope: %v\n%s", err, out)
		}
		if envelope.SchemaVersion != core.ReadinessSchemaVersion || !envelope.Plan.ReadinessChecked {
			t.Fatalf("unversioned or unchecked readiness: %+v", envelope)
		}
		if envelope.Plan.Target != core.StatusDesign || envelope.Plan.ConfigDigest == "" || len(envelope.Plan.ArmedGates) == 0 || len(envelope.Plan.ArtifactDigests) != 3 {
			t.Fatalf("incomplete readiness identity: %+v", envelope.Plan)
		}

		approved, err := captureStdout(t, func() error { return Run(root, "approve", []string{"demo"}, nil) })
		if err != nil {
			t.Fatalf("approve: %v", err)
		}
		for _, want := range []string{envelope.Plan.PlanDigest, "revision " + fmt.Sprint(envelope.Plan.StateRevision)} {
			if !strings.Contains(approved, want) {
				t.Fatalf("approval did not consume readiness identity %q: %s", want, approved)
			}
		}
	})

	t.Run("blockers_and_legacy_shape", func(t *testing.T) {
		root := newDemoSpec(t)
		writeTasks(t, root, "demo", "| T1 | scout | spec.md | T9 | true | ok |")
		out, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "true"})
		})
		if err == nil {
			t.Fatal("blocking readiness returned success")
		}
		var envelope core.ReadinessEnvelope
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatalf("readiness envelope: %v\n%s", err, out)
		}
		if len(envelope.Plan.Blockers) == 0 {
			t.Fatalf("blocking finding missing from plan: %+v", envelope.Plan)
		}
		before, _ := core.LoadState(core.StatePath(root, "demo"))
		approveErr := Run(root, "approve", []string{"demo"}, nil)
		if approveErr == nil || !strings.Contains(approveErr.Error(), envelope.Plan.PlanDigest) {
			t.Fatalf("approval did not consume blocking plan %s: %v", envelope.Plan.PlanDigest, approveErr)
		}
		after, _ := core.LoadState(core.StatePath(root, "demo"))
		if after.Revision != before.Revision || after.Status != before.Status {
			t.Fatalf("blocked approval mutated state: %+v -> %+v", before, after)
		}

		legacy, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "legacy"})
		})
		if err != nil {
			t.Fatalf("legacy check: %v", err)
		}
		var findings []gates.Finding
		if err := json.Unmarshal([]byte(legacy), &findings); err != nil || len(findings) != len(envelope.Findings) {
			t.Fatalf("legacy findings: err=%v findings=%+v envelope=%+v", err, findings, envelope.Findings)
		}
	})

	t.Run("warnings_do_not_block", func(t *testing.T) {
		root := newDemoSpec(t)
		requirements := filepath.Join(core.SpecdDir(root), "specs", "demo", "requirements.md")
		if err := os.WriteFile(requirements, []byte("# Requirements — demo\n\n- R1 describes validation.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "true"})
		})
		if err != nil {
			t.Fatalf("warning-only check: %v", err)
		}
		var envelope core.ReadinessEnvelope
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Plan.Blockers) != 0 || len(envelope.Plan.Warnings) == 0 {
			t.Fatalf("warning classification = %+v", envelope.Plan)
		}
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("warning blocked approval: %v", err)
		}
	})

	t.Run("revision_drift_replans", func(t *testing.T) {
		root := newDemoSpec(t)
		state, _ := core.LoadState(core.StatePath(root, "demo"))
		old, err := buildReadiness(root, "demo", state)
		if err != nil {
			t.Fatal(err)
		}
		if err := Run(root, "request-decision", []string{"demo"}, map[string]string{"text": "record drift"}); err != nil {
			t.Fatal(err)
		}
		approved, err := captureStdout(t, func() error { return Run(root, "approve", []string{"demo"}, nil) })
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(approved, old.Envelope.Plan.PlanDigest) || !strings.Contains(approved, "revision 1") {
			t.Fatalf("approval consumed stale revision-0 plan: %s", approved)
		}
	})
}
