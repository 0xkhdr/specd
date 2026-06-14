package cmd

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

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
			deriveStatus(tt.state)
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
