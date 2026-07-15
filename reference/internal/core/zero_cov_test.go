package core

import (
	"os"
	"testing"
)

// zero_cov_test.go closes the small previously-0% helpers the larger fixture
// tests never touch: the pre-spec preflight, the task-view merge, the verbatim
// prompt injector, the latest-midreq parser, the artifact-reader closure, the
// root locator, the optional-backend registry, and the backtick wrapper. All
// pure or thin IO — single-call coverage, no production changes.

func TestOrchestrationPreflightStages(t *testing.T) {
	root := t.TempDir()

	// Bare repo: workspace not initialized + spec missing.
	items := OrchestrationPreflight(root, "demo")
	if len(items) != 2 || items[0].Kind != "workspace" || items[1].Kind != "spec" {
		t.Fatalf("bare repo preflight = %#v", items)
	}

	// Workspace exists but steering removed → steering repair + spec missing.
	if err := os.MkdirAll(SpecdDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	items = OrchestrationPreflight(root, "demo")
	if len(items) != 2 || items[0].Kind != "steering" || items[1].Kind != "spec" {
		t.Fatalf("missing-steering preflight = %#v", items)
	}

	// Steering present + spec scaffolded → nothing missing.
	if err := os.MkdirAll(SteeringDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	scaffoldSpec(t, root, "demo", StatusExecuting)
	if items := OrchestrationPreflight(root, "demo"); len(items) != 0 {
		t.Fatalf("ready repo preflight = %#v, want empty", items)
	}
}

func TestRequireSpecdRoot(t *testing.T) {
	// Save and restore cwd; go 1.22 has no t.Chdir.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	// No .specd anywhere up-tree → NotFound.
	bare := t.TempDir()
	if err := os.Chdir(bare); err != nil {
		t.Fatal(err)
	}
	if _, err := RequireSpecdRoot(); err == nil {
		t.Fatal("RequireSpecdRoot without .specd should error")
	}

	// .specd present → resolves to that root.
	rooted := t.TempDir()
	if err := os.MkdirAll(SpecdDir(rooted), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(rooted); err != nil {
		t.Fatal(err)
	}
	got, err := RequireSpecdRoot()
	if err != nil {
		t.Fatalf("RequireSpecdRoot: %v", err)
	}
	// Resolve symlinks on both sides (macOS /var → /private/var, etc.).
	wantInfo, _ := os.Stat(SpecdDir(rooted))
	gotInfo, _ := os.Stat(SpecdDir(got))
	if !os.SameFile(wantInfo, gotInfo) {
		t.Fatalf("RequireSpecdRoot = %q, want root of %q", got, rooted)
	}
}

func TestInjectPrompt(t *testing.T) {
	// Empty prompt is byte-identical (no-`--from` path).
	if got := InjectPrompt("body", "   "); got != "body" {
		t.Fatalf("empty prompt mutated body: %q", got)
	}

	// With a `## Requirement ` marker → block inserted before it.
	withMarker := "# Reqs\n\n## Requirement 1\ndo a thing\n"
	got := InjectPrompt(withMarker, "make it fast")
	idxBlock := indexOf(got, "## Originating prompt")
	idxReq := indexOf(got, "## Requirement 1")
	if idxBlock == -1 || idxReq == -1 || idxBlock > idxReq {
		t.Fatalf("block not placed before requirement: %q", got)
	}

	// No marker → appended at end.
	got = InjectPrompt("# Reqs\nno marker here\n", "line one\nline two")
	if indexOf(got, "## Originating prompt") == -1 || indexOf(got, "> line two") == -1 {
		t.Fatalf("appended block missing: %q", got)
	}
}

func TestLatestMidreq(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "demo", StatusExecuting)

	// No artifact → nil.
	if LatestMidreq(root, "demo") != nil {
		t.Fatal("LatestMidreq without artifact should be nil")
	}

	md := "# Mid-requirements\n\n## Turn 1 — impact: minor\n\n## Turn 2 — impact: major\n" +
		"**User input (verbatim):** \"please add retries\"\n"
	if err := os.WriteFile(ArtifactPath(root, "demo", "mid-requirements.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LatestMidreq(root, "demo")
	if got == nil || got.Turn != 2 || got.Impact != "major" || got.Input != "please add retries" {
		t.Fatalf("LatestMidreq = %#v", got)
	}
}

func TestSpecArtifactReader(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "demo", StatusExecuting)
	if err := os.WriteFile(ArtifactPath(root, "demo", "requirements.md"), []byte("reqs"), 0o644); err != nil {
		t.Fatal(err)
	}
	read := SpecArtifactReader(root, "demo")
	if raw, ok := read("requirements.md"); !ok || raw != "reqs" {
		t.Fatalf("reader = (%q,%v), want (reqs,true)", raw, ok)
	}
	if _, ok := read("missing.md"); ok {
		t.Fatal("reader for missing artifact should be ok=false")
	}
}

func TestResolveTaskView(t *testing.T) {
	md := "# Tasks — X\n\n## Wave 2\n- [ ] T1 — doc title\n" +
		"  - why: w\n  - role: auditor\n  - files: a.go\n  - contract: c\n" +
		"  - acceptance: 9.9=TestX\n  - verify: go test ./\n  - depends: —\n  - requirements: 1\n"
	doc, err := ParseTasks(md)
	if err != nil {
		t.Fatalf("ParseTasks: %v", err)
	}
	st := State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Title: "state title", Role: "craftsman", Wave: 1, Depends: []string{"T0"}, Requirements: []int{3}},
		"T9": {ID: "T9", Title: "only state", Role: "craftsman", Wave: 5},
	}}

	// In doc → doc Title/Wave/Meta/role win; Depends/Requirements from state.
	v := ResolveTaskView(doc, &st, "T1")
	if !v.FromDoc || v.Title != "doc title" || v.Wave != 2 || v.Role != "auditor" {
		t.Fatalf("doc merge = %#v", v)
	}
	if len(v.Depends) != 1 || v.Depends[0] != "T0" || len(v.Requirements) != 1 {
		t.Fatalf("state carry-over wrong: %#v", v)
	}

	// Absent from doc → falls back to state entirely.
	v = ResolveTaskView(doc, &st, "T9")
	if v.FromDoc || v.Title != "only state" || v.Role != "craftsman" || v.Wave != 5 {
		t.Fatalf("state fallback = %#v", v)
	}
}

func TestRegisterOptionalBackend(t *testing.T) {
	const name = "covtest"
	t.Cleanup(func() { delete(optionalBackends, name) })

	if _, err := SelectBackend(name); err == nil {
		t.Fatal("unregistered backend should fail closed")
	}
	registerOptionalBackend(name, DefaultBackend)
	if _, err := SelectBackend(name); err != nil {
		t.Fatalf("registered backend should resolve: %v", err)
	}
}

func TestBacktickedHelper(t *testing.T) {
	got := backticked([]string{"a", "b"})
	if len(got) != 2 || got[0] != "`a`" || got[1] != "`b`" {
		t.Fatalf("backticked = %#v", got)
	}
	if len(backticked(nil)) != 0 {
		t.Fatal("backticked(nil) should be empty")
	}
}

// indexOf is a tiny strings.Index wrapper kept local to avoid importing strings
// for a single use in assertions.
func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
