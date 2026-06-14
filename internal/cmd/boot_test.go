package cmd_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

func TestBoot_PopulatesArtifacts(t *testing.T) {
	h := th.New(t)
	h.Init()
	if err := os.WriteFile(h.Path("pyproject.toml"), []byte("[project]\nname=\"app\"\ndependencies=[\"fastapi\"]\n[tool.pytest.ini_options]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := h.RunExpect(core.ExitOK, "boot")
	if !strings.Contains(res.Stdout, "verify:   pytest") {
		t.Fatalf("missing verify line:\n%s", res.Stdout)
	}

	// boot.json written and parseable.
	raw, err := os.ReadFile(h.Path(".specd/boot.json"))
	if err != nil {
		t.Fatalf("boot.json not written: %v", err)
	}
	var br core.BootResult
	if err := json.Unmarshal(raw, &br); err != nil {
		t.Fatalf("boot.json invalid: %v", err)
	}
	if br.Verify != "pytest" {
		t.Fatalf("boot.json verify = %q", br.Verify)
	}

	// tech.md gained the managed section, template preserved.
	tech, _ := os.ReadFile(h.Path(".specd/steering/tech.md"))
	if !strings.Contains(string(tech), "SPECD BOOT: BEGIN") {
		t.Fatalf("tech.md missing boot section:\n%s", tech)
	}
	if !strings.Contains(string(tech), "Verify commands") {
		t.Fatalf("tech.md lost template content")
	}

	// config.json defaultVerify updated from template default.
	cfg := core.LoadConfig(h.Root)
	if cfg.DefaultVerify != "pytest" {
		t.Fatalf("defaultVerify = %q, want pytest", cfg.DefaultVerify)
	}
}

func TestBoot_DryRunWritesNothing(t *testing.T) {
	h := th.New(t)
	h.Init()
	os.WriteFile(h.Path("go.mod"), []byte("module x\ngo 1.22\n"), 0o644)

	h.RunExpect(core.ExitOK, "boot", "--dry-run")

	if core.FileExists(h.Path(".specd/boot.json")) {
		t.Fatal("dry-run wrote boot.json")
	}
	tech, _ := os.ReadFile(h.Path(".specd/steering/tech.md"))
	if strings.Contains(string(tech), "SPECD BOOT") {
		t.Fatal("dry-run mutated tech.md")
	}
}

func TestBoot_Idempotent(t *testing.T) {
	h := th.New(t)
	h.Init()
	os.WriteFile(h.Path("go.mod"), []byte("module x\ngo 1.22\n"), 0o644)

	h.RunExpect(core.ExitOK, "boot")
	first, _ := os.ReadFile(h.Path(".specd/steering/tech.md"))

	// Second run: boot.json is skipped (exists), tech.md section unchanged.
	res := h.RunExpect(core.ExitOK, "boot")
	if !strings.Contains(res.Stdout, "skip") {
		t.Fatalf("expected boot.json skip on re-run:\n%s", res.Stdout)
	}
	second, _ := os.ReadFile(h.Path(".specd/steering/tech.md"))
	if string(first) != string(second) {
		t.Fatalf("tech.md not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestBoot_ForceRegenerates(t *testing.T) {
	h := th.New(t)
	h.Init()
	os.WriteFile(h.Path("go.mod"), []byte("module x\ngo 1.22\n"), 0o644)

	h.RunExpect(core.ExitOK, "boot")
	res := h.RunExpect(core.ExitOK, "boot", "--force")
	if !strings.Contains(res.Stdout, "write .specd/boot.json") {
		t.Fatalf("expected --force to rewrite boot.json:\n%s", res.Stdout)
	}
}

func TestBoot_PreservesHandSetVerify(t *testing.T) {
	h := th.New(t)
	h.Init()
	// Hand-edit config.json to a non-default verify.
	os.WriteFile(h.Path(".specd/config.json"), []byte(`{"version":1,"defaultVerify":"make test"}`), 0o644)
	os.WriteFile(h.Path("go.mod"), []byte("module x\ngo 1.22\n"), 0o644)

	res := h.RunExpect(core.ExitOK, "boot")
	if core.LoadConfig(h.Root).DefaultVerify != "make test" {
		t.Fatal("boot clobbered hand-set defaultVerify without --force")
	}
	if !strings.Contains(res.Stdout, "keep config.defaultVerify") {
		t.Fatalf("expected keep note:\n%s", res.Stdout)
	}

	// --force overrides.
	h.RunExpect(core.ExitOK, "boot", "--force")
	if core.LoadConfig(h.Root).DefaultVerify != "go test ./..." {
		t.Fatal("--force did not override defaultVerify")
	}
}

func TestBoot_JSON(t *testing.T) {
	h := th.New(t)
	h.Init()
	os.WriteFile(h.Path("go.mod"), []byte("module x\ngo 1.22\n"), 0o644)

	res := h.RunExpect(core.ExitOK, "boot", "--json")
	var out struct {
		Detection core.BootResult `json:"detection"`
		Actions   []string        `json:"actions"`
		DryRun    bool            `json:"dryRun"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, res.Stdout)
	}
	if out.Detection.Verify != "go test ./..." {
		t.Fatalf("detection.verify = %q", out.Detection.Verify)
	}
}

func TestCheckBoot_Gate(t *testing.T) {
	h := th.New(t)
	h.Init()
	os.WriteFile(h.Path("go.mod"), []byte("module x\ngo 1.22\n"), 0o644)

	// No boot.json yet → not-found.
	h.RunExpect(core.ExitNotFound, "check", "--boot")

	// After boot, gate is green.
	h.RunExpect(core.ExitOK, "boot")
	res := h.RunExpect(core.ExitOK, "check", "--boot")
	if !strings.Contains(res.Stdout, "boot-freshness") {
		t.Fatalf("missing gate output:\n%s", res.Stdout)
	}

	// Introduce drift → gate fails.
	os.WriteFile(h.Path("package.json"), []byte(`{"name":"w"}`), 0o644)
	h.RunExpect(core.ExitGate, "check", "--boot")

	// JSON form reports the gate too.
	jres := h.RunExpect(core.ExitGate, "check", "--boot", "--json")
	if !strings.Contains(jres.Stdout, "\"gate\": \"boot-freshness\"") {
		t.Fatalf("missing json gate:\n%s", jres.Stdout)
	}
}

func TestBoot_NoStack(t *testing.T) {
	h := th.New(t)
	h.Init()
	res := h.RunExpect(core.ExitOK, "boot")
	if !strings.Contains(res.Out(), "no known tech stack") {
		t.Fatalf("expected no-stack warning:\n%s", res.Out())
	}
	if core.FileExists(h.Path(".specd/boot.json")) {
		t.Fatal("wrote boot.json with no detected stack")
	}
}
