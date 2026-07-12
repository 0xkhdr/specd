package verify

import (
	"context"
	"strings"
	"testing"
)

func TestLimitsAppearInSandboxCommand(t *testing.T) {
	limits := Limits{CPUSeconds: 3, MemoryBytes: 64 << 20, Processes: 16, OutputBytes: 1024, WallSeconds: 5, FileBytes: 2048}
	_, args := sandboxArgv("/usr/bin/bwrap", "/repo", "/host/home", "true", limits)
	joined := strings.Join(args, " ")
	for _, want := range []string{"ulimit -t 3", "ulimit -v 65536", "ulimit -u 16", "ulimit -f 4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("sandbox command missing %q: %s", want, joined)
		}
	}
}

func TestOutputLimitRecordsFailure(t *testing.T) {
	result, err := Run(context.Background(), Options{Command: "printf 123456789", Dir: t.TempDir(), Limits: Limits{OutputBytes: 4}})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if result.ExitCode != LimitExitCode || !strings.Contains(result.Stderr, "output limit exceeded") {
		t.Fatalf("result = %+v, want recorded limit failure", result)
	}
}
