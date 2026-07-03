package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writeTree writes files (path→content) under root, creating parents.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBuildInventoryDeterministic proves the same tree yields byte-identical
// inventory.json across two runs regardless of input order (V10 §5).
func TestBuildInventoryDeterministic(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":       "module example.com/foo\n\ngo 1.22\n",
		"pkg/a.go":     "package pkg\n",
		"pkg/b.go":     "package pkg\n",
		"cmd/main.go":  "package main\n",
		"package.json": `{"name": "@acme/widget", "version": "1.0.0"}`,
	})

	inv1, err := BuildInventory(root, "", []string{"pkg/b.go", "go.mod", "cmd/main.go", "pkg/a.go", "package.json"})
	if err != nil {
		t.Fatal(err)
	}
	inv2, err := BuildInventory(root, "", []string{"package.json", "pkg/a.go", "cmd/main.go", "go.mod", "pkg/b.go"})
	if err != nil {
		t.Fatal(err)
	}
	b1, _ := MarshalInventory(inv1)
	b2, _ := MarshalInventory(inv2)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("inventory not deterministic:\n%s\n---\n%s", b1, b2)
	}
	if len(inv1.Files) != 5 {
		t.Errorf("files = %d, want 5", len(inv1.Files))
	}
	if len(inv1.Modules) != 2 || inv1.Modules[0] != "@acme/widget" || inv1.Modules[1] != "example.com/foo" {
		t.Errorf("modules = %v, want sorted [@acme/widget example.com/foo]", inv1.Modules)
	}
}

// TestBuildInventorySkipsSymlinks confirms symlinks are not followed (explicit
// policy, V10 §5).
func TestBuildInventorySkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"real.go": "package x\n"})
	link := filepath.Join(root, "link.go")
	if err := os.Symlink(filepath.Join(root, "real.go"), link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	inv, err := BuildInventory(root, "", []string{"real.go", "link.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) != 1 || inv.Files[0].Path != "real.go" {
		t.Fatalf("expected only real.go, got %+v", inv.Files)
	}
}

// TestManifestParsers covers the stdlib manifest extractors.
func TestManifestParsers(t *testing.T) {
	if got := goModModule([]byte("// comment\nmodule example.com/x\ngo 1.22\n")); got != "example.com/x" {
		t.Errorf("goModModule = %q", got)
	}
	if got := packageJSONName([]byte(`{"name":"widget"}`)); got != "widget" {
		t.Errorf("packageJSONName = %q", got)
	}
	if got := tomlPackageName([]byte("[package]\nname = \"rustcrate\"\nversion = \"0.1\"\n")); got != "rustcrate" {
		t.Errorf("tomlPackageName = %q", got)
	}
	// Malformed inputs return "" (never panic).
	if goModModule([]byte("garbage")) != "" || packageJSONName([]byte("{")) != "" || tomlPackageName([]byte("[dependencies]\nx=1")) != "" {
		t.Error("malformed manifests should yield empty string")
	}
}

// TestComputeIngestCoverage is the coverage-math table: mapped (referenced),
// waived (reasoned), and unmapped (V10/P5.3).
func TestComputeIngestCoverage(t *testing.T) {
	inv := Inventory{
		Files: []InventoryFile{
			{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}, {Path: "d.go"},
		},
		Waivers: map[string]string{
			"c.go": "generated code, out of scope",
			"d.go": "  ", // empty reason does NOT count as waived
		},
	}
	req := "Requirement 1 covers a.go behavior. See a.go and also b.go."
	cov := ComputeIngestCoverage(inv, req)

	if len(cov.Mapped) != 2 {
		t.Errorf("mapped = %v, want [a.go b.go]", cov.Mapped)
	}
	if len(cov.Waived) != 1 || cov.Waived[0] != "c.go" {
		t.Errorf("waived = %v, want [c.go]", cov.Waived)
	}
	if len(cov.Unmapped) != 1 || cov.Unmapped[0] != "d.go" {
		t.Errorf("unmapped = %v, want [d.go] (empty-reason waiver rejected)", cov.Unmapped)
	}
}

// TestGateIngest exercises the gate: off by default, error/warn severity, and
// clean pass when every file is mapped or waived.
func TestGateIngest(t *testing.T) {
	root := t.TempDir()
	slug := "legacy"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	inv := Inventory{Files: []InventoryFile{{Path: "x.go"}, {Path: "y.go"}}}
	b, _ := MarshalInventory(inv)
	if err := AtomicWrite(InventoryPath(root, slug), string(b)); err != nil {
		t.Fatal(err)
	}
	req := "Requirement covers x.go only."

	// Off by default → no-op.
	off := CheckCtx{Root: root, Slug: slug, Cfg: Config{}, ReqMd: &req}
	if v, w := GateIngest(off); len(v) != 0 || len(w) != 0 {
		t.Errorf("off gate produced findings: v=%v w=%v", v, w)
	}

	// error severity → violation naming the unmapped file.
	errCtx := CheckCtx{Root: root, Slug: slug, ReqMd: &req}
	errCtx.Cfg.Gates.Ingest = "error"
	v, _ := GateIngest(errCtx)
	if len(v) != 1 || v[0].Gate != "ingest" {
		t.Fatalf("error gate = %v, want 1 ingest violation", v)
	}

	// warn severity → warning, not violation.
	warnCtx := CheckCtx{Root: root, Slug: slug, ReqMd: &req}
	warnCtx.Cfg.Gates.Ingest = "warn"
	if v, w := GateIngest(warnCtx); len(v) != 0 || len(w) != 1 {
		t.Errorf("warn gate v=%v w=%v, want 0 violations 1 warning", v, w)
	}

	// Full coverage → clean.
	full := "Requirement covers x.go and y.go."
	fullCtx := CheckCtx{Root: root, Slug: slug, ReqMd: &full}
	fullCtx.Cfg.Gates.Ingest = "error"
	if v, _ := GateIngest(fullCtx); len(v) != 0 {
		t.Errorf("full coverage should pass, got %v", v)
	}
}

// FuzzManifestParsers ensures the hostile-input manifest parsers never panic.
func FuzzManifestParsers(f *testing.F) {
	f.Add([]byte("module x\n"))
	f.Add([]byte(`{"name":"y"}`))
	f.Add([]byte("[package]\nname=\"z\""))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = goModModule(data)
		_ = packageJSONName(data)
		_ = tomlPackageName(data)
	})
}
