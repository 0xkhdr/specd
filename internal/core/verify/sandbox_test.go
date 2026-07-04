package verify

import (
	"context"
	"strings"
	"testing"
)

func TestSandboxFailClosed(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Command:       "true",
		Dir:           t.TempDir(),
		Sandbox:       true,
		SandboxBinary: "definitely-missing-specd-sandbox",
	})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("Run sandbox error = %v, want fail closed", err)
	}
}
