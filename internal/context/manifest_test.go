package context

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestBuildQualityPacketIsCompactAndFreshnessLabelled(t *testing.T) {
	contract := core.QualityContract{
		TaskID: "T1", Verify: "go test ./...",
		Required: []core.EvidenceRequirement{{EvidenceClass: core.EvidenceOutputEval, CheckID: "quality"}, {EvidenceClass: core.EvidenceTest, CheckID: "unit"}},
	}
	subject := core.FreshnessSubject{Revision: "head", DatasetDigest: "dataset", RubricDigest: "rubric", OutputDigest: "output", TraceDigest: "trace"}
	records := []core.EvidenceEnvelopeV1{
		{TaskID: "T1", EvidenceClass: core.EvidenceTest, CheckID: "unit", Verdict: core.EvalPass, SubjectRevision: "head", DatasetDigest: "dataset", RubricDigest: "rubric", OutputDigest: "output", TraceDigest: "trace"},
		{TaskID: "T1", EvidenceClass: core.EvidenceOutputEval, CheckID: "quality", Verdict: core.EvalPass, SubjectRevision: "old", DatasetDigest: "old", RubricDigest: "rubric", OutputDigest: "output"},
	}
	p := BuildQualityPacket(contract, records, subject)
	if p.TaskID != "T1" || p.Verify != "go test ./..." || len(p.Required) != 2 {
		t.Fatalf("packet = %+v", p)
	}
	if p.Required[0].Status != "stale" || p.Required[1].Status != "passed" {
		t.Fatalf("statuses = %+v", p.Required)
	}
	rendered := RenderQualityPacket(p)
	for _, want := range []string{"quality contract", "test/unit", "output_eval/quality", "freshness=stale", "dataset: dataset", "rubric: rubric", "output: output", "trace: trace"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("packet missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "raw corpus") || strings.Contains(rendered, "secret payload") {
		t.Fatalf("packet contains raw payload: %s", rendered)
	}
}

func TestManifestItemsCarryStableMetadata(t *testing.T) {
	root := t.TempDir()
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"main.go"}}}
	a, err := BuildManifest(root, "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildManifest(root, "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatal(err)
	}
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	if string(ra) != string(rb) {
		t.Fatalf("metadata manifest not byte-stable")
	}
	for _, item := range a.Items {
		if item.Path != "" && (item.Reason == "" || item.Digest == "") {
			t.Fatalf("path item lacks context metadata: %+v", item)
		}
	}
}

func TestManifestSelectsPortableSkills(t *testing.T) {
	root := t.TempDir()
	writeManifestFixture(t, root, ".specd/specs/demo/requirements.md", "# Requirements\n")
	writeManifestFixture(t, root, ".specd/specs/demo/design.md", "# Design\n")
	writeManifestFixture(t, root, ".specd/roles/craftsman.md", "# Role\n")
	writeSkill(t, root, "go-test", validSkill("required: false\nbudget: 120"))
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", Acceptance: "R7"}}
	hs := core.Handshake{ConfigDigest: "config", PaletteDigest: "palette", ToolContracts: []core.ToolContract{
		{Name: "status", Phases: []core.Phase{core.PhaseExecute}, Capability: "read"},
		{Name: "verify", Phases: []core.Phase{core.PhaseExecute}, Capability: "write"},
	}}
	m, err := BuildMachineManifest(root, "demo", tasks, "T1", "execute", "execute", 0, hs)
	if err != nil {
		t.Fatal(err)
	}
	var skill MachineItem
	for _, item := range m.Items {
		if item.Kind == "skill" {
			skill = item
		}
	}
	if skill.Source != ".specd/skills/go-test/SKILL.md" || skill.SourceDigest == "" || skill.EstimatedTokens != 120 {
		t.Fatalf("skill = %+v", skill)
	}
	if m.ManifestDigest == "" || m.ManifestDigest != MachineManifestDigest(m) || m.OptionalTokens < 120 {
		t.Fatalf("manifest digest/totals = %+v", m)
	}
	again, err := BuildMachineManifest(root, "demo", tasks, "T1", "execute", "execute", m.RequiredTokens, hs)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range again.Items {
		if item.Kind == "skill" {
			t.Fatalf("optional skill survived tight budget: %+v", item)
		}
	}
	foundOmission := false
	for _, omission := range again.Omissions {
		if omission.Kind == "skill" && omission.Source == ".specd/skills/go-test/SKILL.md" {
			foundOmission = true
		}
	}
	if !foundOmission {
		t.Fatalf("skill budget omission missing: %+v", again.Omissions)
	}
}

