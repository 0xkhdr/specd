package context

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestSelectorRequiredLanes(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		".specd/specs/demo/requirements.md": "# Requirements\n",
		".specd/specs/demo/design.md":       "# Design\n",
		".specd/roles/craftsman.md":         "# Role\n",
		"internal/a.go":                     "package internal\n",
		"internal/a_test.go":                "package internal\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	task := core.TaskRow{ID: "T7", Role: "craftsman", DeclaredFiles: []string{"internal/a.go", "internal/a_test.go"}, Verify: "go test ./...", Acceptance: "R2.1"}
	items, err := SelectRequiredLanes(root, "demo", task)
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []string{"design", "requirements", "role", "source", "task", "test"}
	if len(items) != len(wantKinds) {
		t.Fatalf("items = %+v", items)
	}
	for i, want := range wantKinds {
		if items[i].Kind != want || !items[i].Required || items[i].SourceDigest == "" {
			t.Fatalf("item %d = %+v, want required %s with digest", i, items[i], want)
		}
		wantTrust := ContentTrustUntrustedData
		if items[i].ContentTrust != wantTrust {
			t.Errorf("item %s content trust = %q, want %q", want, items[i].ContentTrust, wantTrust)
		}
	}
}

func TestSelectorGreenfieldDeclaredFile(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		".specd/specs/demo/requirements.md": "# Requirements\n",
		".specd/specs/demo/design.md":       "# Design\n",
		".specd/roles/craftsman.md":         "# Role\n",
		"internal/existing.go":              "package internal\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// One existing output, one that does not exist yet (the task will create it).
	task := core.TaskRow{ID: "T3", Role: "craftsman", DeclaredFiles: []string{"internal/existing.go", "internal/new/module.go"}, Verify: "go test ./...", Acceptance: "R1"}
	items, err := SelectRequiredLanes(root, "demo", task)
	if err != nil {
		t.Fatalf("greenfield declared file must not fail context: %v", err)
	}
	for _, item := range items {
		if item.Source != "internal/new/module.go" {
			continue
		}
		if item.Required || item.Loaded || item.SourceDigest != "" {
			t.Fatalf("missing output must not appear as a required, loaded, or digested lane: %+v", item)
		}
		if item.Lane != LaneProspectiveOutput {
			t.Fatalf("missing output lane = %q, want %q", item.Lane, LaneProspectiveOutput)
		}
	}
	var loadedExisting bool
	for _, item := range items {
		if item.Source == "internal/existing.go" {
			loadedExisting = true
		}
	}
	if !loadedExisting {
		t.Fatalf("existing declared file must load as a source lane: %+v", items)
	}
}

