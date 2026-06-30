package cmd_test

// Concern (cross-cutting): the machine-readable --json output contract shared by
// every command. Locks JSON schemas, error envelopes, and no-ANSI guarantees in
// one place rather than scattering them across per-command files (spec 2.3).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestJSONContracts locks the machine-readable --json schema of each command by
// unmarshalling into the expected struct and asserting key fields, rather than
// relying on brittle substring checks.
func TestJSONContracts(t *testing.T) {
	newSpec := func(h *th.Harness) string {
		return h.Spec("auth").
			Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
			FullDesign().
			AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
			Status(core.StatusExecuting).
			Build()
	}

	t.Run("status", func(t *testing.T) {
		h := th.New(t)
		slug := newSpec(h)
		res := h.RunExpect(core.ExitOK, "status", slug, "--json")
		var got struct {
			Spec   string `json:"spec"`
			Status string `json:"status"`
			Phase  string `json:"phase"`
			Counts struct {
				Total    int `json:"total"`
				Complete int `json:"complete"`
			} `json:"counts"`
			Next struct {
				Kind string `json:"kind"`
				ID   string `json:"id"`
			} `json:"next"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if got.Spec != slug {
			t.Errorf("spec = %q, want %q", got.Spec, slug)
		}
		if got.Status != string(core.StatusExecuting) {
			t.Errorf("status = %q, want executing", got.Status)
		}
		if got.Counts.Total != 1 {
			t.Errorf("counts.total = %d, want 1", got.Counts.Total)
		}
		if got.Next.Kind != string(core.NextTask) || got.Next.ID != "T1" {
			t.Errorf("next = %+v, want task/T1", got.Next)
		}
	})

	t.Run("context", func(t *testing.T) {
		h := th.New(t)
		slug := newSpec(h)
		res := h.RunExpect(core.ExitOK, "context", slug, "--json")
		var got struct {
			Spec   string   `json:"spec"`
			Status string   `json:"status"`
			Skill  string   `json:"skill"`
			Load   []string `json:"load"`
			Next   string   `json:"next"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if got.Spec != slug {
			t.Errorf("spec = %q, want %q", got.Spec, slug)
		}
		if got.Skill == "" || len(got.Load) == 0 {
			t.Errorf("skill/load missing: %+v", got)
		}
	})

	t.Run("next", func(t *testing.T) {
		h := th.New(t)
		slug := newSpec(h)
		res := h.RunExpect(core.ExitOK, "next", slug, "--json")
		var got struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
			Task struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Wave  int    `json:"wave"`
			} `json:"task"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if got.Kind != string(core.NextTask) || got.ID != "T1" {
			t.Errorf("kind/id = %s/%s, want task/T1", got.Kind, got.ID)
		}
		if got.Task.ID != "T1" || got.Task.Wave != 1 {
			t.Errorf("task = %+v, want id T1 wave 1", got.Task)
		}
	})

	t.Run("dispatch", func(t *testing.T) {
		h := th.New(t)
		slug := newSpec(h)
		res := h.RunExpect(core.ExitOK, "dispatch", slug, "--json")
		var got struct {
			Kind    string `json:"kind"`
			Count   int    `json:"count"`
			Packets []struct {
				ID   string `json:"id"`
				Role string `json:"role"`
			} `json:"packets"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if got.Kind != "frontier" {
			t.Errorf("kind = %q, want frontier", got.Kind)
		}
		if got.Count != 1 || len(got.Packets) != 1 || got.Packets[0].ID != "T1" {
			t.Errorf("packets = %+v, want one T1 packet", got.Packets)
		}
	})

	t.Run("program", func(t *testing.T) {
		h := th.New(t)
		newSpec(h)
		res := h.RunExpect(core.ExitOK, "program", "--json")
		var got struct {
			Kind  string `json:"kind"`
			Count int    `json:"count"`
			Specs []struct {
				Slug   string `json:"slug"`
				Status string `json:"status"`
			} `json:"specs"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if got.Kind != "program" {
			t.Errorf("kind = %q, want program", got.Kind)
		}
		if got.Count != 1 || len(got.Specs) != 1 || got.Specs[0].Slug != "auth" {
			t.Errorf("specs = %+v, want one auth spec", got.Specs)
		}
	})

	t.Run("check", func(t *testing.T) {
		h := th.New(t)
		slug := newSpec(h)
		res := h.RunExpect(core.ExitOK, "check", slug, "--json")
		var got struct {
			OK         bool  `json:"ok"`
			Violations []any `json:"violations"`
			Warnings   []any `json:"warnings"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if !got.OK {
			t.Errorf("ok = false, want true for a clean spec; violations=%+v", got.Violations)
		}
		// Stable shape: violations/warnings are always arrays, never null.
		if got.Violations == nil || got.Warnings == nil {
			t.Errorf("violations/warnings must serialize as [], got %+v", got)
		}
	})

	t.Run("waves", func(t *testing.T) {
		h := th.New(t)
		slug := newSpec(h)
		res := h.RunExpect(core.ExitOK, "waves", slug, "--json")
		var got struct {
			Waves []struct {
				Wave  int `json:"wave"`
				Tasks []struct {
					ID      string   `json:"id"`
					Depends []string `json:"depends"`
				} `json:"tasks"`
			} `json:"waves"`
			CriticalPath []string `json:"criticalPath"`
			Blockers     []any    `json:"blockers"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if len(got.Waves) != 1 || len(got.Waves[0].Tasks) != 1 || got.Waves[0].Tasks[0].ID != "T1" {
			t.Errorf("waves = %+v, want one wave with T1", got.Waves)
		}
		// criticalPath/blockers must be arrays, never null.
		if got.CriticalPath == nil || got.Blockers == nil {
			t.Errorf("criticalPath/blockers must serialize as [], got %+v", got)
		}
	})

	t.Run("approve", func(t *testing.T) {
		h := th.New(t)
		slug := h.Spec("auth").
			Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
			Status(core.StatusRequirements).
			Build()
		res := h.RunExpect(core.ExitOK, "approve", slug, "--json")
		var got struct {
			OK     bool   `json:"ok"`
			Action string `json:"action"`
			From   string `json:"from"`
			Status string `json:"status"`
			Phase  string `json:"phase"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if !got.OK || got.Action != "advanced" {
			t.Errorf("ok/action = %v/%q, want true/advanced", got.OK, got.Action)
		}
		if got.From != string(core.StatusRequirements) || got.Status != string(core.StatusDesign) {
			t.Errorf("from/status = %q/%q, want requirements/design", got.From, got.Status)
		}
	})

	t.Run("doctor", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitOK, "init", "--agent", "none", "--non-interactive")
		// Remove a required scaffold file to guarantee doctor fails with ExitGate
		// regardless of the host's installed agent integrations.
		_ = os.Remove(filepath.Join(h.Root, ".specd", "steering", "reasoning.md"))
		res := h.RunExpect(core.ExitGate, "doctor", "--json")
		var got struct {
			SchemaVersion int    `json:"schemaVersion"`
			Status        string `json:"status"`
			Root          string `json:"root"`
			Checks        []struct {
				Name        string `json:"name"`
				Status      string `json:"status"`
				Detail      string `json:"detail"`
				Remediation string `json:"remediation"`
			} `json:"checks"`
			Hosts []struct {
				Name        string `json:"name"`
				Detected    bool   `json:"detected"`
				Registered  bool   `json:"registered"`
				Owned       bool   `json:"owned"`
				Status      string `json:"status"`
				Reason      string `json:"reason"`
				Remediation string `json:"remediation"`
			} `json:"hosts"`
			Remediations []string `json:"remediations"`
			NextAction   string   `json:"nextAction"`
		}
		mustUnmarshal(t, res.Stdout, &got)
		if got.SchemaVersion != 1 {
			t.Errorf("schemaVersion = %d, want 1", got.SchemaVersion)
		}
		if got.Status != "unhealthy" {
			t.Errorf("status = %q, want unhealthy", got.Status)
		}
		if got.Root == "" {
			t.Errorf("root is empty")
		}
		if got.Checks == nil || got.Hosts == nil || got.Remediations == nil {
			t.Errorf("checks/hosts/remediations must serialize as non-null arrays, got %+v", got)
		}
	})
}

