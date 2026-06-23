//go:build !windows

package worker

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestShellRunnerMissionEnvPropagation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell runner")
	}
	var stdout bytes.Buffer
	r := ShellRunner{Stdout: &stdout, Stderr: &stdout}
	m := Mission{
		Command: strings.Join([]string{
			`printf 'SESSION=%s\n' "$SPECD_SESSION"`,
			`printf 'WORKER=%s\n' "$SPECD_WORKER"`,
			`printf 'SPEC=%s\n' "$SPECD_SPEC"`,
			`printf 'TASK=%s\n' "$SPECD_TASK"`,
			`printf 'ROLE=%s\n' "$SPECD_ROLE"`,
			`printf 'ARTIFACT=%s\n' "$SPECD_ARTIFACT"`,
			`test -r "$SPECD_MISSION"`,
			`printf 'MISSION_JSON='; grep -q '"workerId": "worker-1"' "$SPECD_MISSION" && printf 'worker-1\n'`,
		}, "; "),
		MissionID: "mission-1",
		SessionID: "session-1",
		WorkerID:  "worker-1",
		Spec:      "spec-a",
		TaskID:    "T1",
		Role:      "builder",
		Files:     []string{"/tmp/a.md", "dir/b.go"},
		Deadline:  time.Now().Add(5 * time.Second).Format(time.RFC3339Nano),
		Payload: map[string]any{
			"workerId": "worker-1",
		},
	}
	res, err := r.Run(context.Background(), m)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.ExitErr != nil || res.TimedOut {
		t.Fatalf("unexpected result: %+v", res)
	}
	got := stdout.String()
	for _, want := range []string{
		"[worker-1] SESSION=session-1\n",
		"[worker-1] WORKER=worker-1\n",
		"[worker-1] SPEC=spec-a\n",
		"[worker-1] TASK=T1\n",
		"[worker-1] ROLE=builder\n",
		"[worker-1] ARTIFACT=a.md,b.go\n",
		"[worker-1] MISSION_JSON=worker-1\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestShellRunnerDeadlineKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell runner")
	}
	pidFile, err := os.CreateTemp(t.TempDir(), "child-pid-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	pidPath := pidFile.Name()
	_ = pidFile.Close()

	cmd := `sh -c 'sleep 30 & echo $! > ` + pidPath + `; wait'`
	var stderr bytes.Buffer
	r := ShellRunner{Stderr: &stderr}
	start := time.Now()
	res, err := r.Run(context.Background(), Mission{
		Command:  cmd,
		WorkerID: "killer",
		TaskID:   "Tkill",
		Deadline: time.Now().Add(200 * time.Millisecond).Format(time.RFC3339Nano),
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !res.TimedOut {
		t.Fatalf("TimedOut=false: %+v", res)
	}
	if time.Since(start) > waitDelay+2*time.Second {
		t.Fatalf("Run exceeded WaitDelay margin: %s", time.Since(start))
	}

	pidBytes, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid := strings.TrimSpace(string(pidBytes))
	if pid == "" {
		t.Fatal("child pid not written")
	}
	for i := 0; i < 50; i++ {
		if _, err := os.Stat("/proc/" + pid); os.IsNotExist(err) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if runtime.GOOS == "linux" {
		t.Fatalf("child pid %s still exists", pid)
	}
}

func TestShellRunnerPipeDrainNoHang(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell runner")
	}
	var stdout bytes.Buffer
	r := ShellRunner{Stdout: &stdout}
	start := time.Now()
	res, err := r.Run(context.Background(), Mission{
		Command:  `sh -c 'while true; do echo noisy; sleep 0.01; done & wait'`,
		WorkerID: "pipe",
		TaskID:   "Tpipe",
		Deadline: time.Now().Add(150 * time.Millisecond).Format(time.RFC3339Nano),
	})
	if err == nil || !res.TimedOut {
		t.Fatalf("expected timeout, got res=%+v err=%v", res, err)
	}
	if elapsed := time.Since(start); elapsed > waitDelay+2*time.Second {
		t.Fatalf("Run hung past WaitDelay margin: %s", elapsed)
	}
}

func TestShellRunnerWritesMissionItselfWithoutPayload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell runner")
	}
	var stdout bytes.Buffer
	r := ShellRunner{Stdout: &stdout}
	m := Mission{
		Command:  `grep -q '"WorkerID": "self"' "$SPECD_MISSION" && printf 'self\n'`,
		WorkerID: "self",
		TaskID:   "Tself",
		Deadline: time.Now().Add(5 * time.Second).Format(time.RFC3339Nano),
	}
	if _, err := r.Run(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimPrefix(strings.TrimSpace(stdout.String()), "[self] "); got != "self" {
		t.Fatalf("mission self payload check = %q", got)
	}
}
