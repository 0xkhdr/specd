package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEffectiveMode(t *testing.T) {
	if got := (State{}).EffectiveMode(); got != ModeBase {
		t.Errorf("empty ExecutionMode → %q, want %q", got, ModeBase)
	}
	if got := (State{ExecutionMode: ModeOrchestrated}).EffectiveMode(); got != ModeOrchestrated {
		t.Errorf("EffectiveMode = %q, want %q", got, ModeOrchestrated)
	}
}

func TestResolveMode(t *testing.T) {
	cases := []struct {
		name       string
		flag       string
		state      *State
		wantMode   string
		wantOrigin string
	}{
		{"flag wins over base spec", ModeOrchestrated, &State{}, ModeOrchestrated, OriginUser},
		{"flag wins over orchestrated spec", ModeBase, &State{ExecutionMode: ModeOrchestrated, ModeOrigin: OriginUser}, ModeBase, OriginUser},
		{"no flag, recorded orchestrated", "", &State{ExecutionMode: ModeOrchestrated, ModeOrigin: OriginRecommended}, ModeOrchestrated, OriginRecommended},
		{"no flag, empty spec defaults base/default", "", &State{}, ModeBase, OriginDefault},
		{"no flag, nil spec defaults base/default", "", nil, ModeBase, OriginDefault},
		{"recorded mode without origin defaults origin", "", &State{ExecutionMode: ModeOrchestrated}, ModeOrchestrated, OriginDefault},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mode, origin := ResolveMode(c.flag, c.state)
			if mode != c.wantMode || origin != c.wantOrigin {
				t.Errorf("ResolveMode(%q, %+v) = (%q,%q), want (%q,%q)", c.flag, c.state, mode, origin, c.wantMode, c.wantOrigin)
			}
		})
	}
}

func TestProjectOrchestrationEnabled(t *testing.T) {
	// No config → capability absent (fail closed).
	root := t.TempDir()
	if err := os.MkdirAll(SpecdDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if ProjectOrchestrationEnabled(root) {
		t.Error("no config.json should mean no orchestration capability")
	}
	// Config with orchestration.enabled → capable.
	cfg := `{"version":1,"orchestration":{"enabled":true}}`
	if err := os.WriteFile(ConfigPath(root), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ProjectOrchestrationEnabled(root) {
		t.Error("orchestration.enabled:true should mean capable")
	}
}

func TestComputeModeSignals(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write minimal planning artifacts so the token estimate is non-zero.
	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		if err := os.WriteFile(filepath.Join(SpecDir(root, slug), name), []byte("# "+name+"\nsome body text"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state := &State{
		Tasks: map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Role: "builder"},
			"T2": {ID: "T2", Wave: 1, Role: "investigator"},
			"T3": {ID: "T3", Wave: 1, Role: "builder"},
			"T4": {ID: "T4", Wave: 2, Role: "verifier"},
		},
	}
	sig := computeModeSignals(root, slug, state)
	if sig.TaskCount != 4 {
		t.Errorf("TaskCount = %d, want 4", sig.TaskCount)
	}
	if sig.MaxWaveWidth != 3 {
		t.Errorf("MaxWaveWidth = %d, want 3 (wave 1)", sig.MaxWaveWidth)
	}
	if sig.DistinctRoles != 3 {
		t.Errorf("DistinctRoles = %d, want 3 (builder/investigator/verifier)", sig.DistinctRoles)
	}
	if sig.EstimatedTokens <= 0 {
		t.Errorf("EstimatedTokens = %d, want > 0", sig.EstimatedTokens)
	}
}

func TestVerdictFromSignals(t *testing.T) {
	cases := []struct {
		name           string
		sig            ModeSignals
		wantRec        string
		wantConfidence string
	}{
		{"pre-tasks neutral", ModeSignals{}, ModeBase, ConfidenceNeutral},
		{"small serial work stays base", ModeSignals{TaskCount: 5, MaxWaveWidth: 2, DistinctRoles: 2}, ModeBase, ConfidenceNeutral},
		{"parallelism alone suggests", ModeSignals{TaskCount: 12, MaxWaveWidth: 4, DistinctRoles: 2}, ModeOrchestrated, ConfidenceSuggest},
		{"roles alone suggests", ModeSignals{TaskCount: 6, MaxWaveWidth: 2, DistinctRoles: 3}, ModeOrchestrated, ConfidenceSuggest},
		{"two payoffs strong", ModeSignals{TaskCount: 12, MaxWaveWidth: 4, DistinctRoles: 3}, ModeOrchestrated, ConfidenceStrong},
		{"cross-spec edge alone suggests", ModeSignals{TaskCount: 4, MaxWaveWidth: 1, DistinctRoles: 1, CrossSpecEdges: 2}, ModeOrchestrated, ConfidenceSuggest},
		{"tasks>=10 but narrow waves stays base", ModeSignals{TaskCount: 20, MaxWaveWidth: 2, DistinctRoles: 2}, ModeBase, ConfidenceNeutral},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := verdictFromSignals(c.sig)
			if rec.Recommended != c.wantRec || rec.Confidence != c.wantConfidence {
				t.Errorf("verdict = (%q,%q), want (%q,%q); rationale=%q", rec.Recommended, rec.Confidence, c.wantRec, c.wantConfidence, rec.Rationale)
			}
			if !rec.UserDecides {
				t.Error("UserDecides must always be true")
			}
		})
	}
}

// TestVerdictDeterminism: identical signals always yield a byte-identical
// verdict — the property that makes the recommendation reproducible across hosts.
func TestVerdictDeterminism(t *testing.T) {
	sig := ModeSignals{TaskCount: 23, MaxWaveWidth: 6, DistinctRoles: 4, CrossSpecEdges: 2, EstimatedTokens: 41000}
	first := verdictFromSignals(sig)
	for i := 0; i < 100; i++ {
		if got := verdictFromSignals(sig); !reflect.DeepEqual(got, first) {
			t.Fatalf("non-deterministic verdict on run %d: %+v != %+v", i, got, first)
		}
	}
}
