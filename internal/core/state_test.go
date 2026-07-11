package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDigestStable(t *testing.T) {
	a := Digest([]byte("hello"))
	b := Digest([]byte("hello"))
	c := Digest([]byte("world"))
	if a != b {
		t.Fatal("Digest is not deterministic")
	}
	if a == c {
		t.Fatal("Digest collided on different content")
	}
	if len(a) != 64 {
		t.Fatalf("sha256 hex length = %d, want 64", len(a))
	}
}

func TestStateRecordDigestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state := InitialState("demo")
	digest := Digest([]byte("### R1 — X\n- R1.1: When a, system shall b.\n"))
	rec := StampRecord(Record{
		Kind:         "approval",
		Gate:         "requirements",
		SourceDigest: digest,
		CriteriaIDs:  []string{"R1.1"},
	}, "abc123")
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	state.Records["approval:requirements"] = raw

	if err := SaveStateCAS(path, 0, state); err != nil {
		t.Fatalf("CAS save: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Revision != 1 {
		t.Fatalf("revision = %d, want 1 (CAS bump)", loaded.Revision)
	}
	var got Record
	if err := json.Unmarshal(loaded.Records["approval:requirements"], &got); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	if got.SourceDigest != digest {
		t.Fatalf("source digest = %q, want %q", got.SourceDigest, digest)
	}
	if len(got.CriteriaIDs) != 1 || got.CriteriaIDs[0] != "R1.1" {
		t.Fatalf("criteria ids = %+v, want [R1.1]", got.CriteriaIDs)
	}
}

func TestStateLoadsLegacyRecordWithoutDigest(t *testing.T) {
	// A record written by an older specd (no source_digest/criteria_ids) must
	// still load — the new fields are additive and omitempty (backward compat).
	path := filepath.Join(t.TempDir(), "state.json")
	raw := `{"schema_version":2,"slug":"demo","mode":"default","status":"requirements","phase":"perceive","revision":1,` +
		`"records":{"approval:requirements":{"kind":"approval","gate":"requirements","timestamp":"t","git_head":"h","actor":"a"}}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}
	st, err := LoadState(path)
	if err != nil {
		t.Fatalf("legacy record should load: %v", err)
	}
	var rec Record
	if err := json.Unmarshal(st.Records["approval:requirements"], &rec); err != nil {
		t.Fatalf("unmarshal legacy record: %v", err)
	}
	if rec.SourceDigest != "" || rec.CriteriaIDs != nil {
		t.Fatalf("legacy record gained fields: %+v", rec)
	}
}

func TestStateCAS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state := InitialState("demo")

	if err := SaveStateCAS(path, 0, state); err != nil {
		t.Fatalf("initial CAS save: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.Revision != 1 {
		t.Fatalf("revision = %d, want 1", loaded.Revision)
	}

	loaded.Status = StatusDesign
	loaded.Phase = PhaseForStatus(StatusDesign)
	if err := SaveStateCAS(path, 1, loaded); err != nil {
		t.Fatalf("second CAS save: %v", err)
	}
	loaded, err = LoadState(path)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if loaded.Revision != 2 {
		t.Fatalf("revision = %d, want 2", loaded.Revision)
	}

	if err := SaveStateCAS(path, 1, loaded); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale CAS error = %v, want ErrRevisionConflict", err)
	}
}

func TestLoadStateMigratesV1ToCurrentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	raw := `{"schema_version":1,"slug":"demo","mode":"build","status":"requirements","phase":"perceive","revision":1}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	state, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != StateSchemaVersion {
		t.Fatalf("schema version = %d, want %d", state.SchemaVersion, StateSchemaVersion)
	}
}

func TestLoadStateRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	raw := `{"schema_version":2,"slug":"demo","mode":"build","status":"requirements","phase":"perceive","revision":1,"unexpected":true}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState accepted unknown field")
	}
}

func TestStateRejectsInvalidSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0o644); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState accepted unsupported schema")
	}
}
