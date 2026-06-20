package mcp

import (
	"context"
	"encoding/json"
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

	planning := toolNameSet(buildPhaseTools(cfg, core.StatusDesign))
	if !planning["specd_inspect"] || !planning["specd_check"] || !planning["specd_approve"] {
		t.Errorf("planning subset missing inspection/gate tools: %v", planning)
	}
	if planning["specd_next"] || planning["specd_dispatch"] {
		t.Errorf("planning subset must not expose the drive loop: %v", planning)
	}

	executing := toolNameSet(buildPhaseTools(cfg, core.StatusExecuting))
	if !executing["specd_next"] || !executing["specd_dispatch"] || !executing["specd_verify"] {
		t.Errorf("executing subset missing drive-loop tools: %v", executing)
	}
	if executing["specd_check"] || executing["specd_approve"] {
		t.Errorf("executing subset must not expose planning gate tools: %v", executing)
	}
}

// TestBuildPhaseToolsUnknownStatusFallsBack confirms an unmapped status yields
// the essential set rather than an empty list (forward-compatibility).
func TestBuildPhaseToolsUnknownStatusFallsBack(t *testing.T) {
	got := toolNameSet(buildPhaseTools(phaseCfg(), core.SpecStatus("future-phase")))
	for _, want := range []string{"specd_inspect", "specd_verify", "specd_task"} {
		if !got[want] {
			t.Errorf("fallback subset missing %q: %v", want, got)
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
	reg := newToolRegistry(buildPhaseTools(cfg, core.StatusDesign))
	subsets := [][]toolDef{
		buildPhaseTools(cfg, core.StatusDesign),
		buildPhaseTools(cfg, core.StatusExecuting),
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