func TestManifestFreshScaffoldSelectsApplicableSkills(t *testing.T) {
	root := t.TempDir()
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	writeManifestFixture(t, root, ".specd/specs/demo/requirements.md", "# Requirements\n")
	writeManifestFixture(t, root, ".specd/specs/demo/design.md", "# Design\n")
	writeManifestFixture(t, root, "main.go", "package main\n")
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"main.go"}, Verify: "go test ./...", Acceptance: "R1.1"}}
	state := core.InitialState("demo")
	hs, err := core.BootstrapHandshakeForRoot(root, core.Config{}, &state, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildMachineManifest(root, "demo", tasks, "T1", "execute", "execute", 0, hs)
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, item := range m.Items {
		if item.Kind != "skill" {
			continue
		}
		selected[item.Selector] = true
		if item.LoadMode != "lazy" || item.SourceDigest == "" || item.RepresentationDigest == "" || item.EstimatedTokens <= 0 || item.ContentTrust != ContentTrustUntrustedData {
			t.Errorf("selected skill lacks pinned lazy contract: %+v", item)
		}
	}
	for _, want := range []string{"skill:foundation@1.0.0", "skill:execute@1.0.0"} {
		if !selected[want] {
			t.Errorf("applicable skill not selected: %s; got %v", want, selected)
		}
	}
	for _, reject := range []string{"skill:requirements@1.0.0", "skill:design@1.0.0", "skill:delivery@1.0.0"} {
		if selected[reject] {
			t.Errorf("inapplicable skill selected: %s", reject)
		}
	}
}

func writeManifestFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
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

func TestManifestCarriesRoutingRecommendationMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte("routing:\n  classes: standard,reasoning\n  default_class: standard\n  fallback: standard,reasoning\n  class_capabilities: standard=context;reasoning=context+eval\n  recommendations: high=reasoning\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := BuildManifest(root, "demo", []core.TaskRow{{ID: "T1", Role: "craftsman", Complexity: "high"}}, "T1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Routing == nil || m.Routing.Class != "reasoning" || m.Routing.Model != "" {
		t.Fatalf("routing metadata = %#v", m.Routing)
	}
}

// TestManifestVersionFailsClosed (R1.3/R8.2) pins the manifest-version
// decision: V1 stays the compatibility renderer, and any unknown/unsupported
// manifest version is rejected rather than silently reinterpreted. When the
// human manifest schema ever changes it must extend the accepted version set
// explicitly — this test then documents both accepted versions.
func TestManifestVersionFailsClosed(t *testing.T) {
	full := `{"version":"99","mode":"craftsman","slug":"demo","task_id":"T1","items":[{"kind":"a"},{"kind":"b"},{"kind":"c"},{"kind":"d"}]}`
	if err := ValidateManifest([]byte(full)); err == nil {
		t.Fatal("unknown manifest version must fail closed")
	}
	if err := ValidateManifest([]byte(`{"version":"9"}`)); err == nil {
		t.Fatal("unsupported version must be rejected, not reinterpreted")
	}
}

// --- Typed machine manifest schema, canonical order + digest (R1) ------------

func validMachineManifest() MachineManifest {
	return MachineManifest{
		SchemaVersion: MachineManifestVersion, Kind: machineManifestKind, Root: ".", Slug: "demo",
		Action: "implement", Phase: "post-design", TaskID: "T1",
		Items: []MachineItem{
			{Kind: "guardrails", Source: ".specd/steering/product.md", SourceDigest: "guardrail-digest", Required: true, LoadMode: "eager", Trust: "guardrail", ContentTrust: ContentTrustUntrustedData, Sensitivity: "public", Reason: "harness constitution", EstimatedTokens: 3},
			{Kind: "task", Selector: "T1", SourceDigest: "task-digest", Required: true, LoadMode: "eager", Trust: "harness", ContentTrust: ContentTrustUntrustedData, Sensitivity: "public", Reason: "selected task record", EstimatedTokens: 5},
		},
	}
}

