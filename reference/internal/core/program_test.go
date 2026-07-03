package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// scaffoldSpec creates a spec dir with a valid state.json in the given status.
func scaffoldSpec(t *testing.T, root, slug string, status SpecStatus) {
	t.Helper()
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	st := InitialState(slug, slug)
	st.Status = status
	if err := SaveState(root, slug, &st); err != nil {
		t.Fatalf("scaffold %s: %v", slug, err)
	}
}

func TestLoadProgramRoundTrip(t *testing.T) {
	root := t.TempDir()

	manifest := ProgramManifest{
		Version: ProgramVersion,
		DependsOn: map[string][]string{
			"b": {"a", "a", "a"}, // duplicates to be deduped
			"c": {"b"},
			"d": {}, // empty edge to be pruned on save
		},
	}
	if err := SaveProgram(root, manifest); err != nil {
		t.Fatalf("SaveProgram: %v", err)
	}

	got, err := LoadProgram(root)
	if err != nil {
		t.Fatalf("LoadProgram: %v", err)
	}

	// Empty-edge pruning: "d" must not survive the save.
	if _, ok := got.DependsOn["d"]; ok {
		t.Errorf("empty edge 'd' was not pruned: %v", got.DependsOn)
	}
	// Duplicate-edge dedup.
	if deps := got.DependsOn["b"]; !reflect.DeepEqual(deps, []string{"a"}) {
		t.Errorf("dedup failed: b deps = %v, want [a]", deps)
	}
	if deps := got.DependsOn["c"]; !reflect.DeepEqual(deps, []string{"b"}) {
		t.Errorf("c deps = %v, want [b]", deps)
	}

	// Save→load stability: re-saving the loaded manifest yields identical bytes.
	if err := SaveProgram(root, got); err != nil {
		t.Fatalf("re-SaveProgram: %v", err)
	}
	first, _ := os.ReadFile(ProgramPath(root))
	again, err := LoadProgram(root)
	if err != nil {
		t.Fatalf("re-LoadProgram: %v", err)
	}
	if !reflect.DeepEqual(got.DependsOn, again.DependsOn) {
		t.Errorf("not stable: %v != %v", got.DependsOn, again.DependsOn)
	}
	if err := SaveProgram(root, again); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(ProgramPath(root))
	if string(first) != string(second) {
		t.Errorf("save not byte-stable:\n%s\n---\n%s", first, second)
	}
}

func TestLoadProgramMissingReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	got, err := LoadProgram(root)
	if err != nil {
		t.Fatalf("LoadProgram on missing: %v", err)
	}
	if got.Version != ProgramVersion || got.DependsOn == nil || len(got.DependsOn) != 0 {
		t.Errorf("missing program.json should yield empty manifest, got %+v", got)
	}
}

func TestLoadProgramCorruptJSON(t *testing.T) {
	root := t.TempDir()
	if err := AtomicWrite(ProgramPath(root), "{not valid json"); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProgram(root)
	if err == nil {
		t.Fatal("expected gate error on corrupt program.json, got nil")
	}
	if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
		t.Errorf("want GateError, got %v", err)
	}
}

func TestBuildProgram(t *testing.T) {
	t.Run("builds_graph_with_waves", func(t *testing.T) {
		root := t.TempDir()
		scaffoldSpec(t, root, "a", StatusComplete)
		scaffoldSpec(t, root, "b", StatusRequirements)
		if err := SaveProgram(root, ProgramManifest{
			Version:   ProgramVersion,
			DependsOn: map[string][]string{"b": {"a"}},
		}); err != nil {
			t.Fatal(err)
		}

		g, err := BuildProgram(root, nil)
		if err != nil {
			t.Fatalf("BuildProgram: %v", err)
		}
		if len(g.Specs) != 2 {
			t.Fatalf("got %d specs, want 2", len(g.Specs))
		}
		byID := map[string]SpecNode{}
		for _, s := range g.Specs {
			byID[s.Slug] = s
		}
		if byID["a"].Wave != 1 || byID["b"].Wave != 2 {
			t.Errorf("waves wrong: a=%d b=%d, want 1,2", byID["a"].Wave, byID["b"].Wave)
		}
		if !byID["a"].Complete {
			t.Error("spec a should be complete")
		}
	})

	t.Run("missing_state_json_is_gate_error_not_panic", func(t *testing.T) {
		// Regression for the TOCTOU nil-deref (R1.1): ListSpecs lists the spec
		// (its state.json passes os.Stat) but LoadState reads nil. We simulate the
		// concurrent-delete race deterministically by replacing state.json with a
		// directory: Stat still succeeds, ReadFile fails → LoadState returns nil.
		root := t.TempDir()
		scaffoldSpec(t, root, "a", StatusRequirements)
		statePath := filepath.Join(SpecDir(root, "a"), "state.json")
		if err := os.Remove(statePath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(statePath, 0o755); err != nil {
			t.Fatal(err)
		}

		_, err := BuildProgram(root, nil) // must not panic
		if err == nil {
			t.Fatal("expected gate error for vanished state.json, got nil")
		}
		if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
			t.Errorf("want GateError, got %v", err)
		}
	})
}
