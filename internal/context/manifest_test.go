package context

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestBuildManifest(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"a.go", "a_test.go"}, Verify: "go test ./...", Acceptance: "R2.2"}}
	got, err := BuildManifest("", "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if got.Version != ManifestVersion || got.Mode != "craftsman" || len(got.Items) != 7 {
		t.Fatalf("manifest = %+v", got)
	}
	var task Item
	for _, item := range got.Items {
		if item.Kind == "task" {
			task = item
		}
	}
	if task.Role != "craftsman" || task.Verify != "go test ./..." || task.Acceptance != "R2.2" {
		t.Fatalf("task guidance = %+v", task)
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
			{Kind: "guardrails", Source: ".specd/steering/product.md", SourceDigest: "guardrail-digest", Required: true, LoadMode: "eager", Trust: "guardrail", ContentTrust: ContentTrustUntrustedData, Sensitivity: "public", Reason: "harness constitution", EstimatedTokens: 3},
			{Kind: "task", Selector: "T1", SourceDigest: "task-digest", Required: true, LoadMode: "eager", Trust: "harness", ContentTrust: ContentTrustUntrustedData, Sensitivity: "public", Reason: "selected task record", EstimatedTokens: 5},
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
		func(m *ManifestV2) { m.Items[0].ContentTrust = "trusted_by_claim" },
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

func TestManifestV2SelectedTaskRecord(t *testing.T) {
	m := validV2()
	m.SelectedTask = SelectedTaskV2{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"a.go", "a_test.go"}, Verify: "go test ./...", Acceptance: "R2.1"}
	if err := ValidateManifestV2(m); err != nil {
		t.Fatalf("structured selected task rejected: %v", err)
	}
	m.SelectedTask.DeclaredFiles = []string{"../escape"}
	if err := ValidateManifestV2(m); err == nil {
		t.Fatal("unsafe declared file accepted")
	}
}

func TestManifestAuthorityPacket(t *testing.T) {
	m := validV2()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, err := core.BuildAuthority(core.TaskRow{ID: m.TaskID, Role: "craftsman", DeclaredFiles: []string{"a.go"}}, "controller", "w", m.Slug, m.Phase, "abc", "policy", "required", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	m, err = AttachAuthority(m, a)
	if err != nil {
		t.Fatal(err)
	}
	if m.Authority == nil || m.Authority.Digest == "" {
		t.Fatalf("manifest=%+v", m)
	}
}

func TestModeForTaskFailsClosed(t *testing.T) {
	if got := ModeForTask(core.TaskRow{Role: "auditor"}); got != "auditor" {
		t.Fatalf("auditor mode=%q", got)
	}
	if got := ModeForTask(core.TaskRow{Role: "unknown"}); got != "invalid" {
		t.Fatalf("unknown mode=%q", got)
	}
}

func TestManifestDriverLanes(t *testing.T) {
	hs := core.BootstrapHandshake(core.Config{})
	items := DriverItems(hs, "execute", "craftsman")
	if len(items) < 2 {
		t.Fatalf("driver items = %+v", items)
	}
	if items[0].Kind != "guardrails" {
		t.Fatalf("first driver lane = %+v", items[0])
	}
	for _, item := range items[1:] {
		if item.Kind != "tools" || item.Route == "" || item.Capability == "" || item.SourceDigest != hs.PaletteDigest {
			t.Fatalf("incomplete tool lane: %+v", item)
		}
	}
}

// --- W0 T02 R8 baseline fixtures ---------------------------------------------
// These characterize the current (pre-typed-v2) behavior for the R8 negative
// scenarios so each later wave's fix lands as a visible RED->GREEN flip. They
// pass today by design; the named wave updates the assertion when it fixes the
// defect. Scenarios: wrong-root, required-overflow, missing-design, tool-route,
// stale-receipt (steering-missing lives in steering_manifest_test.go).

func TestBuildManifestUsesRuntimeSpecRoot(t *testing.T) {
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
	if spec.Path != ".specd/specs/demo/requirements.md" {
		t.Fatalf("spec path = %q", spec.Path)
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

func TestBuildManifestIncludesRequiredDesign(t *testing.T) {
	m, _ := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 0)
	found := false
	for _, it := range m.Items {
		if it.Kind == "design" {
			found = it.Required && it.Path == ".specd/specs/demo/design.md"
		}
	}
	if !found {
		t.Fatal("required design lane missing")
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
