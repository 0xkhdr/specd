package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	out, err := captureStdout(t, func() error {
		return Run(root, "next", []string{"demo"}, map[string]string{"json": "1"})
	})
	if err != nil {
		t.Fatalf("next before approval: %v", err)
	}
	if !strings.Contains(out, `"items": []`) || !strings.Contains(out, "missing approval") {
		t.Fatalf("next before approval = %s", out)
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err == nil {
		t.Fatalf("verify before approval succeeded")
	}

	if err := Run(root, "approve", []string{"demo", "requirements"}, nil); err != nil {
		t.Fatalf("approve requirements: %v", err)
	}
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err != nil {
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
