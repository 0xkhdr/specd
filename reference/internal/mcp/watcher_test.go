package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func phaseCfg() *core.Config {
	cfg := &core.Config{}
	cfg.MCP.Expose = "phase"
	return cfg
}

// toolNameSet collapses a tool list to a name set for subset assertions.
func toolNameSet(tools []toolDef) map[string]bool {
	set := make(map[string]bool, len(tools))
	for _, t := range tools {
		set[t.Name] = true
	}
	return set
}

// TestBuildPhaseToolsSubsets verifies each lifecycle band advertises its own
// subset: planning exposes inspection + gate tools and hides the drive loop;
// executing exposes the drive loop (spec R2/§5.1).
func TestBuildPhaseToolsSubsets(t *testing.T) {
	cfg := phaseCfg()

	planning := toolNameSet(buildPhaseTools(cfg, core.StatusDesign, "architect"))
	if !planning["specd_inspect"] || !planning["specd_check"] || !planning["specd_approve"] {
		t.Errorf("planning subset missing inspection/gate tools: %v", planning)
	}
	if planning["specd_next"] || planning["specd_dispatch"] {
		t.Errorf("planning subset must not expose the drive loop: %v", planning)
	}

	executing := toolNameSet(buildPhaseTools(cfg, core.StatusExecuting, "craftsman"))
	if !executing["specd_next"] || !executing["specd_verify"] || !executing["specd_task"] {
		t.Errorf("executing subset missing drive-loop tools: %v", executing)
	}
	if executing["specd_check"] || executing["specd_approve"] {
		t.Errorf("executing subset must not expose planning gate tools: %v", executing)
	}
}

// TestBuildPhaseToolsUnknownStatusFallsBack confirms an unmapped status yields
// the essential set rather than an empty list (forward-compatibility).
func TestBuildPhaseToolsUnknownStatusFallsBack(t *testing.T) {
	got := toolNameSet(buildPhaseTools(phaseCfg(), core.SpecStatus("future-phase"), "craftsman"))
	for _, want := range []string{"specd_inspect", "specd_verify", "specd_task"} {
		if !got[want] {
			t.Errorf("fallback subset missing %q: %v", want, got)
		}
	}
}

func TestBuildPhaseToolsRoleIntersection(t *testing.T) {
	got := toolNameSet(buildPhaseTools(phaseCfg(), core.StatusExecuting, "validator"))
	for _, want := range []string{"specd_check", "specd_status", "specd_state_read"} {
		if !got[want] {
			t.Fatalf("validator subset missing %s: %v", want, got)
		}
	}
	for _, forbid := range []string{"specd_next", "specd_dispatch", "specd_verify", "specd_task"} {
		if got[forbid] {
			t.Fatalf("validator subset leaked %s: %v", forbid, got)
		}
	}
}

func TestStatusRankOrdering(t *testing.T) {
	if statusRank(core.StatusExecuting) <= statusRank(core.StatusTasks) {
		t.Error("executing should outrank tasks")
	}
	if statusRank(core.StatusTasks) <= statusRank(core.StatusDesign) {
		t.Error("tasks should outrank design")
	}
	if statusRank(core.StatusComplete) != 0 {
		t.Errorf("complete rank = %d, want 0", statusRank(core.StatusComplete))
	}
}

