package core

import (
	"strings"
	"testing"
)

const sampleDoc = `# Tasks — My Feature

## Wave 1
- [ ] T1 — Build the API
  - why: Need the API endpoint
  - role: builder
  - files: api.go
  - contract: Returns JSON
  - acceptance: API responds 200
  - verify: go test ./...
  - depends: —

`

func TestParseRoundTrip(t *testing.T) {
	doc, err := ParseTasks(sampleDoc)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(doc.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(doc.Tasks))
	}
	serialized := SerializeTasks(doc)
	doc2, err := ParseTasks(serialized)
	if err != nil {
		t.Fatalf("re-parse error: %v", err)
	}
	serialized2 := SerializeTasks(doc2)
	if serialized != serialized2 {
		t.Errorf("round-trip not stable\nfirst:  %q\nsecond: %q", serialized, serialized2)
	}
}

func TestParseAnnotationComplete(t *testing.T) {
	doc := `# Tasks — X

## Wave 1
- [x] T1 — Build API ✓ complete · evidence: tested · 2024-01-01T00:00:00Z
  - why: a
  - role: builder
  - files: f
  - contract: c
  - acceptance: a
  - verify: v
  - depends: —
`
	parsed, err := ParseTasks(doc)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	t1 := parsed.Tasks[0]
	if !t1.Checked {
		t.Error("expected checked=true")
	}
	if t1.Annotation == nil || t1.Annotation.Kind != AnnotComplete {
		t.Error("expected complete annotation")
	}
	if t1.Title != "Build API" {
		t.Errorf("expected bare title 'Build API', got %q", t1.Title)
	}
}

func TestMissingMandatoryKey(t *testing.T) {
	doc := `# Tasks — X

## Wave 1
- [ ] T1 — Missing fields
  - why: something
  - role: builder
  - files: f
  - contract: c
  - acceptance: a
  - depends: —
`
	_, err := ParseTasks(doc)
	if err == nil {
		t.Error("expected error for missing 'verify' key")
	}
	if !strings.Contains(err.Error(), "verify") {
		t.Errorf("error should mention 'verify', got: %v", err)
	}
}

func TestApplyTaskAnnotation(t *testing.T) {
	raw, err := ApplyTaskAnnotation(sampleDoc, "T1", true, &Annotation{
		Kind: AnnotComplete, Evidence: "passed", Ts: "2024-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("apply error: %v", err)
	}
	if !strings.Contains(raw, "[x]") {
		t.Error("expected [x] checkbox")
	}
	if !strings.Contains(raw, "✓ complete") {
		t.Error("expected ✓ complete annotation")
	}
}
