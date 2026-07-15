package contextpkg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestBuildContextHUD asserts the HUD measures each load file from disk, marks
// missing files without inventing cost, totals bytes/tokens, and is byte-stable
// across runs (invariant 6/7).
func TestBuildContextHUD(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("steering/a.md", "hello world")             // 11 bytes
	write("skills/s/SKILL.md", "skill body here yes") // 19 bytes
	load := []string{"steering/a.md", "skills/s/SKILL.md", "steering/missing.md"}
	skills := []string{"skills/s/SKILL.md"}

	hud := BuildContextHUD(root, "demo", "conductor", "tier-2", load, skills)

	if hud.Spec != "demo" || hud.Mode != "conductor" || hud.Tier != "tier-2" {
		t.Fatalf("header wrong: %+v", hud)
	}
	if len(hud.Files) != 3 {
		t.Fatalf("want 3 files, got %d", len(hud.Files))
	}
	if !hud.Files[0].Exists || hud.Files[0].Bytes != 11 {
		t.Fatalf("file0 = %+v, want exists 11 bytes", hud.Files[0])
	}
	if hud.Files[2].Exists || hud.Files[2].Bytes != 0 || hud.Files[2].ApproxTokens != 0 {
		t.Fatalf("missing file must be marked absent with zero cost: %+v", hud.Files[2])
	}
	if hud.TotalBytes != 30 {
		t.Fatalf("total bytes = %d, want 30", hud.TotalBytes)
	}
	if hud.ApproxTokens != EstimateTokens([]byte("hello world"))+EstimateTokens([]byte("skill body here yes")) {
		t.Fatalf("token total not the sum of per-file estimates: %d", hud.ApproxTokens)
	}

	// Determinism: identical inputs produce an identical HUD.
	if !reflect.DeepEqual(BuildContextHUD(root, "demo", "conductor", "tier-2", load, skills), hud) {
		t.Fatalf("HUD is not stable across runs")
	}
}
