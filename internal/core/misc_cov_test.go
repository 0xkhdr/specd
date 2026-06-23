package core

import (
	"os"
	"path/filepath"
	"testing"
)

// misc_cov_test.go covers small, previously-0% helpers: mode recommendation
// verdicts, context-manifest loading, file existence helpers, the UI print
// helpers, runtime path derivation, and JSON emission.

func TestContextManifestTools(t *testing.T) {
	if (ContextManifestTools{}).Present() {
		t.Error("empty manifest should not be Present")
	}
	if !(ContextManifestTools{RequiredTools: []string{"specd_check"}}).Present() {
		t.Error("manifest with required tool should be Present")
	}

	root := t.TempDir()
	dir := SpecDir(root, "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Missing manifest → empty policy.
	if LoadContextManifest(root, "demo").Present() {
		t.Error("missing manifest should yield empty policy")
	}
	// Valid manifest → parsed policy.
	manifest := `{"contextManifest":{"requiredTools":["specd_check"],"forbiddenTools":["specd_diff"]}}`
	if err := os.WriteFile(contextManifestPath(root, "demo"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadContextManifest(root, "demo")
	if len(got.RequiredTools) != 1 || got.RequiredTools[0] != "specd_check" || len(got.ForbiddenTools) != 1 {
		t.Fatalf("parsed manifest wrong: %+v", got)
	}
	// Malformed manifest → empty policy (degrade, never error).
	if err := os.WriteFile(contextManifestPath(root, "demo"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if LoadContextManifest(root, "demo").Present() {
		t.Error("malformed manifest should degrade to empty policy")
	}
}

func TestFileHelpers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if FileExists(path) {
		t.Error("file should not exist yet")
	}
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !FileExists(path) {
		t.Error("file should exist now")
	}
	if ReadOrDefault(path, "fb") != "hello" {
		t.Error("ReadOrDefault should return contents")
	}
	if ReadOrDefault(filepath.Join(dir, "missing"), "fb") != "fb" {
		t.Error("ReadOrDefault should return fallback for missing")
	}
	if got := ReadOrNull(path); got == nil || *got != "hello" {
		t.Error("ReadOrNull should return contents")
	}
	if ReadOrNull(filepath.Join(dir, "missing")) != nil {
		t.Error("ReadOrNull should return nil for missing")
	}
}

func TestUIHelpers(t *testing.T) {
	t.Setenv("NO_COLOR", "1") // deterministic, no escape codes
	// These print to stdout/stderr; we exercise them for coverage and to confirm
	// they don't panic.
	Info("info msg")
	Success("ok msg")
	Warn("warn msg")
	Error("err msg")

	t.Setenv("SPECD_JSON", "1")
	if !IsJSONMode() {
		t.Error("SPECD_JSON=1 should be JSON mode")
	}
	Header("title") // suppressed in JSON mode
	Divider()       // suppressed in JSON mode

	t.Setenv("SPECD_JSON", "")
	Header("title")
	Divider()

	if got := toUpper("abc-XYZ"); got != "ABC-XYZ" {
		t.Errorf("toUpper: %q", got)
	}
	if colorize(colorRed, "x") != "x" {
		t.Error("colorize should be a no-op under NO_COLOR")
	}
}

func TestRuntimePaths(t *testing.T) {
	root := t.TempDir()
	rt, err := RuntimeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(rt) != "runtime" {
		t.Errorf("runtime dir: %q", rt)
	}
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := paths.SessionsDir(); err != nil {
		t.Error(err)
	}
	if _, err := paths.ArchivesDir(); err != nil {
		t.Error(err)
	}
	if _, err := paths.ProgramSessionsDir(); err != nil {
		t.Error(err)
	}
	if _, err := paths.ArtifactsDir("abcdef0123456789abcdef0123456789"); err != nil {
		t.Error(err)
	}
	// Invalid session ID rejected.
	if _, err := paths.SessionDir("../escape"); err == nil {
		t.Error("invalid session ID should be rejected")
	}
}

func TestValidateAgentsMD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	// Missing file → not present, no error.
	if ok, err := ValidateAgentsMD(path); ok || err != nil {
		t.Fatalf("missing: ok=%v err=%v", ok, err)
	}
	// No managed markers → not present, no error.
	if err := os.WriteFile(path, []byte("# plain doc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := ValidateAgentsMD(path); ok || err != nil {
		t.Fatalf("no markers: ok=%v err=%v", ok, err)
	}
	// Well-formed managed section → present.
	good := markerBegin() + "\nmanaged\n" + markerEnd() + "\n"
	if err := os.WriteFile(path, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := ValidateAgentsMD(path); !ok || err != nil {
		t.Fatalf("managed: ok=%v err=%v", ok, err)
	}
	// Duplicate begin marker → malformed error.
	bad := markerBegin() + "\n" + markerBegin() + "\n" + markerEnd() + "\n"
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAgentsMD(path); err == nil {
		t.Fatal("duplicate markers should be malformed")
	}
}

func TestSplitCSV(t *testing.T) {
	got := SplitCSV(" a , b ,, — , - , c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("SplitCSV want %v got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SplitCSV[%d] want %q got %q", i, want[i], got[i])
		}
	}
	if len(SplitCSV("")) != 0 {
		t.Error("empty string → no tokens")
	}
}

func TestIsOrchestrationSessionNotFound(t *testing.T) {
	if !IsOrchestrationSessionNotFound(errOrchestrationSessionNotFound) {
		t.Error("sentinel should be recognized")
	}
	if IsOrchestrationSessionNotFound(nil) {
		t.Error("nil is not a not-found error")
	}
	if IsOrchestrationSessionNotFound(os.ErrClosed) {
		t.Error("unrelated error is not not-found")
	}
}

func TestReadRole(t *testing.T) {
	root := t.TempDir()
	if ReadRole(root, "impl") != nil {
		t.Error("missing role → nil")
	}
	if err := os.MkdirAll(RolesDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(RolesDir(root), "impl.md"), []byte("role body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadRole(root, "impl"); got == nil || *got != "role body" {
		t.Errorf("ReadRole returned %v", got)
	}
}

func TestRecommendModeNotFound(t *testing.T) {
	// No spec on disk → not-found error, never a panic.
	if _, err := RecommendMode(t.TempDir(), "nope"); err == nil {
		t.Fatal("RecommendMode on a missing spec should error")
	}
}

func TestPrintJSON(t *testing.T) {
	if err := PrintJSON(map[string]int{"a": 1}); err != nil {
		t.Fatalf("PrintJSON valid: %v", err)
	}
	// Unmarshalable value (channel) → error, not panic.
	if err := PrintJSON(make(chan int)); err == nil {
		t.Error("PrintJSON should error on an unmarshalable value")
	}
}