// TestMachineManifestValidateFailsClosed (R1.3): unknown version, kind, load_mode, or
// trust — or a missing required field — is rejected, never reinterpreted.
func TestMachineManifestValidateFailsClosed(t *testing.T) {
	if err := ValidateMachineManifest(validMachineManifest()); err != nil {
		t.Fatalf("valid v2 manifest rejected: %v", err)
	}
	mut := []func(m *MachineManifest){
		func(m *MachineManifest) { m.SchemaVersion = "3" },
		func(m *MachineManifest) { m.Kind = "other" },
		func(m *MachineManifest) { m.TaskID = "" },
		func(m *MachineManifest) { m.Items[0].Kind = "wat" },
		func(m *MachineManifest) { m.Items[0].LoadMode = "sometimes" },
		func(m *MachineManifest) { m.Items[0].Trust = "vibes" },
		func(m *MachineManifest) { m.Items[0].ContentTrust = "trusted_by_claim" },
		func(m *MachineManifest) { m.Items[0].Reason = "" },
		func(m *MachineManifest) { m.Items = nil },
	}
	for i, f := range mut {
		m := validMachineManifest()
		f(&m)
		if err := ValidateMachineManifest(m); err == nil {
			t.Fatalf("mutation %d must fail closed", i)
		}
	}
}

// TestMachineManifestCanonicalDigest (R1.4): identical inputs yield byte-identical
// item ordering and a stable manifest digest, independent of input item order,
// and the digest excludes itself.
func TestMachineManifestCanonicalDigest(t *testing.T) {
	a := validMachineManifest()
	b := validMachineManifest()
	b.Items[0], b.Items[1] = b.Items[1], b.Items[0] // shuffled input
	CanonicalizeMachineManifest(&a)
	CanonicalizeMachineManifest(&b)
	da, db := MachineManifestDigest(a), MachineManifestDigest(b)
	if da != db {
		t.Fatalf("digest not order-independent: %s vs %s", da, db)
	}
	if a.Items[0].Kind != b.Items[0].Kind {
		t.Fatalf("canonical order differs: %q vs %q", a.Items[0].Kind, b.Items[0].Kind)
	}
	// Digest excludes the digest field itself (no self-reference).
	a.ManifestDigest = "deadbeef"
	if MachineManifestDigest(a) != da {
		t.Fatal("digest must not depend on the manifest_digest field")
	}
}

func TestMachineManifestSelectedTaskRecord(t *testing.T) {
	m := validMachineManifest()
	m.SelectedTask = MachineSelectedTask{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"a.go", "a_test.go"}, Verify: "go test ./...", Acceptance: "R2.1"}
	if err := ValidateMachineManifest(m); err != nil {
		t.Fatalf("structured selected task rejected: %v", err)
	}
	m.SelectedTask.DeclaredFiles = []string{"../escape"}
	if err := ValidateMachineManifest(m); err == nil {
		t.Fatal("unsafe declared file accepted")
	}
}

func TestManifestAuthorityPacket(t *testing.T) {
	m := validMachineManifest()
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
	state := core.InitialState("demo")
	hs, err := core.BootstrapHandshakeForRoot(t.TempDir(), core.Config{}, &state, nil)
	if err != nil {
		t.Fatal(err)
	}
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

// TestManifestConformanceFailureMatrix is the W6 black-box contract: every
// required lane failure is named, required overflow is not truncated, and a
// receipt built from changed required context is stale.
func TestManifestConformanceFailureMatrix(t *testing.T) {
	root := t.TempDir()
	writeManifestFixture(t, root, ".specd/specs/demo/requirements.md", "# Requirements\n")
	writeManifestFixture(t, root, ".specd/roles/craftsman.md", "# Role\n")
	task := core.TaskRow{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"internal/main.go"}}
	_, err := BuildMachineManifest(root, "demo", []core.TaskRow{task}, "T1", "execute", "execute", 0, core.BootstrapHandshake(core.Config{}))
	if err == nil || !strings.Contains(err.Error(), ".specd/specs/demo/design.md") {
		t.Fatalf("missing required design must be named: %v", err)
	}

	if _, err := ResolveSource(root, ".specd/specs/demo/../../../../etc/passwd"); err == nil {
		t.Fatal("wrong-root traversal accepted")
	}
	if _, _, _, _, err := EnforceMachineBudget([]MachineItem{{Kind: "task", Required: true, EstimatedTokens: 10, Reason: "task"}}, 1); err == nil {
		t.Fatal("required overflow accepted")
	}

	m := receiptManifest()
	r, err := BuildReceipt(m)
	if err != nil {
		t.Fatal(err)
	}
	m.Items[0].RepresentationDigest = strings.Repeat("f", 64)
	m.ManifestDigest = MachineManifestDigest(m)
	if stale := ReceiptStaleness(r, m); len(stale) == 0 {
		t.Fatal("changed required context did not stale receipt")
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

// TestBuildManifestRequiredOverflowFailsClosed (R3.2): when the required core
// exceeds the token budget, BuildManifest fails closed with a BudgetError that
// names the concise remediation — it never silently truncates a required item.
func TestBuildManifestRequiredOverflowFailsClosed(t *testing.T) {
	_, err := BuildManifest("", "demo", []core.TaskRow{{ID: "T1", Role: "craftsman"}}, "T1", 1)
	be, ok := err.(BudgetError)
	if !ok {
		t.Fatalf("required overflow must fail closed with BudgetError, got %v", err)
	}
	if be.Budget != 1 || be.RequiredTokens <= 1 {
		t.Fatalf("BudgetError = %+v", be)
	}
	for _, want := range []string{"exceeds budget", "decompose", "narrow declared files"} {
		if !strings.Contains(be.Error(), want) {
			t.Fatalf("remediation missing %q: %s", want, be.Error())
		}
	}
}

// TestManifestByteIdentical (R3.4): identical inputs yield a byte-identical
// manifest and a stable canonical digest across repeated builds.
func TestManifestByteIdentical(t *testing.T) {
	root := t.TempDir()
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"a.go", "a_test.go"}}}
	a, err := BuildManifest(root, "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	b, err := BuildManifest(root, "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	if string(ra) != string(rb) {
		t.Fatalf("manifest not byte-identical:\n%s\n%s", ra, rb)
	}
	if ManifestDigest(a) != ManifestDigest(b) || ManifestDigest(a) == "" {
		t.Fatalf("manifest digest unstable: %q vs %q", ManifestDigest(a), ManifestDigest(b))
	}
}

