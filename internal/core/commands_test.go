package core_test

import (
	"reflect"
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
