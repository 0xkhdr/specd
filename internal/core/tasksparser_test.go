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

// metaBlock returns the 7 mandatory meta lines for a task, with the given deps.
func metaBlock(deps string) string {
	return "  - why: w\n  - role: builder\n  - files: f\n  - contract: c\n" +
		"  - acceptance: a\n  - verify: v\n  - depends: " + deps + "\n"
}

func TestParseMalformed(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr string // substring; "" means must parse without error
	}{
		{
			name: "empty_deps_element_skipped",
			doc:  "# Tasks — X\n\n## Wave 1\n- [ ] T1 — a\n" + metaBlock("T2, , T3") + "- [ ] T2 — b\n" + metaBlock("—") + "- [ ] T3 — c\n" + metaBlock("—"),
		},
		{
			name:    "duplicate_id_rejected",
			doc:     "# Tasks — X\n\n## Wave 1\n- [ ] T1 — a\n" + metaBlock("—") + "- [ ] T1 — b\n" + metaBlock("—"),
			wantErr: "duplicate task id T1",
		},
		{
			name:    "checkbox_without_meta",
			doc:     "# Tasks — X\n\n## Wave 1\n- [ ] T1 — a\n",
			wantErr: "missing key",
		},
		{
			name:    "meta_before_task",
			doc:     "# Tasks — X\n\n## Wave 1\n  - why: orphaned\n",
			wantErr: "outside of a task",
		},
		{
			name:    "task_before_wave",
			doc:     "# Tasks — X\n\n- [ ] T1 — a\n" + metaBlock("—"),
			wantErr: "before any",
		},
		{
			name:    "missing_title_header",
			doc:     "## Wave 1\n- [ ] T1 — a\n" + metaBlock("—"),
			wantErr: "missing '# Tasks",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := ParseTasks(tt.doc)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				// deterministic: deps with empty element drops the blank.
				if got := doc.Tasks[0].Meta["depends"]; got != "" {
					deps := ParseDepends(got)
					for _, d := range deps {
						if strings.TrimSpace(d) == "" {
							t.Fatalf("ParseDepends kept empty element: %v", deps)
						}
					}
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseDependsEmptyElements(t *testing.T) {
	got := ParseDepends("T1, , T3,")
	if len(got) != 2 || got[0] != "T1" || got[1] != "T3" {
		t.Fatalf("expected [T1 T3], got %v", got)
	}
}

func TestAnnotationSeparatorRoundTrip(t *testing.T) {
	// Evidence containing the field delimiter, a literal middot, and a newline
	// must round-trip losslessly through serialize -> parse.
	tricky := []string{
		"go test passed · 3 cases · ok",
		"value=a·b·c",
		"line one\nline two",
		`backslash \ and \n literal`,
		"plain evidence",
	}
	for _, ev := range tricky {
		doc := ParsedTasks{
			Title: "X",
			Tasks: []ParsedTask{{
				ID:         "T1",
				Title:      "a",
				Wave:       1,
				Checked:    true,
				Meta:       map[string]string{"why": "w", "role": "builder", "files": "f", "contract": "c", "acceptance": "a", "verify": "v", "depends": "—"},
				Annotation: &Annotation{Kind: AnnotComplete, Evidence: ev, Ts: "2024-01-01T00:00:00Z"},
			}},
		}
		serialized := SerializeTasks(doc)
		// The annotation line must not be broken by an embedded newline.
		lines := strings.Split(serialized, "\n")
		titleLines := 0
		for _, l := range lines {
			if strings.HasPrefix(l, "- [x] T1 —") {
				titleLines++
			}
		}
		if titleLines != 1 {
			t.Fatalf("evidence %q produced %d title lines, want 1\n%s", ev, titleLines, serialized)
		}
		parsed, err := ParseTasks(serialized)
		if err != nil {
			t.Fatalf("evidence %q: parse error: %v", ev, err)
		}
		got := parsed.Tasks[0].Annotation
		if got == nil || got.Evidence != ev {
			t.Fatalf("evidence round-trip lost data: want %q, got %q", ev, got.Evidence)
		}
	}
}

func TestEncodeDecodeAnnotationField(t *testing.T) {
	cases := []string{"", "plain", "a·b", "a\nb", `\already escaped`, "·\n\\·"}
	for _, c := range cases {
		if got := decodeAnnotationField(encodeAnnotationField(c)); got != c {
			t.Errorf("round-trip failed for %q: got %q", c, got)
		}
	}
	// Legacy unescaped fields decode to themselves.
	if got := decodeAnnotationField("legacy evidence no escapes"); got != "legacy evidence no escapes" {
		t.Errorf("legacy decode changed value: %q", got)
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