// TestManifestAccountingDistinctQuantities (R3.3): the estimate, host-reported,
// and provider-billed tokens are distinct fields; host-reported and
// provider-billed default to unknown (nil), never zero; the ledger carries the
// canonical digest and the supplied items, and RecordHostAck fills only the
// host-reported quantity.
func TestManifestAccountingDistinctQuantities(t *testing.T) {
	root := t.TempDir()
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"a.go"}}}
	m, err := BuildManifest(root, "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	acc := BuildAccounting(m)
	if acc.EstimatedInputTokens != m.EstimatedTokens {
		t.Fatalf("estimate = %d, want %d", acc.EstimatedInputTokens, m.EstimatedTokens)
	}
	if acc.HostReportedInputTokens != nil || acc.ProviderBilledTokens != nil {
		t.Fatal("host-reported and provider-billed must default to unknown (nil), not zero")
	}
	if acc.ContextManifestDigest != ManifestDigest(m) || acc.ContextManifestDigest == "" {
		t.Fatalf("digest = %q", acc.ContextManifestDigest)
	}
	if len(acc.SuppliedItems) != len(m.Items) {
		t.Fatalf("supplied items = %d, want %d", len(acc.SuppliedItems), len(m.Items))
	}

	acc.RecordHostAck(HostAck{ManifestDigest: acc.ContextManifestDigest, ReportedTokens: 4242})
	if acc.HostReportedInputTokens == nil || *acc.HostReportedInputTokens != 4242 {
		t.Fatalf("host ack not recorded: %+v", acc.HostReportedInputTokens)
	}
	if acc.ProviderBilledTokens != nil {
		t.Fatal("recording host ack must not fabricate a provider-billed value")
	}
	if acc.HostAck == nil || acc.HostAck.ManifestDigest != acc.ContextManifestDigest {
		t.Fatalf("host ack = %+v", acc.HostAck)
	}
}

// TestManifestAccountingOmissions (R3.3): optional items shed under budget are
// reported as omitted items with a ref and a reason, and appear in the ledger.
func TestManifestAccountingOmissions(t *testing.T) {
	root := t.TempDir()
	writeMem(t, root, "demo", 400) // memory ~100 tokens, reference-if-needed
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman"}}
	m, err := BuildManifest(root, "demo", tasks, "T1", 50)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if len(m.Omissions) == 0 {
		t.Fatal("expected the over-budget memory item to be shed")
	}
	acc := BuildAccounting(m)
	if len(acc.OmittedItems) == 0 || acc.OmittedItems[0].Reason == "" || acc.OmittedItems[0].Kind != "memory" {
		t.Fatalf("omitted items = %+v", acc.OmittedItems)
	}
	for _, it := range m.Items {
		if it.Kind == "memory" {
			t.Fatal("memory should have been shed to fit budget")
		}
	}
}

func writeMem(t *testing.T, root, slug string, n int) {
	t.Helper()
	dir := filepath.Join(root, ".specd", "steering")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte(strings.Repeat("m", n)), 0o644); err != nil {
		t.Fatal(err)
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
