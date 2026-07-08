package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestApproveDesignStub (P4.2/R4.2): the design gate refuses to approve a
// design that is the unedited scaffold stub or has empty sections, and accepts
// one with filled sections.
func TestApproveDesignStub(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "new", []string{"demo"}, nil); err != nil {
		t.Fatalf("new: %v", err)
	}
	dir := filepath.Join(root, ".specd", "specs", "demo")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Real requirements so the requirements gate passes.
	write("requirements.md", "# Requirements — demo\n\n- **R1** When x runs, the system shall y.\n")
	if err := Run(root, "approve", []string{"demo", "requirements"}, nil); err != nil {
		t.Fatalf("approve requirements: %v", err)
	}

	// Design is still the scaffold stub → refused.
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err == nil {
		t.Fatal("approve design accepted an unedited stub")
	}
	// Empty sections → still refused.
	write("design.md", "# Design — demo\n\n## Modules\n\n## Invariants\n")
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err == nil {
		t.Fatal("approve design accepted empty sections")
	}
	// Filled sections → accepted.
	write("design.md", "# Design — demo\n\n## Modules\nThe module runs gates.\n")
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err != nil {
		t.Fatalf("approve design (filled): %v", err)
	}
}
