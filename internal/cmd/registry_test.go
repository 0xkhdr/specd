package cmd

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestEveryCommandHasHandler is the parity guard (R13.2): every verb in
// core.Commands must resolve to a non-nil handler or carry Deferred:true.
func TestEveryCommandHasHandler(t *testing.T) {
	for _, command := range core.Commands {
		if command.Deferred {
			continue
		}
		if executable[command.Name] == nil {
			t.Errorf("command %q has no handler and is not marked Deferred", command.Name)
		}
	}
}

// TestUnknownCommandFailsClosed guards R13.1: an unregistered verb returns
// ErrUnknownCommand so the dispatcher can exit 2 instead of 0.
func TestUnknownCommandFailsClosed(t *testing.T) {
	err := Run(".", "bogusverb", nil, nil)
	if err == nil {
		t.Fatal("unknown verb returned nil (fail-open)")
	}
}
