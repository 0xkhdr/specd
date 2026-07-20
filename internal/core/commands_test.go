package core_test

import (
	"reflect"
	"strings"
	"testing"

	command "github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
)

func TestRegistryMatchesHelp(t *testing.T) {
	seen := make(map[string]bool)
	for _, meta := range core.Commands {
		if meta.Name == "" {
			t.Fatal("command metadata has empty name")
		}
		if meta.Usage == "" {
			t.Fatalf("%s usage is empty", meta.Name)
		}
		if meta.Description == "" {
			t.Fatalf("%s description is empty", meta.Name)
		}
		if seen[meta.Name] {
			t.Fatalf("duplicate command metadata for %s", meta.Name)
		}
		seen[meta.Name] = true
	}

	var registryNames []string
	for name := range command.Registry {
		registryNames = append(registryNames, name)
	}

	helpNames := core.CommandNames()
	if !sameSet(registryNames, helpNames) {
		t.Fatalf("registry commands = %v, help commands = %v", registryNames, helpNames)
	}
	if !reflect.DeepEqual(command.RegisteredCommandNames(), helpNames) {
		t.Fatalf("registry help order = %v, want %v", command.RegisteredCommandNames(), helpNames)
	}
}

func TestCommandsFlagValueShapes(t *testing.T) {
	commands := map[string]core.Command{}
	for _, command := range core.Commands {
		commands[command.Name] = command
	}
	findFlag := func(command, name string) core.Flag {
		t.Helper()
		for _, flag := range commands[command].Flags {
			if flag.Name == name {
				return flag
			}
		}
		t.Fatalf("%s missing --%s", command, name)
		return core.Flag{}
	}
	if values := findFlag("eval", "check").Values; !strings.Contains(values, "test|output_eval|trajectory_eval|review") {
		t.Fatalf("eval --check values omit evidence-class enum: %q", values)
	}
	if values := findFlag("verify", "status").Values; values != "pass|fail" {
		t.Fatalf("verify --status values = %q", values)
	}
	if !strings.Contains(commands["brain"].Description, "minted by brain dispatch") || !strings.Contains(commands["brain"].Description, "brain status") {
		t.Fatalf("brain help omits mission-id provenance: %q", commands["brain"].Description)
	}
}

// TestSpecSlugArgPositions is the slug-position regression (SPEC-02 T-02-04):
// dispatch resolves the spec slug from a fixed argv index, and every verb that
// resolves one must read it from the right place. `brain` takes a subcommand
// first (`specd brain start <spec>`), so its slug is argAt(1); every other
// slug-bearing verb reads argAt(0). A verb resolving no fixed-position slug
// leaves SpecSlugArg nil and is skipped by the phase gate. This pins that
// contract so a future edit can't silently shift an index and mis-phase-check.
func TestSpecSlugArgPositions(t *testing.T) {
	for _, meta := range core.Commands {
		if meta.SpecSlugArg == nil {
			continue
		}
		want := 0
		// Subcommand-first verbs carry their slug at argAt(1): `brain start <spec>`,
		// `release candidate <spec>`, and `session open <spec>`. Every other
		// slug-bearing verb reads argAt(0).
		if meta.Name == "brain" || meta.Name == "release" || meta.Name == "session" {
			want = 1
		}
		if got := *meta.SpecSlugArg; got != want {
			t.Errorf("%s: SpecSlugArg = argAt(%d), want argAt(%d)", meta.Name, got, want)
		}
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, item := range a {
		counts[item]++
	}
	for _, item := range b {
		counts[item]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