func TestSelectorDeclaredFileTraversalFails(t *testing.T) {
	root := t.TempDir()
	for name := range map[string]string{
		".specd/specs/demo/requirements.md": "r",
		".specd/specs/demo/design.md":       "d",
		".specd/roles/craftsman.md":         "role",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	task := core.TaskRow{ID: "T9", Role: "craftsman", DeclaredFiles: []string{"../outside.go"}, Verify: "go test ./...", Acceptance: "R1"}
	if _, err := SelectRequiredLanes(root, "demo", task); err == nil {
		t.Fatal("declared file escaping repository base must fail closed")
	}
}

func TestSelectorNamesMissingRequiredSource(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".specd/specs/demo", ".specd/roles"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{".specd/specs/demo/requirements.md", ".specd/roles/craftsman.md"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("r"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := SelectRequiredLanes(root, "demo", core.TaskRow{ID: "T1", Role: "craftsman"})
	if err == nil || !strings.Contains(err.Error(), ".specd/specs/demo/design.md") || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v", err)
	}
}

// laneRoot builds a minimal spec tree with the three managed policy sources the
// selector always requires, plus any extra files given as path→body.
func laneRoot(t *testing.T, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".specd/specs/demo/requirements.md": "# Requirements\n",
		".specd/specs/demo/design.md":       "# Design\n",
		".specd/roles/craftsman.md":         "# Role\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func laneOf(items []MachineItem, source string) (MachineItem, bool) {
	for _, item := range items {
		if item.Source == source {
			return item, true
		}
	}
	return MachineItem{}, false
}

// TestContextLaneSemantics pins the typed context lanes (spec 05 R2.1–R2.6).
// Each subtest names a required check: a missing declared output stays
// authorized without content, a missing required input fails closed with column,
// path, and recovery, a bare directory is an authoring error, a symlink escape
// never enters a lane, and a terminal task is not swept for context.
func TestContextLaneSemantics(t *testing.T) {
	task := func(files, context string) core.TaskRow {
		row := core.TaskRow{ID: "T1", Role: "craftsman", Files: files, Verify: "go test ./...", Acceptance: "R2.1", Context: context}
		row.DeclaredFiles, _ = declaredFilesFor(t, files)
		return row
	}

	t.Run("missing_output_keeps_authority_without_content", func(t *testing.T) {
		root := laneRoot(t, map[string]string{"pkg/existing.go": "package pkg\n"})
		items, err := SelectRequiredLanes(root, "demo", task("pkg/existing.go,pkg/new.go", ""))
		if err != nil {
			t.Fatalf("greenfield output failed context: %v", err)
		}
		missing, ok := laneOf(items, "pkg/new.go")
		if !ok {
			t.Fatalf("prospective output lane dropped entirely: %+v", items)
		}
		if missing.Lane != LaneProspectiveOutput || missing.Existence != ExistenceAbsent {
			t.Fatalf("prospective lane = %+v", missing)
		}
		if missing.Required || missing.Loaded || missing.SourceDigest != "" || missing.EstimatedTokens != 0 {
			t.Fatalf("prospective output must carry no content and no cost: %+v", missing)
		}
		existing, ok := laneOf(items, "pkg/existing.go")
		if !ok || existing.Lane != LaneOptionalExistingOutput || !existing.Loaded || existing.SourceDigest == "" {
			t.Fatalf("existing output lane = %+v (ok=%t)", existing, ok)
		}
	})

	t.Run("missing_input_fails_with_column_path_and_recovery", func(t *testing.T) {
		root := laneRoot(t, nil)
		_, err := SelectRequiredLanes(root, "demo", task("pkg/new.go", "docs/absent.md"))
		if err == nil {
			t.Fatal("missing required input was accepted")
		}
		for _, want := range []string{"CONTEXT_REQUIRED_INPUT_MISSING", "task T1", "column context", `"docs/absent.md"`, "remove it from the context column"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error missing %q: %v", want, err)
			}
		}
		// The same path in the files column is authority, not an input: it must
		// not fail. This is the distinction the whole lane model exists for.
		if _, err := SelectRequiredLanes(root, "demo", task("docs/absent.md", "")); err != nil {
			t.Fatalf("declared output with the same missing path must not fail: %v", err)
		}
	})

	t.Run("bare_directory_requires_a_bounded_selector", func(t *testing.T) {
		root := laneRoot(t, map[string]string{"pkg/a.go": "package pkg\n", "pkg/b.go": "package pkg\n", "pkg/sub/c.go": "package sub\n"})
		_, err := SelectRequiredLanes(root, "demo", task("pkg/new.go", "pkg"))
		if err == nil {
			t.Fatal("bare directory was accepted as a context source")
		}
		for _, want := range []string{"CONTEXT_BARE_DIRECTORY", "column context", "pkg/*.go"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error missing %q: %v", want, err)
			}
		}

		// A bounded selector is legal and expands deterministically.
		items, err := SelectRequiredLanes(root, "demo", task("pkg/new.go", "pkg/*.go"))
		if err != nil {
			t.Fatalf("bounded selector refused: %v", err)
		}
		var matched []string
		for _, item := range items {
			if item.Lane == LaneDirectoryQuery {
				if item.Selector != "pkg/*.go" || !item.Loaded {
					t.Fatalf("directory query item = %+v", item)
				}
				matched = append(matched, item.Source)
			}
		}
		if !reflect.DeepEqual(matched, []string{"pkg/a.go", "pkg/b.go"}) {
			t.Fatalf("selector matches = %v, want a.go and b.go in sorted order (no recursion)", matched)
		}
		recursive, err := SelectRequiredLanes(root, "demo", task("pkg/new.go", "pkg/**/*.go"))
		if err != nil {
			t.Fatalf("recursive selector refused: %v", err)
		}
		if _, ok := laneOf(recursive, "pkg/sub/c.go"); !ok {
			t.Fatal("recursive selector did not reach pkg/sub/c.go")
		}
		// A selector that matches nothing is an authoring error, not an empty lane.
		if _, err := SelectRequiredLanes(root, "demo", task("pkg/new.go", "pkg/*.rs")); err == nil ||
			!strings.Contains(err.Error(), "CONTEXT_QUERY_EMPTY") {
			t.Fatalf("empty selector error = %v", err)
		}
	})

	t.Run("symlink_escape_never_enters_a_lane", func(t *testing.T) {
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		root := laneRoot(t, map[string]string{"pkg/keep.go": "package pkg\n"})
		if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "pkg", "escape.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		for _, context := range []string{"pkg/escape.md", "pkg/*.md", "../outside.md"} {
			if _, err := SelectRequiredLanes(root, "demo", task("pkg/new.go", context)); err == nil {
				t.Fatalf("context %q escaped the repository base without failing", context)
			}
		}
		if _, err := SelectRequiredLanes(root, "demo", task("pkg/escape.md", "")); err == nil {
			t.Fatal("declared file escaping via symlink must still fail closed")
		}
	})

	t.Run("terminal_task_is_not_swept_for_context", func(t *testing.T) {
		for _, status := range []core.TaskRunStatus{core.TaskComplete} {
			if SelectableForContext(status) {
				t.Errorf("terminal status %q is still swept for context", status)
			}
		}
		for _, status := range []core.TaskRunStatus{core.TaskPending, core.TaskRunning, core.TaskBlocked, ""} {
			if !SelectableForContext(status) {
				t.Errorf("active status %q was skipped by the context sweep", status)
			}
		}
		// A terminal task remains explicitly selectable for reopen/revalidation:
		// the predicate governs the sweep, never direct selection.
		root := laneRoot(t, map[string]string{"pkg/done.go": "package pkg\n"})
		if _, err := SelectRequiredLanes(root, "demo", task("pkg/done.go", "")); err != nil {
			t.Fatalf("explicit selection of a completed task's context failed: %v", err)
		}
	})

	t.Run("budget_counts_only_required_and_loaded_lanes", func(t *testing.T) {
		prospective := MachineItem{Kind: "source", Source: "new.go", Lane: LaneProspectiveOutput, Existence: ExistenceAbsent, Reason: "prospective", EstimatedTokens: 5000}
		required := MachineItem{Kind: "task", Required: true, Loaded: true, Lane: LaneManagedPolicy, Reason: "task", EstimatedTokens: 100}
		optional := MachineItem{Kind: "memory", Source: "m1", Priority: 9, Loaded: true, Reason: "memory", EstimatedTokens: 30}
		if CountsAgainstBudget(prospective) {
			t.Fatal("a prospective output must not count against the budget")
		}
		kept, omissions, req, opt, err := EnforceMachineBudget([]MachineItem{required, prospective, optional}, 100)
		if err != nil {
			t.Fatalf("prospective tokens were charged to the budget: %v", err)
		}
		if req != 100 || opt != 0 {
			t.Fatalf("required=%d optional=%d", req, opt)
		}
		if len(omissions) != 1 || omissions[0].Source != "m1" {
			t.Fatalf("expected only the memory item to shed: %+v", omissions)
		}
		if _, ok := laneOf(kept, "new.go"); !ok {
			t.Fatal("budget pressure shed a prospective output and revoked its write authority")
		}
	})

	t.Run("assurance_is_advisory_without_proven_containment", func(t *testing.T) {
		root := laneRoot(t, map[string]string{"pkg/a.go": "package pkg\n"})
		tasks := []core.TaskRow{task("pkg/a.go,pkg/new.go", "")}
		m, err := BuildMachineManifest(root, "demo", tasks, "T1", "context", "execute", 0, core.BootstrapHandshake(core.Config{}))
		if err != nil {
			t.Fatalf("BuildMachineManifest: %v", err)
		}
		if m.Assurance != string(core.AssuranceAdvisory) {
			t.Fatalf("assurance = %q, want advisory when host containment is unproven", m.Assurance)
		}
		if _, ok := laneOf(m.Items, "pkg/new.go"); !ok {
			t.Fatalf("prospective lane missing from the built manifest: %+v", m.Items)
		}
		if err := ValidateMachineManifest(m); err != nil {
			t.Fatalf("built manifest fails its own validator: %v", err)
		}
	})
}

// declaredFilesFor runs a raw files cell through the canonical task contract so
// the tests exercise the same normalization the parser gives real rows.
func declaredFilesFor(t *testing.T, files string) ([]string, error) {
	t.Helper()
	contract, err := core.ParseTaskContract(core.TaskRow{ID: "T1", Files: files})
	if err != nil {
		return nil, err
	}
	return contract.OutputPaths, nil
}
