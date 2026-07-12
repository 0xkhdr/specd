package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRunSpanKindClosedEnumAndExtension pins the closed enum and its extension
// escape hatch (spec 07 R6.1): every documented kind parses, a namespaced
// extension ("x-…") is tolerated, and any other unknown critical kind fails so it
// cannot silently disappear from a trace.
func TestRunSpanKindClosedEnumAndExtension(t *testing.T) {
	for _, k := range []SpanKind{SpanContext, SpanModel, SpanTool, SpanEdit, SpanVerify, SpanEval, SpanApproval, SpanDispatch} {
		if got, err := ParseSpanKind(string(k)); err != nil || got != k {
			t.Fatalf("ParseSpanKind(%q) = (%q, %v), want (%q, nil)", k, got, err, k)
		}
	}
	if got, err := ParseSpanKind("x-otel-link"); err != nil || got != SpanKind("x-otel-link") {
		t.Fatalf("extension kind must parse, got (%q, %v)", got, err)
	}
	for _, bad := range []string{"", "deploy", "x-", "model ", "Verify"} {
		if _, err := ParseSpanKind(bad); err == nil {
			t.Fatalf("ParseSpanKind(%q) must fail closed", bad)
		}
	}
}

// TestRunSpanCodeEffectRequiresGitHead pins R6.3: a span claiming a code effect
// or completion — edit, verify, eval — is invalid without a git_head; a
// non-code-effect span (approval, dispatch) needs none.
func TestRunSpanCodeEffectRequiresGitHead(t *testing.T) {
	for _, k := range []SpanKind{SpanEdit, SpanVerify, SpanEval} {
		s := RunSpan{SpanID: "a", SpecID: "demo", Kind: k}
		if err := s.Validate(); err == nil {
			t.Fatalf("%s span without git_head must be rejected", k)
		}
		s.GitHead = "0000000000000000000000000000000000000000"
		if err := s.Validate(); err != nil {
			t.Fatalf("%s span with git_head must validate: %v", k, err)
		}
	}
	for _, k := range []SpanKind{SpanApproval, SpanDispatch, SpanContext, SpanModel, SpanTool} {
		s := RunSpan{SpanID: "a", SpecID: "demo", Kind: k}
		if err := s.Validate(); err != nil {
			t.Fatalf("%s span needs no git_head: %v", k, err)
		}
	}
}

// TestRenderTraceJSONStableAndResolvable pins R6.2: two exports over the same
// spans are byte-identical regardless of input order, ordering is resolved on
// equal/missing timestamps by the (SourceRank, Seq) tie-break, and the export
// fails closed on a duplicate id or an unresolvable parent reference.
func TestRenderTraceJSONStableAndResolvable(t *testing.T) {
	head := "0000000000000000000000000000000000000000"
	root := RunSpan{SpanID: "root", SpecID: "demo", Kind: SpanDispatch, TaskID: "T1", SourceRank: 0, Seq: 0}
	spans := []RunSpan{
		{SpanID: "b", ParentSpanID: "root", SpecID: "demo", Kind: SpanVerify, TaskID: "T1", GitHead: head, SourceRank: 3, Seq: 1},
		{SpanID: "a", ParentSpanID: "root", SpecID: "demo", Kind: SpanVerify, TaskID: "T1", GitHead: head, SourceRank: 3, Seq: 0},
		root,
	}
	first, err := RenderTraceJSON(spans)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Re-render from a reshuffled copy: byte-identical (R6.2).
	shuffled := []RunSpan{spans[2], spans[0], spans[1]}
	second, err := RenderTraceJSON(shuffled)
	if err != nil {
		t.Fatalf("render 2: %v", err)
	}
	if first != second {
		t.Fatalf("trace export not byte-identical:\n--- 1 ---\n%s\n--- 2 ---\n%s", first, second)
	}
	// Missing/equal timestamps resolve by (SourceRank, Seq): root, then a, then b.
	lines := strings.Split(strings.TrimRight(first, "\n"), "\n")
	wantOrder := []string{"root", "a", "b"}
	for i, want := range wantOrder {
		var s RunSpan
		if err := json.Unmarshal([]byte(lines[i]), &s); err != nil {
			t.Fatalf("line %d not JSON: %v", i, err)
		}
		if s.SpanID != want {
			t.Fatalf("line %d span_id = %q, want %q", i, s.SpanID, want)
		}
	}

	// A duplicate span id fails closed.
	dup := append([]RunSpan{}, spans...)
	dup = append(dup, RunSpan{SpanID: "root", SpecID: "demo", Kind: SpanDispatch})
	if _, err := RenderTraceJSON(dup); err == nil {
		t.Fatal("duplicate span_id must fail")
	}
	// An unresolvable parent reference fails closed.
	orphan := []RunSpan{{SpanID: "x", ParentSpanID: "ghost", SpecID: "demo", Kind: SpanApproval}}
	if _, err := RenderTraceJSON(orphan); err == nil {
		t.Fatal("unresolvable parent must fail")
	}
}

// TestRunSpanTimestampsInformational pins R6.3: ordering never depends on
// wall-clock alone. Spans whose timestamps sort opposite to their (SourceRank,
// Seq) still emit in tie-break order, so no outcome can be derived from clock
// order. NewSpanID is a pure function of stable coordinates, not time.
func TestRunSpanTimestampsInformational(t *testing.T) {
	a := NewSpanID("demo", SpanVerify, 3, 0, "task=T1")
	b := NewSpanID("demo", SpanVerify, 3, 0, "task=T1")
	if a != b || a == "" {
		t.Fatalf("NewSpanID not deterministic: %q vs %q", a, b)
	}
	if c := NewSpanID("demo", SpanVerify, 3, 1, "task=T1"); c == a {
		t.Fatalf("NewSpanID must differ by seq")
	}
	// A later-seq span with an earlier timestamp must still sort after by seq
	// once timestamps are equal/absent — clock order is not load-bearing.
	spans := []RunSpan{
		{SpanID: "late", SpecID: "demo", Kind: SpanApproval, StartedAt: "", SourceRank: 0, Seq: 5},
		{SpanID: "early", SpecID: "demo", Kind: SpanApproval, StartedAt: "", SourceRank: 0, Seq: 1},
	}
	out, err := RenderTraceJSON(spans)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, `{"span_id":"early"`) {
		t.Fatalf("tie-break order not by seq:\n%s", out)
	}
}
