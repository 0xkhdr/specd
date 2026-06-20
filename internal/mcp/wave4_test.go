package mcp

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// td is a minimal toolDef carrying just the name the Wave 4 filters key on.
func td(name string) toolDef { return toolDef{Name: name} }

func names(tools []toolDef) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Name
	}
	return out
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote, so
// tests can assert the R4/diagnostic lines the filters emit.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stderr = orig
	return <-done
}

// --- C1: context-manifest tool filtering -----------------------------------

func TestApplyManifestFilterRequiredOptional(t *testing.T) {
	// AC1: required+optional define the allowlist; everything else is dropped.
	cand := []toolDef{td("specd_inspect"), td("specd_verify"), td("specd_task"), td("specd_status")}
	m := core.ContextManifestTools{
		RequiredTools: []string{"specd_inspect", "specd_verify"},
		OptionalTools: []string{"specd_task"},
	}
	got := names(applyManifestFilter(cand, m))
	want := []string{"specd_inspect", "specd_verify", "specd_task"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("allowlist = %v, want %v", got, want)
	}
}

func TestApplyManifestForbiddenWins(t *testing.T) {
	// AC2/R3: forbidden excludes a tool even when required/optional would allow it.
	cand := []toolDef{td("specd_inspect"), td("specd_approve")}
	m := core.ContextManifestTools{
		RequiredTools:  []string{"specd_inspect", "specd_approve"},
		ForbiddenTools: []string{"specd_approve"},
	}
	got := names(applyManifestFilter(cand, m))
	if strings.Join(got, ",") != "specd_inspect" {
		t.Fatalf("forbidden not excluded: %v", got)
	}
}

func TestApplyManifestNoManifestUnchanged(t *testing.T) {
	// AC4: an empty manifest is a no-op.
	cand := []toolDef{td("specd_status"), td("specd_verify")}
	got := applyManifestFilter(cand, core.ContextManifestTools{})
	if len(got) != len(cand) {
		t.Fatalf("empty manifest altered list: %v", names(got))
	}
}

func TestApplyManifestRequiredGatedOffDiagnostic(t *testing.T) {
	// AC3/R4: a required tool missing from the (config-gated) candidate set stays
	// excluded and emits a diagnostic — config safety wins over manifest required.
	cand := []toolDef{td("specd_status")}
	m := core.ContextManifestTools{RequiredTools: []string{"specd_status", "specd_update"}}
	var got []toolDef
	diag := captureStderr(t, func() { got = applyManifestFilter(cand, m) })
	if strings.Join(names(got), ",") != "specd_status" {
		t.Fatalf("gated required leaked into list: %v", names(got))
	}
	if !strings.Contains(diag, "specd_update") || !strings.Contains(diag, "config gate") {
		t.Fatalf("missing R4 diagnostic, got: %q", diag)
	}
}

func TestApplyManifestUnknownNameIgnored(t *testing.T) {
	cand := []toolDef{td("specd_status")}
	m := core.ContextManifestTools{RequiredTools: []string{"specd_status", "specd_bogus"}}
	var got []toolDef
	diag := captureStderr(t, func() { got = applyManifestFilter(cand, m) })
	if strings.Join(names(got), ",") != "specd_status" {
		t.Fatalf("unknown name affected output: %v", names(got))
	}
	if !strings.Contains(diag, "specd_bogus") || !strings.Contains(diag, "unknown tool") {
		t.Fatalf("missing unknown-name warning, got: %q", diag)
	}
}

// --- C2: host capability negotiation ---------------------------------------

func TestApplyHostPrefsMaxToolsCap(t *testing.T) {
	// AC1: maxTools caps the emitted count.
	cand := []toolDef{td("a"), td("b"), td("c"), td("d"), td("e"), td("f")}
	got := applyHostPrefs(cand, hostPrefs{maxTools: 5}, nil)
	if len(got) != 5 {
		t.Fatalf("maxTools=5 emitted %d tools", len(got))
	}
}

func TestApplyHostPrefsPreferredNamespaceOrder(t *testing.T) {
	// AC2: a preferred namespace (named by a member tool) orders its tools first,
	// preserving relative order within and outside the bucket.
	cand := []toolDef{td("specd_status"), td("specd_inspect"), td("specd_verify"), td("specd_read")}
	got := names(applyHostPrefs(cand, hostPrefs{preferredNamespaces: []string{"specd_read"}}, nil))
	want := []string{"specd_inspect", "specd_read", "specd_status", "specd_verify"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("namespace order = %v, want %v", got, want)
	}
}

func TestApplyHostPrefsRequiredSurvivesCap(t *testing.T) {
	// AC3/R4: required tools beyond maxTools are still all emitted, with a stderr
	// diagnostic — safety gates win over the cap.
	cand := []toolDef{td("specd_inspect"), td("specd_verify"), td("specd_task"), td("specd_status")}
	required := map[string]bool{"specd_inspect": true, "specd_verify": true, "specd_task": true}
	var got []toolDef
	diag := captureStderr(t, func() {
		got = applyHostPrefs(cand, hostPrefs{maxTools: 2}, required)
	})
	gm := toolNames(got)
	for n := range required {
		if !gm[n] {
			t.Fatalf("required %s dropped by cap: %v", n, names(got))
		}
	}
	if !strings.Contains(diag, "maxTools=2") {
		t.Fatalf("missing over-cap diagnostic, got: %q", diag)
	}
}

func TestApplyHostPrefsNoHintsNoop(t *testing.T) {
	// AC4: no hints ⇒ identical slice (order + length).
	cand := []toolDef{td("a"), td("b"), td("c")}
	got := applyHostPrefs(cand, hostPrefs{}, nil)
	if strings.Join(names(got), ",") != "a,b,c" {
		t.Fatalf("no-hint path altered list: %v", names(got))
	}
}

func TestApplyHostPrefsGarbageIsSafe(t *testing.T) {
	// AC5: a negative maxTools (clamped at parse) and unknown namespaces no-op.
	cand := []toolDef{td("specd_status"), td("specd_inspect")}
	hp := parseHostPrefs([]byte(`{"capabilities":{"specd":{"maxTools":-3,"preferredNamespaces":["nope"]}}}`))
	if hp.maxTools != 0 {
		t.Fatalf("negative maxTools not clamped: %+v", hp)
	}
	// The unknown namespace is dropped at apply time, leaving the list untouched.
	got := applyHostPrefs(cand, hp, nil)
	if strings.Join(names(got), ",") != "specd_status,specd_inspect" {
		t.Fatalf("garbage altered list: %v", names(got))
	}
}

func TestParseHostPrefs(t *testing.T) {
	hp := parseHostPrefs([]byte(`{"capabilities":{"specd":{"maxTools":5,"preferredNamespaces":["read"]}}}`))
	if hp.maxTools != 5 || len(hp.preferredNamespaces) != 1 || hp.preferredNamespaces[0] != "read" {
		t.Fatalf("parsed prefs wrong: %+v", hp)
	}
}
