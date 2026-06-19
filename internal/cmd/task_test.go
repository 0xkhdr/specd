package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestValidateComplete(t *testing.T) {
	const slug, id = "demo", "T1"
	verifyLine := "go test ./..."
	goodRec := &core.VerificationRecord{Command: verifyLine, Verified: true, RanAt: "2026-01-01T00:00:00Z"}
	docTask := func() *core.ParsedTask {
		return &core.ParsedTask{ID: id, Meta: map[string]string{"verify": verifyLine}}
	}

	tests := []struct {
		name      string
		state     *core.State
		ts        core.TaskState
		docTask   *core.ParsedTask
		flags     map[string]string
		wantEv    string // substring expected in returned evidence (empty = skip)
		wantErr   bool
		errSubstr string // substring expected in error message
	}{
		{
			name:      "dependency not complete → error",
			state:     &core.State{Tasks: map[string]core.TaskState{"T0": {ID: "T0", Status: core.TaskPending}}},
			ts:        core.TaskState{ID: id, Depends: []string{"T0"}, Verification: goodRec},
			docTask:   docTask(),
			wantErr:   true,
			errSubstr: "dependencies not complete",
		},
		{
			name:      "unverified without evidence → error",
			state:     &core.State{Tasks: map[string]core.TaskState{}},
			ts:        core.TaskState{ID: id},
			docTask:   docTask(),
			flags:     map[string]string{"unverified": "true"},
			wantErr:   true,
			errSubstr: "requires non-empty --evidence",
		},
		{
			name:    "unverified with evidence → ok",
			state:   &core.State{Tasks: map[string]core.TaskState{}},
			ts:      core.TaskState{ID: id},
			docTask: docTask(),
			flags:   map[string]string{"unverified": "true", "evidence": "manual proof"},
			wantEv:  "manual proof",
		},
		{
			name:      "no passing verification → error",
			state:     &core.State{Tasks: map[string]core.TaskState{}},
			ts:        core.TaskState{ID: id},
			docTask:   docTask(),
			wantErr:   true,
			errSubstr: "requires a passing",
		},
		{
			name:      "stale verification command → error",
			state:     &core.State{Tasks: map[string]core.TaskState{}},
			ts:        core.TaskState{ID: id, Verification: &core.VerificationRecord{Command: "old cmd", Verified: true}},
			docTask:   docTask(),
			wantErr:   true,
			errSubstr: "verification is stale",
		},
		{
			name:    "verified → derives evidence",
			state:   &core.State{Tasks: map[string]core.TaskState{}},
			ts:      core.TaskState{ID: id, Verification: goodRec},
			docTask: docTask(),
			wantEv:  "verified: `go test ./...`",
		},
		{
			name:    "verified with explicit evidence overrides derived",
			state:   &core.State{Tasks: map[string]core.TaskState{}},
			ts:      core.TaskState{ID: id, Verification: goodRec},
			docTask: docTask(),
			flags:   map[string]string{"evidence": "custom note"},
			wantEv:  "custom note",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, serr := core.ValidateTaskCompletion(tt.state, tt.ts, tt.docTask, slug, id, tt.flags["evidence"], tt.flags["unverified"] == "true")
			if tt.wantErr {
				if serr == nil {
					t.Fatalf("expected error, got nil (ev=%q)", ev)
				}
				if tt.errSubstr != "" && !strings.Contains(serr.Message, tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", serr.Message, tt.errSubstr)
				}
				return
			}
			if serr != nil {
				t.Fatalf("unexpected error: %v", serr)
			}
			if tt.wantEv != "" && !strings.Contains(ev, tt.wantEv) {
				t.Fatalf("evidence %q does not contain %q", ev, tt.wantEv)
			}
		})
	}
}

func TestDeriveStatus(t *testing.T) {
	tests := []struct {
		name       string
		state      *core.State
		wantStatus core.SpecStatus
		wantPhase  core.Phase
		noChange   bool // if true, Status/Phase should remain at their initial zero values
	}{
		{
			name:     "empty — no tasks → no-op",
			state:    &core.State{Tasks: map[string]core.TaskState{}},
			noChange: true,
		},
		{
			name: "not started — all pending → no-op",
			state: &core.State{Tasks: map[string]core.TaskState{
				"T1": {ID: "T1", Status: core.TaskPending},
				"T2": {ID: "T2", Status: core.TaskPending},
			}},
			noChange: true,
		},
		{
			name: "mixed — some running, some pending → executing",
			state: &core.State{Tasks: map[string]core.TaskState{
				"T1": {ID: "T1", Status: core.TaskComplete},
				"T2": {ID: "T2", Status: core.TaskPending},
			}},
			wantStatus: core.StatusExecuting,
			wantPhase:  core.PhaseExecute,
		},
		{
			name: "all complete, not yet complete → verifying",
			state: &core.State{
				Status: core.StatusExecuting,
				Tasks: map[string]core.TaskState{
					"T1": {ID: "T1", Status: core.TaskComplete},
					"T2": {ID: "T2", Status: core.TaskComplete},
				},
			},
			wantStatus: core.StatusVerifying,
			wantPhase:  core.PhaseVerify,
		},
		{
			name: "all complete, already complete → stays complete",
			state: &core.State{
				Status: core.StatusComplete,
				Tasks: map[string]core.TaskState{
					"T1": {ID: "T1", Status: core.TaskComplete},
					"T2": {ID: "T2", Status: core.TaskComplete},
				},
			},
			wantStatus: core.StatusComplete,
			wantPhase:  core.PhaseReflect,
		},
		{
			name: "all blocked — no runnable → blocked",
			state: &core.State{Tasks: map[string]core.TaskState{
				"T1": {ID: "T1", Status: core.TaskBlocked},
				"T2": {ID: "T2", Status: core.TaskBlocked},
			}},
			wantStatus: core.StatusBlocked,
			wantPhase:  core.PhaseExecute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origStatus := tt.state.Status
			origPhase := tt.state.Phase
			core.DeriveSpecStatus(tt.state)
			if tt.noChange {
				if tt.state.Status != origStatus || tt.state.Phase != origPhase {
					t.Fatalf("expected no change, got status=%q phase=%q", tt.state.Status, tt.state.Phase)
				}
				return
			}
			if tt.state.Status != tt.wantStatus {
				t.Fatalf("status: want %q, got %q", tt.wantStatus, tt.state.Status)
			}
			if tt.state.Phase != tt.wantPhase {
				t.Fatalf("phase: want %q, got %q", tt.wantPhase, tt.state.Phase)
			}
		})
	}
}
