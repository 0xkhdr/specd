package core

import (
	"os"
	"strings"
	"testing"
)

func TestLoadInventoryCorrupt(t *testing.T) {
	root := t.TempDir()
	slug := "s"
	_ = os.MkdirAll(SpecDir(root, slug), 0o755)
	_ = AtomicWrite(InventoryPath(root, slug), "{not json")
	if _, err := LoadInventory(root, slug); err == nil {
		t.Error("corrupt inventory should error")
	}
	// Missing → nil, no error.
	if inv, err := LoadInventory(root, "absent"); inv != nil || err != nil {
		t.Errorf("missing inventory = %v, %v", inv, err)
	}
}

func TestAppendDeployEntryInvalidSlug(t *testing.T) {
	if err := AppendDeployEntry(t.TempDir(), "Bad Slug", DeployLedgerEntry{}); err == nil {
		t.Error("invalid slug should error")
	}
}

func TestSortedDeployEnvsEmpty(t *testing.T) {
	if envs := SortedDeployEnvs(t.TempDir()); envs != nil {
		t.Errorf("no deploy dir → %v, want nil", envs)
	}
}

func TestManifestModuleVariants(t *testing.T) {
	if goModModule([]byte("go 1.22\n")) != "" {
		t.Error("go.mod with no module directive should be empty")
	}
	if got := tomlPackageName([]byte("[project]\nname = \"proj\"\n")); got != "proj" {
		t.Errorf("[project] name = %q", got)
	}
	if got := tomlPackageName([]byte("[tool.poetry]\nname = \"po\"\n")); got != "po" {
		t.Errorf("[tool.poetry] name = %q", got)
	}
	if tomlPackageName([]byte("name = \"orphan\"\n")) != "" {
		t.Error("name outside a package table should be ignored")
	}
}

func TestReadCappedTruncates(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(strings.Repeat("a", 100))
	_ = f.Close()
	data, err := readCapped(f.Name(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 10 {
		t.Errorf("readCapped len = %d, want 10", len(data))
	}
}

func TestManifestModuleDispatch(t *testing.T) {
	dir := t.TempDir()
	gm := dir + "/go.mod"
	_ = os.WriteFile(gm, []byte("module example.com/z\n"), 0o644)
	if got := manifestModule(gm, "go.mod"); got != "example.com/z" {
		t.Errorf("manifestModule(go.mod) = %q", got)
	}
	// Non-manifest → "".
	other := dir + "/a.go"
	_ = os.WriteFile(other, []byte("package a\n"), 0o644)
	if got := manifestModule(other, "a.go"); got != "" {
		t.Errorf("manifestModule(a.go) = %q, want empty", got)
	}
}

func TestRenderObserveMidreqMinimal(t *testing.T) {
	p := ErrorPayload{Severity: "warning", Message: "m"}
	c := Correlation{Spec: "s", Impact: "medium", Confidence: "low", Facts: []string{"f"}}
	body := RenderObserveMidreq(p, c)
	if !strings.Contains(body, "impact medium") || strings.Contains(body, "Service:") {
		t.Errorf("minimal render wrong:\n%s", body)
	}
}

func TestDeployGateEvidenceGaps(t *testing.T) {
	// review missing vs wrong verdict.
	s := &State{Review: &ReviewRecord{Verdict: "revise"}}
	if deployGateEvidenceGap(s, "review") == "" {
		t.Error("revise verdict should gap")
	}
	s2 := &State{}
	if deployGateEvidenceGap(s2, "review") == "" || deployGateEvidenceGap(s2, "security") == "" {
		t.Error("missing review/security should gap")
	}
	// unknown gate → no gap (defensive).
	if deployGateEvidenceGap(s2, "bogus") != "" {
		t.Error("unknown gate should not gap")
	}
}

func TestLoadDeployPlanEdgeCases(t *testing.T) {
	root := t.TempDir()
	// Bad env → validation error before filesystem.
	if _, err := LoadDeployPlan(root, "../x"); err == nil {
		t.Error("bad env should error")
	}
	// Oversize file → rejected.
	dir := DeployPlanPath(root, "big")
	_ = os.MkdirAll(strings.TrimSuffix(dir, "/big.json"), 0o755)
	_ = os.WriteFile(dir, []byte(strings.Repeat("x", MaxDeployPlanBytes+1)), 0o644)
	if _, err := LoadDeployPlan(root, "big"); err == nil {
		t.Error("oversize plan should error")
	}
}

func TestReadDeployLedgerCorrupt(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(SpecDir(root, "s"), 0o755)
	_ = AppendFile(DeployLedgerPath(root, "s"), "{not json\n")
	if _, err := ReadDeployLedger(root, "s"); err == nil {
		t.Error("corrupt ledger line should error")
	}
}

func TestBuildInventorySkipsDirsAndMissing(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(root+"/adir", 0o755)
	_ = os.WriteFile(root+"/real.go", []byte("package x\n"), 0o644)
	// A directory and a missing file in the list are skipped; duplicates collapsed.
	inv, err := BuildInventory(root, "", []string{"adir", "real.go", "real.go", "gone.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) != 1 || inv.Files[0].Path != "real.go" {
		t.Fatalf("files = %+v, want only real.go", inv.Files)
	}
}

func TestManifestModuleMoreTypes(t *testing.T) {
	dir := t.TempDir()
	pj := dir + "/package.json"
	_ = os.WriteFile(pj, []byte(`{"name":"widget"}`), 0o644)
	if got := manifestModule(pj, "package.json"); got != "widget" {
		t.Errorf("package.json module = %q", got)
	}
	ct := dir + "/Cargo.toml"
	_ = os.WriteFile(ct, []byte("[package]\nname = \"crate\"\n"), 0o644)
	if got := manifestModule(ct, "Cargo.toml"); got != "crate" {
		t.Errorf("Cargo.toml module = %q", got)
	}
}

func TestMatchSpecFramesEdges(t *testing.T) {
	root := t.TempDir()
	// A task with a wildcard-only files contract is skipped.
	seedSpecWithFiles(t, root, "wild", "*", StatusExecuting)
	tasks, files := matchSpecFrames(root, "wild", []StackFrame{{File: "a.go"}})
	if len(tasks) != 0 || len(files) != 0 {
		t.Errorf("wildcard contract should not match: %v %v", tasks, files)
	}
	// A spec with no tasks.md yields nothing.
	_ = os.MkdirAll(SpecDir(root, "empty"), 0o755)
	if tk, fl := matchSpecFrames(root, "empty", []StackFrame{{File: "a.go"}}); tk != nil || fl != nil {
		t.Errorf("no tasks → %v %v", tk, fl)
	}
}

func TestFirstN(t *testing.T) {
	if got := firstN([]string{"a", "b", "c"}, 2); len(got) != 2 {
		t.Errorf("firstN cap = %v", got)
	}
	if got := firstN([]string{"a"}, 5); len(got) != 1 {
		t.Errorf("firstN under = %v", got)
	}
}

func TestMarshalInventoryStable(t *testing.T) {
	inv := Inventory{Base: "x", Files: []InventoryFile{{Path: "a", Size: 1}}}
	b1, err := MarshalInventory(inv)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := MarshalInventory(inv)
	if string(b1) != string(b2) || b1[len(b1)-1] != '\n' {
		t.Error("MarshalInventory not stable / missing trailing newline")
	}
}
