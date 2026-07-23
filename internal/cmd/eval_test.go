package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestEvalImportAndStatusAreLocal(t *testing.T) {
	root := t.TempDir()
	env := core.EvidenceEnvelopeV1{SchemaVersion: core.EvalSchemaVersion, EvidenceID: "e1", EvidenceClass: core.EvidenceOutputEval, SpecSlug: "demo", TaskID: "T1", RunID: "r1", Attempt: 1, SubjectRevision: "abc", Producer: "ci", ProducerVersion: "1", ConfigDigest: "cfg", CheckID: "rubric-v1", Verdict: core.EvalPass, CreatedAt: "2026-01-01T00:00:00Z", Actor: "ci", ArtifactRef: "evals/e1.json", ArtifactDigest: "sha", DatasetDigest: "ds", RubricDigest: "rb", OutputDigest: "out"}
	line, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(root, "adapter.jsonl")
	if err := os.WriteFile(in, append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "eval", []string{"import", "demo", "adapter.jsonl"}, map[string]string{"task": "T1"}); err != nil {
		t.Fatalf("eval import: %v", err)
	}
	stored, err := core.LoadEvals(core.EvalStorePath(root, "demo"))
	if err != nil || len(stored) != 1 || stored[0].EvidenceID != "e1" {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	out, err := captureStdout(t, func() error {
		return Run(root, "eval", []string{"status", "demo"}, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("eval status: %v", err)
	}
	var report evalStatusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Count != 1 || report.Records[0].EvidenceID != "e1" {
		t.Fatalf("status=%+v", report)
	}
}

func TestEvalImportRejectsOutsideRootAndBadAdapter(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "eval", []string{"import", "demo", "../outside.jsonl"}, map[string]string{"task": "T1"}); err == nil || !strings.Contains(err.Error(), "path") {
		t.Fatalf("path escape accepted: %v", err)
	}
	path := filepath.Join(root, "bad.jsonl")
	if err := os.WriteFile(path, []byte("{bad}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "eval", []string{"import", "demo", "bad.jsonl"}, map[string]string{"task": "T1"}); err == nil || !strings.Contains(err.Error(), "EVAL_IMPORT_MALFORMED") {
		t.Fatalf("bad adapter accepted: %v", err)
	}
}

func TestEvalAbsolutePathTypedRefusal(t *testing.T) {
	t.Run("import-refusal", func(t *testing.T) {
		root := t.TempDir()
		artifact := filepath.Join(root, "adapter.jsonl")
		const contents = "external evidence"
		if err := os.WriteFile(artifact, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}

		err := Run(root, "eval", []string{"import", "demo", artifact}, map[string]string{"task": "T1", "check": "rubric-v1"})
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "ARTIFACT_PATH_ABSOLUTE" || !errors.Is(err, ErrUsage) {
			t.Fatalf("absolute path refusal = %#v, %v", refusal, err)
		}
		if refusal.Observed != artifact || refusal.Expected != "workspace-relative artifact path" {
			t.Fatalf("absolute path context = %#v", refusal)
		}
		if !strings.Contains(refusal.Detail, artifact) || !strings.Contains(refusal.Detail, "workspace-relative") {
			t.Fatalf("absolute path detail = %q", refusal.Detail)
		}
		const recovery = "specd eval import demo <workspace-relative-file> --task T1 --check rubric-v1"
		if refusal.RecoveryCommand != recovery || strings.Contains(refusal.RecoveryCommand, artifact) {
			t.Fatalf("absolute path recovery = %q, want %q", refusal.RecoveryCommand, recovery)
		}
		if _, statErr := os.Stat(core.EvalStorePath(root, "demo")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("absolute path refusal mutated eval store: %v", statErr)
		}
		raw, readErr := os.ReadFile(artifact)
		if readErr != nil || string(raw) != contents {
			t.Fatalf("absolute path refusal mutated artifact: contents=%q err=%v", raw, readErr)
		}
	})

	t.Run("completion-recovery", func(t *testing.T) {
		contract := core.QualityContract{
			TaskID: "T1",
			Required: []core.EvidenceRequirement{{
				EvidenceClass: core.EvidenceOutputEval,
				CheckID:       "rubric-v1",
			}},
		}
		err := qualityEvidenceRefusal("demo", "T1", contract, nil, "head")
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "EVIDENCE_MISSING" {
			t.Fatalf("completion refusal = %#v, %v", refusal, err)
		}
		const recovery = "specd eval import demo <workspace-relative-file> --task T1 --check rubric-v1"
		if refusal.RecoveryCommand != recovery {
			t.Fatalf("completion recovery = %q, want %q", refusal.RecoveryCommand, recovery)
		}
	})
}

func TestEvalCommandRegistered(t *testing.T) {
	if _, ok := Registry["eval"]; !ok {
		t.Fatal("eval command missing from registry")
	}
	if _, ok := core.CommandByName("eval"); !ok {
		t.Fatal("eval command missing from palette")
	}
}
