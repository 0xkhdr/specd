package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testTemplate = `# Test Template

This is a test template for AGENTS.md.

## Section 1

Some content here.

## Section 2

More content.`

func TestMergeAgentsMD_NewFile(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "AGENTS.md")

	err := MergeAgentsMD(path, testTemplate, false)
	if err != nil {
		t.Fatalf("MergeAgentsMD failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	contentStr := string(content)

	if !strings.Contains(contentStr, markerBegin()) {
		t.Error("missing BEGIN marker")
	}
	if !strings.Contains(contentStr, markerEnd()) {
		t.Error("missing END marker")
	}
	if !strings.Contains(contentStr, "Test Template") {
		t.Error("template content not found")
	}
}

func TestMergeAgentsMD_Idempotent(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "AGENTS.md")

	// First merge
	err := MergeAgentsMD(path, testTemplate, false)
	if err != nil {
		t.Fatalf("first merge failed: %v", err)
	}
	content1, _ := os.ReadFile(path)

	// Second merge (same template)
	err = MergeAgentsMD(path, testTemplate, false)
	if err != nil {
		t.Fatalf("second merge failed: %v", err)
	}
	content2, _ := os.ReadFile(path)

	if string(content1) != string(content2) {
		t.Error("merge not idempotent: file changed on second run")
	}
}

func TestMergeAgentsMD_PreserveCustom(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "AGENTS.md")

	// Initial merge
	MergeAgentsMD(path, testTemplate, false)

	// Add custom content after markers
	customPost := "\n## Custom Section\n\nThis is custom."
	existing, _ := os.ReadFile(path)
	os.WriteFile(path, append(existing, []byte(customPost)...), 0o644)

	// Merge again
	err := MergeAgentsMD(path, testTemplate, false)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	contentStr := string(content)

	if !strings.Contains(contentStr, "Custom Section") {
		t.Error("custom content after markers not preserved")
	}
	if !strings.Contains(contentStr, "Test Template") {
		t.Error("template content lost")
	}
}

func TestMergeAgentsMD_PreservePreamble(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "AGENTS.md")

	// Initial merge
	MergeAgentsMD(path, testTemplate, false)

	// Add custom content before markers
	preamble := "# Project Notes\n\nSome preamble.\n\n"
	existing, _ := os.ReadFile(path)
	newContent := preamble + string(existing)
	os.WriteFile(path, []byte(newContent), 0o644)

	// Merge again
	err := MergeAgentsMD(path, testTemplate, false)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	contentStr := string(content)

	if !strings.HasPrefix(contentStr, "# Project Notes") {
		t.Error("preamble not preserved")
	}
	if !strings.Contains(contentStr, "Test Template") {
		t.Error("template content lost")
	}
}

func TestMergeAgentsMD_Force(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "AGENTS.md")

	// Initial merge with custom content
	MergeAgentsMD(path, testTemplate, false)
	preamble := "# Preamble\n\n"
	postamble := "\n## Custom\n\nContent"
	existing, _ := os.ReadFile(path)
	newContent := preamble + string(existing) + postamble
	os.WriteFile(path, []byte(newContent), 0o644)

	// Force merge (should reset to default)
	err := MergeAgentsMD(path, testTemplate, true)
	if err != nil {
		t.Fatalf("force merge failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	contentStr := string(content)

	if strings.Contains(contentStr, "# Preamble") {
		t.Error("preamble not removed by force")
	}
	if strings.Contains(contentStr, "## Custom") {
		t.Error("custom postamble not removed by force")
	}
	if !strings.HasPrefix(contentStr, markerBegin()) {
		t.Error("file should start with BEGIN marker after force")
	}
	if !strings.HasSuffix(strings.TrimRight(contentStr, "\n"), markerEnd()) {
		t.Error("file should end with END marker after force")
	}
}

func TestMergeAgentsMD_NoMarkersAppends(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "AGENTS.md")

	// Write file without markers
	oldContent := "# Old Content\n\nSome stuff."
	os.WriteFile(path, []byte(oldContent), 0o644)

	// Merge (should append with markers)
	err := MergeAgentsMD(path, testTemplate, false)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	contentStr := string(content)

	if !strings.Contains(contentStr, "# Old Content") {
		t.Error("old content lost")
	}
	if !strings.Contains(contentStr, markerBegin()) {
		t.Error("BEGIN marker not added")
	}
	if !strings.Contains(contentStr, testTemplate) {
		t.Error("template not added")
	}
}
