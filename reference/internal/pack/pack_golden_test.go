package pack

import (
	"os"
	"path/filepath"
	"testing"
)

// applyToFreshRoot resolves a built-in pack and applies it to a brand-new temp
// root, returning the root and the apply result.
func applyToFreshRoot(t *testing.T, name string, force bool) (string, PackApplyResult) {
	t.Helper()
	p, err := ResolvePack(name, "")
	if err != nil {
		t.Fatalf("ResolvePack(%q): %v", name, err)
	}
	root := t.TempDir()
	res, err := ApplyPack(root, p, force)
	if err != nil {
		t.Fatalf("ApplyPack(%q): %v", name, err)
	}
	return root, res
}

// R2.1: applying the same pack twice to clean targets yields byte-identical
// output — scaffolding is reproducible, never input-order or environment
// dependent.
func TestPackApplyDeterministic(t *testing.T) {
	for _, name := range []string{"minimal", "go-service"} {
		t.Run(name, func(t *testing.T) {
			rootA, resA := applyToFreshRoot(t, name, false)
			rootB, resB := applyToFreshRoot(t, name, false)

			if len(resA.Written) != len(resB.Written) {
				t.Fatalf("written count differs: %d vs %d", len(resA.Written), len(resB.Written))
			}
			for i := range resA.Written {
				if resA.Written[i] != resB.Written[i] {
					t.Errorf("written[%d] order differs: %q vs %q", i, resA.Written[i], resB.Written[i])
				}
			}
			for _, rel := range resA.Written {
				a, err := os.ReadFile(filepath.Join(rootA, filepath.FromSlash(rel)))
				if err != nil {
					t.Fatalf("read A %s: %v", rel, err)
				}
				b, err := os.ReadFile(filepath.Join(rootB, filepath.FromSlash(rel)))
				if err != nil {
					t.Fatalf("read B %s: %v", rel, err)
				}
				if string(a) != string(b) {
					t.Errorf("%s: byte content differs between applies", rel)
				}
			}
		})
	}
}

// R2.3: without --force a pre-existing target is a hard error and nothing is
// overwritten — apply is all-or-nothing, never a silent clobber.
func TestPackApplyForceGuard(t *testing.T) {
	p, err := ResolvePack("minimal", "")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if _, err := ApplyPack(root, p, false); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Mark an already-written file so we can prove it is not clobbered.
	target := filepath.Join(root, filepath.FromSlash(p.Files[0].Path))
	sentinel := "USER EDIT — must survive\n"
	if err := os.WriteFile(target, []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-apply without force: must refuse.
	if _, err := ApplyPack(root, p, false); err == nil {
		t.Fatal("re-apply without force succeeded — overwrote existing files")
	}
	got, _ := os.ReadFile(target)
	if string(got) != sentinel {
		t.Errorf("existing file was modified despite no --force: %q", string(got))
	}

	// With force, the same path is overwritten (the escape hatch still works).
	if _, err := ApplyPack(root, p, true); err != nil {
		t.Fatalf("force apply: %v", err)
	}
}

// R2.2: the embedded packs are enumerable (the engine behind `--list-packs`),
// sorted and addressable by name.
func TestBuiltinPacksList(t *testing.T) {
	packs, err := BuiltinPacks()
	if err != nil {
		t.Fatalf("BuiltinPacks: %v", err)
	}
	have := map[string]bool{}
	for _, p := range packs {
		have[p.Name] = true
	}
	for _, want := range []string{"minimal", "go-service"} {
		if !have[want] {
			t.Errorf("--list-packs missing built-in %q", want)
		}
	}
	for i := 1; i < len(packs); i++ {
		if packs[i-1].Name > packs[i].Name {
			t.Errorf("packs not sorted: %q before %q", packs[i-1].Name, packs[i].Name)
		}
	}
}
