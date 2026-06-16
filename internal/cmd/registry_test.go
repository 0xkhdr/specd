package cmd

import (
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// metaOnly are core.Commands entries that are handled in main.run before
// dispatch (not Registry handlers).
var metaOnly = map[string]bool{"help": true, "version": true, "mcp": true}

// TestRegistryMatchesHelp asserts the dispatch Registry and the help metadata
// (core.Commands) describe exactly the same command set. This is the guarantee
// that `specd help` can never list a command that does not dispatch, and that
// no dispatchable command is missing from help.
func TestRegistryMatchesHelp(t *testing.T) {
	inRegistry := map[string]bool{}
	for _, c := range Registry {
		if inRegistry[c.Name] {
			t.Errorf("duplicate command %q in Registry", c.Name)
		}
		inRegistry[c.Name] = true
	}

	inHelp := map[string]bool{}
	for _, c := range core.Commands {
		if metaOnly[c.Command] {
			continue
		}
		inHelp[c.Command] = true
	}

	for name := range inRegistry {
		if !inHelp[name] {
			t.Errorf("command %q in Registry but missing from core.Commands help metadata", name)
		}
	}
	for name := range inHelp {
		if !inRegistry[name] {
			t.Errorf("command %q in core.Commands but missing from dispatch Registry", name)
		}
	}
}

// TestRegistryHandlersNonNil guards against a nil handler slipping into the
// dispatch table.
func TestRegistryHandlersNonNil(t *testing.T) {
	for _, c := range Registry {
		if c.Run == nil {
			t.Errorf("command %q has a nil Run handler", c.Name)
		}
	}
}

// TestDispatchUnknownCommand confirms an unregistered command is reported as
// not-found by Dispatch (the caller renders help + exits ExitUsage).
func TestDispatchUnknownCommand(t *testing.T) {
	if _, ok := Dispatch("definitely-not-a-command", cli.Args{}); ok {
		t.Error("Dispatch should report unknown command as not found")
	}
}
