package core

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCustomGateSchema(t *testing.T) {
	in := CustomGateInput{
		Spec: "auth", Root: "/tmp/x", Status: "executing",
		Tasks: []CustomGateTaskRef{{ID: "T1", Status: "pending", Role: "builder", Wave: 1}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"spec"`, `"root"`, `"status"`, `"tasks"`, `"id"`, `"wave"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("input JSON missing %s: %s", key, b)
		}
	}
	var back CustomGateInput
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Spec != "auth" || len(back.Tasks) != 1 || back.Tasks[0].ID != "T1" {
		t.Errorf("input round-trip lost data: %+v", back)
	}

	out := CustomGateOutput{
		Violations: []CustomGateFinding{{Location: "tasks.md:3", Message: "bad"}},
		Warnings:   []CustomGateFinding{{Location: "x", Message: "meh"}},
	}
	ob, _ := json.Marshal(out)
	var oback CustomGateOutput
	if err := json.Unmarshal(ob, &oback); err != nil {
		t.Fatal(err)
	}
	if len(oback.Violations) != 1 || oback.Violations[0].Message != "bad" {
		t.Errorf("output round-trip lost data: %+v", oback)
	}

	// BuildCustomGateInput projects state read-only.
	st := mkState("auth", 1, TaskState{ID: "T1", Wave: 1, Role: "builder", Status: TaskPending})
	bi := BuildCustomGateInput("/root", st)
	if bi.Spec != "auth" || bi.Root != "/root" || len(bi.Tasks) != 1 {
		t.Errorf("BuildCustomGateInput = %+v", bi)
	}
}

func TestCustomGateRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based custom gate is POSIX-only")
	}
	ctx := context.Background()
	in := CustomGateInput{Spec: "auth", Tasks: []CustomGateTaskRef{{ID: "T1"}}}

	t.Run("happy_path_parses_findings_and_echoes_stdin", func(t *testing.T) {
		// The gate reads stdin and emits a violation only if it saw the spec name,
		// proving the input contract reaches the subprocess.
		cmd := `grep -q '"spec":"auth"' && printf '{"violations":[{"location":"tasks.md","message":"nope"}],"warnings":[]}'`
		out, err := RunCustomGate(ctx, t.TempDir(), "sh", cmd, in, 5*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out.Violations) != 1 || out.Violations[0].Message != "nope" {
			t.Errorf("findings = %+v", out.Violations)
		}
	})

	t.Run("invalid_json_is_an_error", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "printf 'not json'", in, 5*time.Second)
		if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("want invalid-JSON error, got %v", err)
		}
	})

	t.Run("non_zero_exit_is_an_error", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "exit 2", in, 5*time.Second)
		if err == nil || !strings.Contains(err.Error(), "non-zero") {
			t.Errorf("want non-zero error, got %v", err)
		}
	})

	t.Run("timeout_is_an_error", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "sleep 5", in, 50*time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Errorf("want timeout error, got %v", err)
		}
	})

	t.Run("nul_byte_rejected", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "echo ok\x00", in, time.Second)
		if err == nil || !strings.Contains(err.Error(), "NUL") {
			t.Errorf("want NUL rejection, got %v", err)
		}
	})
}