// TestJSONErrorPath drives R2.3: a command that fails still emits a
// machine-readable object under --json (ok:false + structured context), and
// exits non-zero — never a bare error string an agent's parser would choke on.
// A spec whose requirements.md is not valid EARS fails the `ears` gate.
func TestJSONErrorPath(t *testing.T) {
	h := th.New(t)
	slug := h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "this criterion is not valid EARS").
		Status(core.StatusRequirements).
		Build()

	res := h.RunExpect(core.ExitGate, "check", slug, "--json")
	var got struct {
		OK         bool `json:"ok"`
		Violations []struct {
			Gate    string `json:"gate"`
			Message string `json:"message"`
		} `json:"violations"`
	}
	mustUnmarshal(t, res.Stdout, &got)
	if got.OK {
		t.Errorf("ok = true, want false on a failing check")
	}
	if len(got.Violations) == 0 {
		t.Errorf("expected at least one violation under --json, got none")
	}
}

// TestJSONNoANSI drives R2.2: no command's --json stdout may carry ANSI escape
// codes (no colors, spinners, or cursor moves) — the channel must be pure data
// for non-terminal consumers.
func TestJSONNoANSI(t *testing.T) {
	h := th.New(t)
	h.RunExpect(core.ExitOK, "init", "--agent", "none", "--non-interactive")
	build := func(h *th.Harness) string {
		return h.Spec("auth").
			Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
			FullDesign().
			AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
			Status(core.StatusExecuting).
			Build()
	}
	s := build(h)
	cmds := [][]string{
		{"status", s, "--json"},
		{"context", s, "--json"},
		{"next", s, "--json"},
		{"dispatch", s, "--json"},
		{"program", "--json"},
		{"check", s, "--json"},
		{"waves", s, "--json"},
	}
	for _, argv := range cmds {
		t.Run(argv[0], func(t *testing.T) {
			res := h.RunExpect(core.ExitOK, argv[0], argv[1:]...)
			if strings.ContainsRune(res.Stdout, '\x1b') {
				t.Errorf("%s --json stdout contains ANSI escape: %q", argv[0], res.Stdout)
			}
		})
	}
	t.Run("doctor", func(t *testing.T) {
		// Remove a required scaffold file to guarantee doctor fails with ExitGate
		// regardless of the host's installed agent integrations.
		_ = os.Remove(filepath.Join(h.Root, ".specd", "steering", "reasoning.md"))
		res := h.RunExpect(core.ExitGate, "doctor", "--json")
		if strings.ContainsRune(res.Stdout, '\x1b') {
			t.Errorf("doctor --json stdout contains ANSI escape: %q", res.Stdout)
		}
	})
}

// TestJSONUninstall covers the retired runtime command contract: uninstall is
// now install-script-only, but --json still emits a stable machine-readable
// deprecation object with a non-zero exit.
func TestJSONUninstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	h := th.New(t)
	res := h.RunExpect(core.ExitGate, "uninstall", "--json")
	var got struct {
		Kind    string `json:"kind"`
		Command string `json:"command"`
	}
	mustUnmarshal(t, res.Stdout, &got)
	if got.Kind != "deprecated-command" {
		t.Errorf("kind = %q, want deprecated-command", got.Kind)
	}
	if got.Command != "uninstall" {
		t.Errorf("command = %q, want uninstall", got.Command)
	}
}

func mustUnmarshal(t *testing.T, raw string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		t.Fatalf("json.Unmarshal failed: %v\nraw: %s", err, raw)
	}
}
