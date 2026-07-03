package core

import (
	"path/filepath"
	"testing"
)

func TestMissionPathDerivation(t *testing.T) {
	root := t.TempDir()
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}

	missionsDir, err := paths.MissionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, ".specd", "runtime", "missions"); missionsDir != want {
		t.Fatalf("MissionsDir = %q, want %q", missionsDir, want)
	}

	got, err := paths.MissionPath("auth-flow", "T1", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".specd", "runtime", "missions", "auth-flow-T1-2.json")
	if got != want {
		t.Fatalf("MissionPath = %q, want %q", got, want)
	}
}

func TestMissionPathDeterministic(t *testing.T) {
	paths, err := NewACPRuntimePaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := paths.MissionPath("spec-x", "T10", 3)
	if err != nil {
		t.Fatal(err)
	}
	b, err := paths.MissionPath("spec-x", "T10", 3)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("MissionPath not deterministic: %q != %q", a, b)
	}
	// A different spec or attempt must never share the filename.
	other, err := paths.MissionPath("spec-y", "T10", 3)
	if err != nil {
		t.Fatal(err)
	}
	if other == a {
		t.Fatalf("distinct specs collided on %q", a)
	}
}

func TestMissionPathRejectsInvalidInput(t *testing.T) {
	paths, err := NewACPRuntimePaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		slug    string
		taskID  string
		attempt int
	}{
		{"bad slug traversal", "../escape", "T1", 1},
		{"bad slug upper", "Spec", "T1", 1},
		{"empty slug", "", "T1", 1},
		{"lowercase task id", "spec", "t1", 1},
		{"task id traversal", "spec", "../x", 1},
		{"task id no number", "spec", "T", 1},
		{"task id wrong prefix", "spec", "B1", 1},
		{"zero attempt", "spec", "T1", 0},
		{"negative attempt", "spec", "T1", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := paths.MissionPath(tc.slug, tc.taskID, tc.attempt); err == nil {
				t.Fatalf("MissionPath(%q,%q,%d) accepted invalid input", tc.slug, tc.taskID, tc.attempt)
			}
		})
	}
}
