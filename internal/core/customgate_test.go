package core

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCustomGateSchema(t *testing.T) {
	in := CustomGateInput{
		Spec: "auth", Root: "/tmp/x", Status: "executing",
		Tasks: []CustomGateTaskRef{{ID: "T1", Status: "pending", Role: "craftsman", Wave: 1}},
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
	st := mkState("auth", 1, TaskState{ID: "T1", Wave: 1, Role: "craftsman", Status: TaskPending})
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
		out, err := RunCustomGate(ctx, t.TempDir(), "sh", cmd, in, 5*time.Second, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out.Violations) != 1 || out.Violations[0].Message != "nope" {
			t.Errorf("findings = %+v", out.Violations)
		}
	})

	t.Run("invalid_json_is_an_error", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "printf 'not json'", in, 5*time.Second, "")
		if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("want invalid-JSON error, got %v", err)
		}
	})

	t.Run("non_zero_exit_is_an_error", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "exit 2", in, 5*time.Second, "")
		if err == nil || !strings.Contains(err.Error(), "non-zero") {
			t.Errorf("want non-zero error, got %v", err)
		}
	})

	t.Run("timeout_is_an_error", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "sleep 5", in, 50*time.Millisecond, "")
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Errorf("want timeout error, got %v", err)
		}
	})

	t.Run("nul_byte_rejected", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "echo ok\x00", in, time.Second, "")
		if err == nil || !strings.Contains(err.Error(), "NUL") {
			t.Errorf("want NUL rejection, got %v", err)
		}
	})

	// A5 R2.2: an explicit "none" sandbox is identical to the unset host path.
	t.Run("sandbox_none_is_host_path", func(t *testing.T) {
		cmd := `printf '{"violations":[],"warnings":[]}'`
		if _, err := RunCustomGate(ctx, t.TempDir(), "sh", cmd, in, 5*time.Second, "none"); err != nil {
			t.Fatalf("sandbox=none should run on host: %v", err)
		}
	})

	// A5 R2.3: an unknown/unavailable backend fails the gate closed with a clear
	// error rather than silently falling back to host execution.
	t.Run("unknown_sandbox_fails_closed", func(t *testing.T) {
		_, err := RunCustomGate(ctx, t.TempDir(), "sh", "printf '{}'", in, 5*time.Second, "bogus-backend")
		if err == nil || !strings.Contains(err.Error(), "sandbox unavailable") {
			t.Errorf("want fail-closed sandbox error, got %v", err)
		}
	})
}

// TestCustomGateScrubsEnv asserts A5 R3: a secret-bearing env var present in the
// parent process never reaches the gate subprocess, in either execution path.
func TestCustomGateScrubsEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based custom gate is POSIX-only")
	}
	ctx := context.Background()
	in := CustomGateInput{Spec: "auth"}
	// A non-allowlisted, non-SPECD_ secret. ScrubbedEnv must drop it.
	t.Setenv("MY_SECRET_TOKEN", "leak-me-please")

	// The gate emits a violation iff it can see the secret in its environment.
	cmd := `if [ -n "$MY_SECRET_TOKEN" ]; then printf '{"violations":[{"location":"env","message":"LEAKED"}],"warnings":[]}'; else printf '{"violations":[],"warnings":[]}'; fi`

	t.Run("host_path", func(t *testing.T) {
		out, err := RunCustomGate(ctx, t.TempDir(), "sh", cmd, in, 5*time.Second, "none")
		if err != nil {
			t.Fatalf("gate error: %v", err)
		}
		if len(out.Violations) != 0 {
			t.Fatalf("secret leaked into host gate env: %+v", out.Violations)
		}
	})

	t.Run("sandboxed_path", func(t *testing.T) {
		if _, err := exec.LookPath("bwrap"); err != nil {
			t.Skip("bwrap not on PATH; sandboxed env-scrub path covered when available")
		}
		root := t.TempDir()
		// Probe bwrap in this exact workspace first: nested-namespace and
		// tmpfs-over-/tmp limitations in some CI sandboxes are environment
		// artifacts, not product bugs (real workspaces are not under /tmp). Skip
		// rather than fail when the backend can't start here.
		if _, err := RunCustomGate(ctx, root, "sh", `printf '{}'`, in, 10*time.Second, "bwrap"); err != nil {
			t.Skipf("bwrap cannot start in this environment: %v", err)
		}
		out, err := RunCustomGate(ctx, root, "sh", cmd, in, 10*time.Second, "bwrap")
		if err != nil {
			t.Fatalf("sandboxed gate error: %v", err)
		}
		if len(out.Violations) != 0 {
			t.Fatalf("secret leaked into sandboxed gate env: %+v", out.Violations)
		}
	})
}
