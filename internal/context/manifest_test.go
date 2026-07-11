package context

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestBuildManifest(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman"}}
	got, err := BuildManifest("", "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if got.Version != ManifestVersion || got.Mode != "craftsman" || len(got.Items) != 4 {
		t.Fatalf("manifest = %+v", got)
	}
	if got.Items[0].Kind != "role" || got.Items[3].Kind != "tasks" {
		t.Fatalf("items not deterministic: %+v", got.Items)
	}
}

func TestManifestValidate(t *testing.T) {
	manifest, err := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "validator"}}, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := ValidateManifest(raw); err != nil {
		t.Fatalf("ValidateManifest: %v", err)
	}
	if err := ValidateManifest([]byte(`{"version":"1"}`)); err == nil {
		t.Fatalf("ValidateManifest accepted malformed manifest")
	}
}

// TestManifestVersionFailsClosed (R1.3/R8.2) pins the W0 V1/V2 migration
// decision: V1 stays the compatibility renderer, and any unknown/unsupported
// manifest version is rejected rather than silently reinterpreted. When the
// typed V2 contract lands (W1) it must extend the accepted version set
// explicitly — this test then documents both accepted versions.
func TestManifestVersionFailsClosed(t *testing.T) {
	full := `{"version":"2","mode":"craftsman","slug":"demo","task_id":"T1","items":[{"kind":"a"},{"kind":"b"},{"kind":"c"},{"kind":"d"}]}`
	if err := ValidateManifest([]byte(full)); err == nil {
		t.Fatal("unknown manifest version 2 must fail closed until V2 lands")
	}
	if err := ValidateManifest([]byte(`{"version":"9"}`)); err == nil {
		t.Fatal("unsupported version must be rejected, not reinterpreted")
	}
}

// --- W1 T04 typed V2 schema, canonical order + digest (R1) -------------------

func validV2() ManifestV2 {
	return ManifestV2{
		SchemaVersion: ManifestVersionV2, Kind: manifestKindV2, Root: ".", Slug: "demo",
		Action: "implement", Phase: "post-design", TaskID: "T1",
		Items: []ItemV2{
			{Kind: "guardrails", Source: ".specd/steering/product.md", Required: true, LoadMode: "eager", Trust: "guardrail", Sensitivity: "public", Reason: "harness constitution", EstimatedTokens: 3},
			{Kind: "task", Selector: "T1", Required: true, LoadMode: "eager", Trust: "harness", Sensitivity: "public", Reason: "selected task record", EstimatedTokens: 5},
		},
	}
}

// TestManifestV2ValidateFailsClosed (R1.3): unknown version, kind, load_mode, or
// trust — or a missing required field — is rejected, never reinterpreted.
func TestManifestV2ValidateFailsClosed(t *testing.T) {
	if err := ValidateManifestV2(validV2()); err != nil {
		t.Fatalf("valid v2 manifest rejected: %v", err)
	}
	mut := []func(m *ManifestV2){
		func(m *ManifestV2) { m.SchemaVersion = "3" },
		func(m *ManifestV2) { m.Kind = "other" },
		func(m *ManifestV2) { m.TaskID = "" },
		func(m *ManifestV2) { m.Items[0].Kind = "wat" },
		func(m *ManifestV2) { m.Items[0].LoadMode = "sometimes" },
		func(m *ManifestV2) { m.Items[0].Trust = "vibes" },
		func(m *ManifestV2) { m.Items[0].Reason = "" },
		func(m *ManifestV2) { m.Items = nil },
	}
	for i, f := range mut {
		m := validV2()
		f(&m)
		if err := ValidateManifestV2(m); err == nil {
			t.Fatalf("mutation %d must fail closed", i)
		}
	}
}

