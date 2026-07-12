package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModeSchema(t *testing.T) {
	for _, mode := range []Mode{ModeDefault, ModeAgent, ModeOrchestrated} {
		state := InitialState("demo")
		state.Mode = mode
		if err := state.Validate(); err != nil {
			t.Fatalf("mode %q rejected: %v", mode, err)
		}
	}

	state := InitialState("demo")
	state.Mode = Mode("orchestratd")
	if err := state.Validate(); err == nil || !strings.Contains(err.Error(), "invalid state mode") {
		t.Fatalf("unknown mode error = %v", err)
	}
}

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

func TestAmendmentAppendOnly(t *testing.T) {
	state := InitialState("demo")
	a := Amendment{ChangeID: "chg-1", AffectedIDs: []string{"R2", "R2.1"}, Rationale: "clarify failure path", BeforeDigests: map[string]string{"R2": "before"}, AfterDigests: map[string]string{"R2": "after"}, RequiredRechecks: []string{"design", "tasks"}}
	if err := state.AppendAmendment(a); err != nil {
		t.Fatalf("append amendment: %v", err)
	}
	if err := state.AppendAmendment(a); err != nil {
		t.Fatalf("append second amendment: %v", err)
	}
	if len(state.Records) != 2 {
		t.Fatalf("records = %d, want two append-only records", len(state.Records))
	}
	amendments, err := state.Amendments()
	if err != nil || len(amendments) != 2 {
		t.Fatalf("amendments = %+v, err = %v", amendments, err)
	}
	if amendments[0].ChangeID != "chg-1" || amendments[1].AffectedIDs[1] != "R2.1" {
		t.Fatalf("amendments not round-tripped: %+v", amendments)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := SaveStateCAS(path, 0, state); err != nil {
		t.Fatalf("persist amendment: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("reload amendment: %v", err)
	}
	if got, err := loaded.Amendments(); err != nil || len(got) != 2 {
		t.Fatalf("persisted amendments = %+v, err = %v", got, err)
	}
}

func TestAmendmentValidation(t *testing.T) {
	for _, amendment := range []Amendment{{}, {ChangeID: "x", Rationale: "why", AffectedIDs: []string{"R1"}}} {
		if err := amendment.Validate(); err == nil {
			t.Fatalf("invalid amendment accepted: %+v", amendment)
		}
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
	raw := `{"schema_version":1,"slug":"demo","mode":"agent","status":"requirements","phase":"perceive","revision":1}`
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

func TestSchemaPreflight(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "current", raw: `{"schema_version":2}`},
		{name: "legacy_upgrade", raw: `{"schema_version":1}`},
		{name: "future_unsafe_downgrade", raw: `{"schema_version":99}`, wantErr: "unsafe downgrade"},
		{name: "missing", raw: `{}`, wantErr: "schema_version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := PreflightStateSchema([]byte(tc.raw))
			if tc.wantErr == "" && err != nil {
				t.Fatalf("preflight rejected schema: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("preflight error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