// TestInitializeListChangedGating asserts capabilities.tools.listChanged is true
// only under expose:"phase" (spec R1/AC1).
func TestInitializeListChangedGating(t *testing.T) {
	cases := []struct {
		name string
		cfg  *core.Config
		want bool
	}{
		{"nil cfg", nil, false},
		{"phase mode", phaseCfg(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := initializeResult(json.RawMessage(`{}`), tc.cfg)
			caps := res["capabilities"].(map[string]any)
			tools := caps["tools"].(map[string]any)
			if got := tools["listChanged"].(bool); got != tc.want {
				t.Errorf("tools.listChanged = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestToolRegistryConcurrent drives concurrent reads against swaps; run under
// -race it proves the RWMutex guards the list (spec R4/AC4).
func TestToolRegistryConcurrent(t *testing.T) {
	cfg := phaseCfg()
	reg := newToolRegistry(buildPhaseTools(cfg, core.StatusDesign, "architect"))
	subsets := [][]toolDef{
		buildPhaseTools(cfg, core.StatusDesign, "architect"),
		buildPhaseTools(cfg, core.StatusExecuting, "craftsman"),
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = len(reg.list())
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			reg.swap(subsets[i%2])
		}
		close(stop)
	}()
	wg.Wait()
}

// TestPhaseWatcherStopsOnCancel asserts the goroutine returns when its context is
// cancelled — no leak, no panic (spec AC5/R5). No specd root exists here, so the
// watcher idles, exercising the cancel path directly.
func TestPhaseWatcherStopsOnCancel(t *testing.T) {
	w := &phaseWatcher{
		registry: newToolRegistry(nil),
		cfg:      phaseCfg(),
		interval: 5 * time.Millisecond,
		debounce: 5 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher goroutine did not stop after cancel")
	}
}

func TestPhaseWatcherTracksRoleChange(t *testing.T) {
	root := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := core.InitialState("auth", "Auth")
	state.Status = core.StatusExecuting
	state.Phase = core.PhaseForStatus(state.Status)
	state.Tasks["T1"] = core.TaskState{ID: "T1", Wave: 1, Role: "craftsman", Status: core.TaskPending}
	if err := core.SaveState(root, "auth", &state); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(root, ".specd", "specs", "auth", "tasks.md")
	tasks := "# Tasks — Auth\n\n## Wave 1\n- [ ] T1 — Login\n  - why: w\n  - role: craftsman\n  - files: x.go\n  - contract: c\n  - acceptance: a\n  - verify: true\n  - depends: —\n  - requirements: 1\n"
	if err := os.WriteFile(tasksPath, []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := phaseCfg()
	reg := newToolRegistry(buildPhaseTools(cfg, core.StatusExecuting, "craftsman"))
	w := &phaseWatcher{registry: reg, cfg: cfg, interval: 5 * time.Millisecond, debounce: 5 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.run(ctx)

	waitFor := func(pred func(map[string]bool) bool) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if pred(toolNameSet(reg.list())) {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for watcher state: %v", toolNameSet(reg.list()))
	}

	waitFor(func(set map[string]bool) bool { return !set["specd_state_read"] && set["specd_next"] })

	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	updated := strings.Replace(string(raw), "- role: craftsman", "- role: validator", 1)
	if updated == string(raw) {
		t.Fatal("task role line not found")
	}
	if err := os.WriteFile(tasksPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}

	waitFor(func(set map[string]bool) bool {
		return set["specd_state_read"] && !set["specd_next"] && set["specd_check"]
	})
}

func TestPhaseWatcherPinnedSpecsStayIsolated(t *testing.T) {
	root := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	seedPinnedSpec := func(slug string, status core.SpecStatus) {
		if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", slug), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", slug, err)
		}
		state := core.InitialState(slug, slug)
		state.Status = status
		state.Phase = core.PhaseForStatus(status)
		if err := core.SaveState(root, slug, &state); err != nil {
			t.Fatalf("SaveState(%s): %v", slug, err)
		}
		tasks := "# Tasks — " + slug + "\n\n## Wave 1\n- [ ] T1 — Login\n  - why: w\n  - role: craftsman\n  - files: x.go\n  - contract: c\n  - acceptance: a\n  - verify: true\n  - depends: —\n  - requirements: 1\n"
		if err := os.WriteFile(filepath.Join(root, ".specd", "specs", slug, "tasks.md"), []byte(tasks), 0o644); err != nil {
			t.Fatalf("write tasks(%s): %v", slug, err)
		}
	}
	seedPinnedSpec("alpha", core.StatusDesign)
	seedPinnedSpec("beta", core.StatusDesign)

	cfg := phaseCfg()
	regAlpha := newToolRegistry(buildPhaseToolsForSpec(cfg, core.StatusDesign, "architect", "alpha"))
	regBeta := newToolRegistry(buildPhaseToolsForSpec(cfg, core.StatusDesign, "architect", "beta"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go (&phaseWatcher{registry: regAlpha, cfg: cfg, pinned: "alpha", interval: 5 * time.Millisecond, debounce: 5 * time.Millisecond}).run(ctx)
	go (&phaseWatcher{registry: regBeta, cfg: cfg, pinned: "beta", interval: 5 * time.Millisecond, debounce: 5 * time.Millisecond}).run(ctx)

	waitForSet := func(reg *toolRegistry, pred func(map[string]bool) bool) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if pred(toolNameSet(reg.list())) {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for watcher state: %v", toolNameSet(reg.list()))
	}

	waitForSet(regAlpha, func(set map[string]bool) bool { return set["specd_check"] && !set["specd_next"] })
	waitForSet(regBeta, func(set map[string]bool) bool { return set["specd_check"] && !set["specd_next"] })

	updateSpecStatus := func(slug string, status core.SpecStatus) {
		t.Helper()
		loaded, err := core.LoadSpec(root, slug)
		if err != nil {
			t.Fatalf("LoadSpec(%s): %v", slug, err)
		}
		state := loaded.State
		state.Status = status
		state.Phase = core.PhaseForStatus(status)
		if err := core.SaveState(root, slug, state); err != nil {
			t.Fatalf("SaveState(%s): %v", slug, err)
		}
	}

	updateSpecStatus("beta", core.StatusExecuting)
	waitForSet(regBeta, func(set map[string]bool) bool { return set["specd_next"] && !set["specd_check"] })
	if got := toolNameSet(regAlpha.list()); got["specd_next"] {
		t.Fatalf("alpha watcher leaked beta transition: %v", got)
	}

	updateSpecStatus("alpha", core.StatusExecuting)
	waitForSet(regAlpha, func(set map[string]bool) bool { return set["specd_next"] && !set["specd_check"] })
	if got := toolNameSet(regBeta.list()); !got["specd_next"] || got["specd_approve"] {
		t.Fatalf("beta watcher lost its own executing state: %v", got)
	}
}
