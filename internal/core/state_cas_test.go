package core

import (
	"os"
	"testing"
)

// specRoot returns a temp project root with an empty spec directory ready for
// state.json.
func specRoot(t *testing.T, slug string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSaveStateBumpsRevision(t *testing.T) {
	// Arrange
	root := specRoot(t, "s")
	st := InitialState("s", "S")

	// Act
	if err := SaveState(root, "s", &st); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Assert: first write moves revision 0 -> 1 and updates timestamp.
	if st.Revision != 1 {
		t.Errorf("revision = %d, want 1", st.Revision)
	}
	got, err := LoadState(root, "s")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.Revision != 1 {
		t.Errorf("persisted revision = %d, want 1", got.Revision)
	}
}

func TestSaveStateDetectsConcurrentWrite(t *testing.T) {
	// Arrange: persist once, then a second writer commits underfoot.
	root := specRoot(t, "s")
	st := InitialState("s", "S")
	if err := SaveState(root, "s", &st); err != nil {
		t.Fatal(err)
	}
	// Simulate a concurrent agent: load, save, advancing on-disk revision.
	other, _ := LoadState(root, "s")
	if err := SaveState(root, "s", other); err != nil {
		t.Fatal(err)
	}

	// Act: our stale handle (revision 1) tries to save over revision 2.
	err := SaveState(root, "s", &st)

	// Assert: CAS rejects with a gate error, no clobber.
	if err == nil {
		t.Fatal("SaveState = nil, want concurrent-write gate error")
	}
	if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
		t.Errorf("err = %v, want gate error", err)
	}
}

func TestLoadStateMissingReturnsNil(t *testing.T) {
	root := specRoot(t, "s")
	got, err := LoadState(root, "s")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("state = %v, want nil for missing file", got)
	}
}

func TestLoadStateRejectsCorruptJSON(t *testing.T) {
	root := specRoot(t, "s")
	if err := os.WriteFile(statePath(root, "s"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(root, "s")
	if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
		t.Errorf("err = %v, want gate error on corrupt json", err)
	}
}

func TestLoadStateRejectsMissingRequiredFields(t *testing.T) {
	root := specRoot(t, "s")
	// Valid JSON object but no "spec" field.
	if err := os.WriteFile(statePath(root, "s"), []byte(`{"title":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(root, "s")
	if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
		t.Errorf("err = %v, want gate error on malformed state", err)
	}
}

func TestLoadStateRejectsNewerSchema(t *testing.T) {
	root := specRoot(t, "s")
	future := SchemaVersion + 1
	data := []byte(`{"spec":"s","schemaVersion":` + itoa(future) + `}`)
	if err := os.WriteFile(statePath(root, "s"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(root, "s")
	if se, ok := IsSpecdError(err); !ok || se.Code != ExitGate {
		t.Errorf("err = %v, want gate error on newer schema", err)
	}
}

func TestLoadStateNormalizesNilMaps(t *testing.T) {
	root := specRoot(t, "s")
	if err := os.WriteFile(statePath(root, "s"), []byte(`{"spec":"s"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(root, "s")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.Tasks == nil {
		t.Error("Tasks is nil, want empty map")
	}
	if st.Blockers == nil {
		t.Error("Blockers is nil, want empty slice")
	}
}

// itoa avoids pulling strconv into the test for a single small int.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
