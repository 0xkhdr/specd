package cmd_test

// Surface A/B context-manifest integration: asserts `specd context` and
// `specd dispatch` emit the shared engine's manifest (measured accounting,
// load items) and that dispatch dedupes the role asset across a same-role wave.
// Source spec: Level Up context-engineering, T5/T6 (AC-1, AC-5, AC-8).

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

func execSpec(h *th.Harness, tasks ...th.TaskSpec) string {
	b := h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign()
	for _, t := range tasks {
		b = b.AddTask(t)
	}
	return b.Status(core.StatusExecuting).Build()
}

// TestContextManifestJSON asserts `specd context --json` carries the additive
// contextManifest accounting block while preserving the legacy keys (AC-1, AC-8).
func TestContextManifestJSON(t *testing.T) {
	h := th.New(t)
	slug := execSpec(h, th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}})
	res := h.RunExpect(core.ExitOK, "context", slug, "--json")

	var got struct {
		Spec            string   `json:"spec"`
		Skill           string   `json:"skill"`
		Load            []string `json:"load"`
		ContextManifest struct {
			Version         int `json:"version"`
			EstimatedTokens int `json:"estimatedTokens"`
			Budget          int `json:"budget"`
			Items           []struct {
				Kind      string `json:"kind"`
				Mode      string `json:"mode"`
				TokenHint int    `json:"tokenHint"`
				Required  bool   `json:"required"`
			} `json:"items"`
		} `json:"contextManifest"`
	}
	mustUnmarshal(t, res.Stdout, &got)

	// Legacy keys preserved (back-compat).
	if got.Spec != slug || got.Skill == "" || len(got.Load) == 0 {
		t.Errorf("legacy context keys missing: %+v", got)
	}
	m := got.ContextManifest
	if m.Version != 1 {
		t.Errorf("manifest version = %d, want 1", m.Version)
	}
	if len(m.Items) == 0 {
		t.Fatalf("manifest has no items")
	}
	if m.EstimatedTokens <= 0 || m.Budget <= 0 {
		t.Errorf("accounting missing: est=%d budget=%d", m.EstimatedTokens, m.Budget)
	}
	if m.Items[0].Kind != "role" {
		t.Errorf("first item kind = %q, want role", m.Items[0].Kind)
	}
	// EstimatedTokens equals the sum of required-item hints.
	sum := 0
	for _, it := range m.Items {
		if it.Required {
			sum += it.TokenHint
		}
	}
	if sum != m.EstimatedTokens {
		t.Errorf("estimatedTokens = %d, want sum of required hints %d", m.EstimatedTokens, sum)
	}
}

// TestContextManifestHuman asserts the human briefing prints the LOAD NOW table
// and the budget line produced by the engine.
func TestContextManifestHuman(t *testing.T) {
	h := th.New(t)
	slug := execSpec(h, th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}})
	res := h.RunExpect(core.ExitOK, "context", slug)
	for _, want := range []string{"LOAD NOW", "budget", "est "} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("human context missing %q\n%s", want, res.Stdout)
		}
	}
}

// TestDispatchRoleDedup asserts a multi-task same-role wave references the role
// asset once via the shared assets map and never inlines the role prompt (AC-5).
func TestDispatchRoleDedup(t *testing.T) {
	h := th.New(t)
	slug := execSpec(h,
		th.TaskSpec{ID: "T1", Title: "Login form", Role: "builder", Verify: "true", Requirements: []int{1}},
		th.TaskSpec{ID: "T2", Title: "Login api", Role: "builder", Verify: "true", Requirements: []int{1}},
	)
	res := h.RunExpect(core.ExitOK, "dispatch", slug, "--json")

	var got struct {
		Count   int               `json:"count"`
		Assets  map[string]string `json:"assets"`
		Packets []struct {
			ID         string `json:"id"`
			Role       string `json:"role"`
			RolePath   string `json:"rolePath"`
			RolePrompt string `json:"rolePrompt"`
			Context    *struct {
				EstimatedTokens int `json:"estimatedTokens"`
				Budget          int `json:"budget"`
			} `json:"contextManifest"`
		} `json:"packets"`
	}
	mustUnmarshal(t, res.Stdout, &got)

	if got.Count != 2 || len(got.Packets) != 2 {
		t.Fatalf("want 2 packets, got %+v", got.Packets)
	}
	if len(got.Assets) != 1 || got.Assets["role/builder"] == "" {
		t.Errorf("assets = %+v, want single role/builder entry", got.Assets)
	}
	for _, p := range got.Packets {
		if p.RolePrompt != "" {
			t.Errorf("packet %s inlined rolePrompt without --inline-roles", p.ID)
		}
		if p.RolePath != got.Assets["role/"+p.Role] {
			t.Errorf("packet %s rolePath %q != asset %q", p.ID, p.RolePath, got.Assets["role/"+p.Role])
		}
		if p.Context == nil || p.Context.Budget <= 0 {
			t.Errorf("packet %s missing context manifest accounting", p.ID)
		}
	}
}

// TestDispatchInlineRoles asserts --inline-roles restores the pre-dedupe shape:
// each packet carries the full role text and the shared assets map is dropped.
func TestDispatchInlineRoles(t *testing.T) {
	h := th.New(t)
	h.Init() // scaffold roles/ so the inlined role prompt has content
	slug := execSpec(h,
		th.TaskSpec{ID: "T1", Title: "Login form", Role: "builder", Verify: "true", Requirements: []int{1}},
		th.TaskSpec{ID: "T2", Title: "Login api", Role: "builder", Verify: "true", Requirements: []int{1}},
	)
	res := h.RunExpect(core.ExitOK, "dispatch", slug, "--json", "--inline-roles")

	var got struct {
		Assets  map[string]string `json:"assets"`
		Packets []struct {
			ID         string `json:"id"`
			RolePrompt string `json:"rolePrompt"`
		} `json:"packets"`
	}
	mustUnmarshal(t, res.Stdout, &got)

	if got.Assets != nil {
		t.Errorf("assets should be omitted under --inline-roles, got %+v", got.Assets)
	}
	for _, p := range got.Packets {
		if p.RolePrompt == "" {
			t.Errorf("packet %s missing inlined rolePrompt under --inline-roles", p.ID)
		}
	}
}
