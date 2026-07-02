package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardrailsAbsentPasses(t *testing.T) {
	root := t.TempDir()
	_, violations, warnings := EvaluateGuardrails(root, nil, true)
	if len(violations) != 0 || len(warnings) != 0 {
		t.Fatalf("absent guardrails = violations %v warnings %v", violations, warnings)
	}
}

func TestGuardrailsRejectsUnknownFields(t *testing.T) {
	root := writeGuardrailsRoot(t, `{"unknown": true}`)
	_, violations, _ := EvaluateGuardrails(root, nil, true)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	if !strings.Contains(violations[0].Message, "unknown field") {
		t.Fatalf("message = %q, want unknown field", violations[0].Message)
	}
}

func TestGuardrailsRejectsInvalidRegex(t *testing.T) {
	root := writeGuardrailsRoot(t, `{"forbiddenPatterns":[{"id":"bad","pattern":"["}]}`)
	_, violations, _ := EvaluateGuardrails(root, nil, true)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	if !strings.Contains(violations[0].Message, "forbiddenPatterns[0].pattern") {
		t.Fatalf("message = %q, want field context", violations[0].Message)
	}
}

func TestGuardrailsFindingsAreSortedByPathThenRule(t *testing.T) {
	root := writeGuardrailsRoot(t, `{
		"forbiddenPatterns":[
			{"id":"z","pattern":"bad"},
			{"id":"a","pattern":"bad"}
		],
		"forbiddenPaths":[{"id":"path","pattern":"\\.secret$"}]
	}`)
	writeFile(t, root, "b.go", "bad\n")
	writeFile(t, root, "a.go", "bad\n")
	writeFile(t, root, "z.secret", "ok\n")

	_, violations, _ := EvaluateGuardrails(root, nil, true)
	got := make([]string, 0, len(violations))
	for _, v := range violations {
		got = append(got, v.Location+" "+v.Message)
	}
	want := []string{
		"a.go a: forbidden pattern \"bad\"",
		"a.go z: forbidden pattern \"bad\"",
		"b.go a: forbidden pattern \"bad\"",
		"b.go z: forbidden pattern \"bad\"",
		"z.secret path: forbidden pattern \"\\\\.secret$\"",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestGuardrailsScansVerifyCommands(t *testing.T) {
	root := writeGuardrailsRoot(t, `{"forbiddenCommands":[{"id":"shell","pattern":"rm -rf"}]}`)
	doc := &ParsedTasks{Tasks: []ParsedTask{{
		ID:   "T1",
		Line: 7,
		Meta: map[string]string{"verify": "rm -rf /tmp/demo"},
	}}}
	_, violations, _ := EvaluateGuardrails(root, doc, true)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	if violations[0].Location != "tasks.md:7" || !strings.Contains(violations[0].Message, "shell:") {
		t.Fatalf("violation = %+v", violations[0])
	}
}

func writeGuardrailsRoot(t *testing.T, data string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".specd/guardrails.json", data)
	return root
}

func writeFile(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
