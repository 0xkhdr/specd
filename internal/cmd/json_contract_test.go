package cmd_test

import (
	"encoding/json"
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
}

func mustUnmarshal(t *testing.T, raw string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		t.Fatalf("json.Unmarshal failed: %v\nraw: %s", err, raw)
	}
}
