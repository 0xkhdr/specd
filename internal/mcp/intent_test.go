package mcp

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// captureDispatch records the command and parsed args of the single dispatch an
// intent tool produces, so a test can assert the translation without running the
// real handler. It always reports the command as known and succeeding.
func captureDispatch(command *string, args *cli.Args) Dispatcher {
	return func(c string, a cli.Args) (int, bool) {
		*command = c
		*args = a
		return core.ExitOK, true
	}
}

func callIntent(t *testing.T, name string, arguments map[string]any) (string, cli.Args, *rpcError) {
	t.Helper()
	params, err := json.Marshal(callParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var gotCmd string
	var gotArgs cli.Args
	_, rerr := callTool(params, captureDispatch(&gotCmd, &gotArgs), 0)
	return gotCmd, gotArgs, rerr
}

// TestIntentToolTranslation asserts each intent tool (GAP-5) routes to the right
// underlying command + argv, that --json is always forced, and that sane
// defaults (planning via `brain run`, --bootstrap) and goal→title wiring hold.
func TestIntentToolTranslation(t *testing.T) {
	cases := []struct {
		name      string
		args      map[string]any
		wantCmd   string
		wantPos   []string
		wantFlags map[string]string // subset that must be present
	}{
		{
			name:    "brain_orchestrate",
			args:    map[string]any{"spec": "auth", "goal": "Add login"},
			wantCmd: "brain",
			wantPos: []string{"run", "auth"},
			wantFlags: map[string]string{
				"bootstrap": "true", "title": "Add login", "json": "true",
			},
		},
		{
			name:    "brain_orchestrate",
			args:    map[string]any{"spec": "auth", "worker_cmd": "run-worker", "approval_policy": "session", "max_steps": "12"},
			wantCmd: "brain",
			wantPos: []string{"run", "auth"},
			wantFlags: map[string]string{
				"worker-cmd": "run-worker", "approval-policy": "session", "max-steps": "12", "bootstrap": "true",
			},
		},
		{
			name:    "brain_orchestrate",
			args:    map[string]any{"spec": "auth", "no_bootstrap": true},
			wantCmd: "brain",
			wantPos: []string{"run", "auth"},
		},
		{
			name:      "brain_status",
			args:      map[string]any{"session": "S1"},
			wantCmd:   "brain",
			wantPos:   []string{"status"},
			wantFlags: map[string]string{"session": "S1"},
		},
		{
			name:      "brain_status",
			args:      map[string]any{"session": "S1", "program": true},
			wantCmd:   "brain",
			wantPos:   []string{"status"},
			wantFlags: map[string]string{"session": "S1", "program": "true"},
		},
		{
			name:    "brain_approve",
			args:    map[string]any{"spec": "auth"},
			wantCmd: "approve",
			wantPos: []string{"auth"},
		},
		{
			name:      "brain_pause",
			args:      map[string]any{"session": "S1"},
			wantCmd:   "brain",
			wantPos:   []string{"pause"},
			wantFlags: map[string]string{"session": "S1"},
		},
		{
			name:      "brain_resume",
			args:      map[string]any{"session": "S1"},
			wantCmd:   "brain",
			wantPos:   []string{"resume"},
			wantFlags: map[string]string{"session": "S1"},
		},
		{
			name:      "brain_resume",
			args:      map[string]any{"max_age_minutes": "30"},
			wantCmd:   "brain",
			wantPos:   []string{"resume"},
			wantFlags: map[string]string{"list": "true", "max-age-minutes": "30"},
		},
		{
			name:      "brain_cancel",
			args:      map[string]any{"session": "S1", "program": true},
			wantCmd:   "brain",
			wantPos:   []string{"cancel"},
			wantFlags: map[string]string{"session": "S1", "program": "true"},
		},
	}

	for _, tc := range cases {
		gotCmd, gotArgs, rerr := callIntent(t, tc.name, tc.args)
		if rerr != nil {
			t.Errorf("%s(%v): unexpected error %v", tc.name, tc.args, rerr)
			continue
		}
		if gotCmd != tc.wantCmd {
			t.Errorf("%s(%v): command = %q, want %q", tc.name, tc.args, gotCmd, tc.wantCmd)
		}
		if !reflect.DeepEqual(gotArgs.Pos, tc.wantPos) {
			t.Errorf("%s(%v): positionals = %v, want %v", tc.name, tc.args, gotArgs.Pos, tc.wantPos)
		}
		// --json is forced for every intent tool so the host gets structured output.
		if gotArgs.Str("json") != "true" {
			t.Errorf("%s(%v): --json not forced", tc.name, tc.args)
		}
		for k, v := range tc.wantFlags {
			if got := gotArgs.Str(k); got != v {
				t.Errorf("%s(%v): flag %q = %q, want %q", tc.name, tc.args, k, got, v)
			}
		}
		// no_bootstrap omits --bootstrap.
		if tc.name == "brain_orchestrate" && tc.args["no_bootstrap"] == true {
			if gotArgs.Has("bootstrap") {
				t.Errorf("%s(%v): --bootstrap present despite no_bootstrap", tc.name, tc.args)
			}
		}
	}
}

// TestIntentToolValidation asserts missing required arguments and wrong-typed
// arguments fail closed as MCP invalid-params errors instead of dispatching.
func TestIntentToolValidation(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"brain_orchestrate", map[string]any{"goal": "no spec"}},
		{"brain_status", map[string]any{}},
		{"brain_approve", map[string]any{}},
		{"brain_pause", map[string]any{}},
		{"brain_cancel", map[string]any{}},
		{"brain_status", map[string]any{"session": "S1", "program": "yes"}}, // program must be boolean
		{"brain_orchestrate", map[string]any{"spec": []any{"bad"}}},         // spec must be a scalar string
	}
	for _, tc := range cases {
		dispatched := false
		params, _ := json.Marshal(callParams{Name: tc.name, Arguments: tc.args})
		_, rerr := callTool(params, func(string, cli.Args) (int, bool) {
			dispatched = true
			return core.ExitOK, true
		}, 0)
		if rerr == nil {
			t.Errorf("%s(%v): want invalid-params error, got nil", tc.name, tc.args)
		}
		if dispatched {
			t.Errorf("%s(%v): dispatched despite validation failure", tc.name, tc.args)
		}
	}
}
