//go:build windows

package worker

import (
	"context"
	"testing"
)

func TestShellRunnerWindowsFailsFast(t *testing.T) {
	res, err := ShellRunner{}.Run(context.Background(), Mission{Command: "echo no-op"})
	if err == nil {
		t.Fatal("expected Windows orchestration to fail fast")
	}
	if got := err.Error(); got != windowsUnsupportedMessage {
		t.Fatalf("message = %q, want %q", got, windowsUnsupportedMessage)
	}
	if res.ExitErr == nil || res.ExitErr.Error() != windowsUnsupportedMessage {
		t.Fatalf("result exit error = %v, want %q", res.ExitErr, windowsUnsupportedMessage)
	}
}
