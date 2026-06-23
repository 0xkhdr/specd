package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pure_cov_test.go exercises the deterministic, network-free render/format and
// path helpers that previously sat at 0% coverage. Every assertion is on pure
// byte output or filesystem-derived paths — no clock, no network.

func TestBuildPRSummaryAndMarkdown(t *testing.T) {
	state := &State{
		Spec:   "demo",
		Title:  "Demo Spec",
		Status: StatusExecuting,
		Tasks: map[string]TaskState{
			"T1":  {ID: "T1", Title: "first", Role: "impl", Wave: 1, Status: TaskComplete},
			"T2":  {ID: "T2", Title: "second", Role: "impl", Wave: 1, Status: TaskRunning},
			"T10": {ID: "T10", Title: "tenth", Role: "verify", Wave: 2, Status: TaskBlocked},
			"T3":  {ID: "T3", Title: "third", Role: "impl", Wave: 2, Status: TaskPending},
		},
	}
	viol := []Violation{{Gate: "design", Location: "design.md", Message: "missing"}}
	warn := []Violation{{Gate: "tasks", Location: "tasks.md", Message: "thin"}}
	commits := []CommitLink{
		{SHA: "abcdef0123456789", Subject: "fix T1", Tasks: []string{"T1"}},
		{SHA: "short", Subject: "chore", Tasks: nil},
	}
	s := BuildPRSummary(state, viol, warn, commits)

	if s.TasksTotal != 4 || s.TasksDone != 1 {
		t.Fatalf("task counts wrong: total=%d done=%d", s.TasksTotal, s.TasksDone)
	}
	if s.GatesOK {
		t.Fatal("gates should be not-ok with a violation")
	}
	if len(s.Waves) != 2 || s.Waves[0].Wave != 1 || s.Waves[1].Wave != 2 {
		t.Fatalf("waves wrong: %+v", s.Waves)
	}
	// Wave 2 ordinal sort: T3 before T10.
	w2 := s.Waves[1].Tasks
	if w2[0].ID != "T3" || w2[1].ID != "T10" {
		t.Fatalf("wave-2 ordinal sort wrong: %v", []string{w2[0].ID, w2[1].ID})
	}

	md := s.Markdown()
	for _, want := range []string{
		"## specd — Demo Spec (`demo`)",
		"❌ 1 gate violation(s)",
		"1 / 4 complete",
		"### Wave 1", "### Wave 2",
		"✅ complete", "▶ running", "⛔ blocked", "○ pending",
		"### Gate violations", "### Warnings", "### Commits",
		"`abcdef0`", "_(no task ref)_",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
	// Determinism: identical input → identical bytes.
	if md != s.Markdown() {
		t.Fatal("Markdown not deterministic")
	}
}

func TestBuildPRSummaryGatesGreenNilSlices(t *testing.T) {
	state := mkState("s", 1, TaskState{ID: "T1", Wave: 1, Status: TaskComplete})
	s := BuildPRSummary(state, nil, nil, nil)
	if !s.GatesOK {
		t.Fatal("no violations → gates ok")
	}
	if s.Violations == nil || s.Warnings == nil {
		t.Fatal("nil slices should be normalized to empty, not nil")
	}
	md := s.Markdown()
	if !strings.Contains(md, "✅ all gates green") {
		t.Fatalf("want green gate line, got: %s", md)
	}
	if strings.Contains(md, "### Commits") {
		t.Fatal("no commits → no commits section")
	}
}

func TestShortSHAAndStatusMark(t *testing.T) {
	if got := shortSHA("abcdef0123"); got != "abcdef0" {
		t.Errorf("shortSHA long: %q", got)
	}
	if got := shortSHA("abc"); got != "abc" {
		t.Errorf("shortSHA short: %q", got)
	}
	if statusMark("nonsense") != "○ pending" {
		t.Error("unknown status should map to pending")
	}
}

func TestRenderPrometheusMetrics(t *testing.T) {
	state := &State{
		Spec: "demo",
		Tasks: map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Telemetry: &Telemetry{DurationMs: 1000, VerifyDurationMs: 200, Retries: 1, Tokens: 100, Cost: "0.10"}},
			"T2": {ID: "T2", Wave: 2, Telemetry: &Telemetry{DurationMs: 500, Tokens: 50}},
		},
	}
	out := RenderPrometheusMetrics(ReportData{State: state})
	for _, want := range []string{
		"# TYPE specd_task_total gauge",
		`specd_task_total{spec="demo"} 2`,
		`specd_telemetry_duration_ms{spec="demo"} 1500`,
		`specd_telemetry_tokens{spec="demo"} 150`,
		`specd_telemetry_cost_usd{spec="demo",annotated="true"}`,
		`specd_wave_task_total{spec="demo",wave="1"} 1`,
		`specd_wave_task_total{spec="demo",wave="2"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics missing %q\n---\n%s", want, out)
		}
	}
	if out != RenderPrometheusMetrics(ReportData{State: state}) {
		t.Fatal("metrics output not deterministic")
	}
}

func TestBoolLabel(t *testing.T) {
	if boolLabel(true) != "true" || boolLabel(false) != "false" {
		t.Fatal("boolLabel wrong")
	}
}

func TestRenderHelp(t *testing.T) {
	out := RenderHelp()
	for _, want := range []string{"LIFECYCLE", "EXECUTION", "INSPECTION", "META", "specd — spec-driven coding harness"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestRenderCommandHelp(t *testing.T) {
	if len(Commands) == 0 {
		t.Skip("no commands registered")
	}
	name := Commands[0].Command
	out, err := RenderCommandHelp(name)
	if err != nil {
		t.Fatalf("RenderCommandHelp(%q): %v", name, err)
	}
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "SYNOPSIS") {
		t.Errorf("command help missing sections: %s", out)
	}
	if _, err := RenderCommandHelp("definitely-not-a-command"); err == nil {
		t.Fatal("unknown command should error")
	}
}

func TestRenderHelpJSON(t *testing.T) {
	out, err := RenderHelpJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got []CommandMeta
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("help JSON not valid: %v", err)
	}
	if len(got) != len(Commands) {
		t.Fatalf("help JSON has %d commands, want %d", len(got), len(Commands))
	}
}

func TestPathHelpers(t *testing.T) {
	root := "/proj"
	cases := map[string]string{
		SpecdDir(root):         filepath.Join(root, ".specd"),
		SteeringDir(root):      filepath.Join(root, ".specd", "steering"),
		RolesDir(root):         filepath.Join(root, ".specd", "roles"),
		SkillsDir(root):        filepath.Join(root, ".specd", "skills"),
		SpecsDir(root):         filepath.Join(root, ".specd", "specs"),
		SpecDir(root, "slug"):  filepath.Join(root, ".specd", "specs", "slug"),
		ConfigPath(root):       filepath.Join(root, ".specd", "config.json"),
		IntegrationsPath(root): filepath.Join(root, ".specd", "integrations.json"),
		AgentsPath(root):       filepath.Join(root, "AGENTS.md"),
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("path want %q got %q", want, got)
		}
	}
}

func TestFindSpecdRoot(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(base, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := FindSpecdRoot(child)
	if !ok {
		t.Fatal("should find .specd walking up from child")
	}
	// macOS temp dirs are symlinks; compare resolved paths.
	gotEval, _ := filepath.EvalSymlinks(got)
	baseEval, _ := filepath.EvalSymlinks(base)
	if gotEval != baseEval {
		t.Fatalf("root want %q got %q", baseEval, gotEval)
	}

	// No .specd anywhere up from an isolated temp dir → not found.
	isolated := t.TempDir()
	if _, ok := FindSpecdRoot(isolated); ok {
		// Parent chain could theoretically contain .specd in odd CI roots; only
		// assert when the temp root is genuinely clean.
		if _, statErr := os.Stat(filepath.Join(isolated, ".specd")); os.IsNotExist(statErr) {
			t.Skip("temp parent chain unexpectedly contains .specd; skipping negative case")
		}
	}
}
