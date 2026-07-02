package cmd

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestCommandReferenceMatchesRegistry keeps docs/command-reference.md aligned
// with the actual command surface: dispatch Registry plus the three commands
// routed directly by main.go. It catches stale removed commands and newly added
// commands missing from the public reference.
func TestCommandReferenceMatchesRegistry(t *testing.T) {
	docBytes, err := os.ReadFile("../../docs/command-reference.md")
	if err != nil {
		t.Fatalf("read command reference: %v", err)
	}
	doc := string(docBytes)

	for _, stale := range []string{"`specd doctor`", "`specd migrate`"} {
		if strings.Contains(doc, stale) {
			t.Fatalf("command reference contains stale command %s", stale)
		}
	}

	want := map[string]bool{"help": true, "version": true, "mcp": true}
	for _, registered := range Registry {
		want[registered.Name] = true
	}

	got := documentedCheatSheetCommands(t, doc)
	for name := range want {
		if !got[name] {
			t.Errorf("command reference cheat sheet missing `specd %s`", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("command reference cheat sheet documents unknown command `specd %s`", name)
		}
	}
}

func documentedCheatSheetCommands(t *testing.T, doc string) map[string]bool {
	t.Helper()
	start := strings.Index(doc, "## Cheat sheet")
	end := strings.Index(doc, "## Daily workflow commands")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("command reference missing cheat sheet section")
	}

	section := doc[start:end]
	re := regexp.MustCompile("`specd ([a-z][a-z-]*)`")
	matches := re.FindAllStringSubmatch(section, -1)
	if len(matches) == 0 {
		t.Fatalf("command reference cheat sheet lists no commands")
	}

	out := make(map[string]bool, len(matches))
	for _, match := range matches {
		out[match[1]] = true
	}
	return out
}

func TestCommandReferenceRequiredCommandsHaveDetailRows(t *testing.T) {
	docBytes, err := os.ReadFile("../../docs/command-reference.md")
	if err != nil {
		t.Fatalf("read command reference: %v", err)
	}
	doc := string(docBytes)

	commands := make([]string, 0, len(Registry)+3)
	for _, registered := range Registry {
		commands = append(commands, registered.Name)
	}
	commands = append(commands, "help", "version", "mcp")
	sort.Strings(commands)

	for _, name := range commands {
		usageCell := "| `specd " + name
		if !strings.Contains(doc, usageCell) {
			t.Errorf("command reference missing detail row for `specd %s`", name)
		}
	}
}