// TestManifestV2CanonicalDigest (R1.4): identical inputs yield byte-identical
// item ordering and a stable manifest digest, independent of input item order,
// and the digest excludes itself.
func TestManifestV2CanonicalDigest(t *testing.T) {
	a := validV2()
	b := validV2()
	b.Items[0], b.Items[1] = b.Items[1], b.Items[0] // shuffled input
	CanonicalizeV2(&a)
	CanonicalizeV2(&b)
	da, db := ManifestV2Digest(a), ManifestV2Digest(b)
	if da != db {
		t.Fatalf("digest not order-independent: %s vs %s", da, db)
	}
	if a.Items[0].Kind != b.Items[0].Kind {
		t.Fatalf("canonical order differs: %q vs %q", a.Items[0].Kind, b.Items[0].Kind)
	}
	// Digest excludes the digest field itself (no self-reference).
	a.ManifestDigest = "deadbeef"
	if ManifestV2Digest(a) != da {
		t.Fatal("digest must not depend on the manifest_digest field")
	}
}

// --- W0 T02 R8 baseline fixtures ---------------------------------------------
// These characterize the current (pre-typed-v2) behavior for the R8 negative
// scenarios so each later wave's fix lands as a visible RED->GREEN flip. They
// pass today by design; the named wave updates the assertion when it fixes the
// defect. Scenarios: wrong-root, required-overflow, missing-design, tool-route,
// stale-receipt (steering-missing lives in steering_manifest_test.go).

// TestBuildManifestWrongRootBaseline (R8/R2.2) pins the wrong-tree defect:
// BuildManifest emits `specs/<slug>/...` instead of the runtime
// `.specd/specs/<slug>/...` base. W1 flips this to the canonical repo-base path.
func TestBuildManifestWrongRootBaseline(t *testing.T) {
	m, err := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	var spec Item
	for _, it := range m.Items {
		if it.Kind == "spec" {
			spec = it
		}
	}
	if !strings.HasPrefix(spec.Path, "specs/") || strings.HasPrefix(spec.Path, ".specd/") {
		t.Fatalf("baseline expected wrong-tree specs/ path, got %q — update this baseline in W1", spec.Path)
	}
}

// TestBuildManifestRequiredOverflowBaseline (R8/R3.2) pins the false-pass: when
// the required core exceeds the token budget, BuildManifest neither drops a core
// item nor errors — the overflow survives silently. W3 flips this to a
// fail-closed decomposition finding.
func TestBuildManifestRequiredOverflowBaseline(t *testing.T) {
	m, err := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 1)
	if err != nil {
		t.Fatalf("baseline expected silent survival, got error: %v — update this baseline in W3", err)
	}
	n := 0
	for _, it := range m.Items {
		switch it.Kind {
		case "spec", "tasks", "task", "role":
			n++
		}
	}
	if n != 4 {
		t.Fatalf("baseline expected all 4 core items to survive overflow, got %d", n)
	}
}

// TestBuildManifestMissingDesignBaseline (R8/R2.1) pins the gap: applicable
// design.md is never a required manifest item, so a missing design is
// undetectable. W1 adds the design lane and fails closed on a missing required
// item.
func TestBuildManifestMissingDesignBaseline(t *testing.T) {
	m, _ := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 0)
	for _, it := range m.Items {
		if it.Kind == "design" {
			t.Fatal("design lane already present — update this baseline in W1")
		}
	}
}

// TestBuildManifestNoRouteBaseline (R8/R4.2) pins the gap: the manifest carries
// no tool item and no route/capability metadata. W2 (driver contract) adds tool
// lanes with route/phase/capability.
func TestBuildManifestNoRouteBaseline(t *testing.T) {
	m, _ := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 0)
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), `"route"`) || strings.Contains(string(raw), `"kind":"tool"`) {
		t.Fatal("route/tool metadata already present — update this baseline in W2")
	}
}

// TestBuildManifestNoReceiptBaseline (R8/R5) pins the gap: the manifest has no
// manifest_digest and no receipt, so staleness is undetectable. W5/W6 add the
// receipt with config/palette/skill digests and freshness.
func TestBuildManifestNoReceiptBaseline(t *testing.T) {
	m, _ := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 0)
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), "manifest_digest") || strings.Contains(string(raw), "receipt") {
		t.Fatal("receipt/digest already present — update this baseline in W5/W6")
	}
}
