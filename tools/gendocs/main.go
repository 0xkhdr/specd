// Command gendocs single-sources docs/command-reference.md from the specd
// command palette (internal/core.Commands). It is a build-time tool and is not
// imported by the shipped binary.
//
// Modes:
//
//	go run ./tools/gendocs           write docs/command-reference.md
//	go run ./tools/gendocs -check    fail (exit 1) if the committed doc drifts
//
// Output is a pure function of the palette: no timestamps, no map iteration.
// Commands are sorted by Name and flags by Name, so the same palette always
// renders byte-identical output.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

func main() {
	check := flag.Bool("check", false, "verify the committed doc matches the palette instead of writing it")
	flag.Parse()

	out := render()
	path := docPath()

	if *check {
		committed, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gendocs -check: %v\n", err)
			os.Exit(1)
		}
		if !bytes.Equal(committed, out) {
			fmt.Fprintf(os.Stderr, "gendocs -check: %s is stale; regenerate with `go run ./tools/gendocs`\n", filepath.Base(path))
			os.Exit(1)
		}
		return
	}

	if err := os.WriteFile(path, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		os.Exit(1)
	}
}

// docPath resolves docs/command-reference.md relative to this source file so the
// tool works regardless of the caller's working directory.
func docPath() string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		panic("gendocs: cannot locate source file")
	}
	return filepath.Join(filepath.Dir(self), "..", "..", "docs", "command-reference.md")
}

func render() []byte {
	cmds := append([]core.Command(nil), core.Commands...)
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })

	var b strings.Builder
	fmt.Fprintf(&b, "# specd — Command Reference\n\n")
	fmt.Fprintf(&b, "<!-- GENERATED FILE — do not edit by hand.\n")
	fmt.Fprintf(&b, "     Source of truth: internal/core/commands.go (the `specd help --json` palette, schema version %d).\n", core.HelpSchemaVersion)
	fmt.Fprintf(&b, "     Regenerate with: go run ./tools/gendocs -->\n\n")
	fmt.Fprintf(&b, "> **Status:** Normative documentation for current `specd` behavior, generated from the\n")
	fmt.Fprintf(&b, "> command palette (`specd help --json`, schema version %d).\n\n", core.HelpSchemaVersion)

	b.WriteString("## Conventions\n\n")
	b.WriteString("`specd <verb> [args] [flags]`. Run `specd help` for the live palette, or `specd help <verb>`\n")
	b.WriteString("for one command. `specd help --json` emits the machine-readable palette this page is generated from.\n\n")
	b.WriteString("Unknown verbs and disallowed flag values fail closed (exit 2). Deferred verbs print a notice and exit 0.\n\n")
	b.WriteString("**Exit codes** (the standard convention; per-verb deviations are noted on the verb):\n\n")
	b.WriteString("| Code | Meaning |\n|---|---|\n")
	for _, ec := range exitCodeUnion(cmds) {
		fmt.Fprintf(&b, "| `%d` | %s |\n", ec.Code, ec.Meaning)
	}
	b.WriteString("\n")

	b.WriteString("## Commands\n")
	for _, c := range cmds {
		renderCommand(&b, c)
	}

	return []byte(b.String())
}

// exitCodeUnion collects every exit code across all commands, de-duplicated by
// (code, meaning) and sorted by code then meaning.
func exitCodeUnion(cmds []core.Command) []core.ExitCode {
	seen := map[core.ExitCode]bool{}
	var union []core.ExitCode
	for _, c := range cmds {
		for _, ec := range c.ExitCodes {
			if !seen[ec] {
				seen[ec] = true
				union = append(union, ec)
			}
		}
	}
	sort.Slice(union, func(i, j int) bool {
		if union[i].Code != union[j].Code {
			return union[i].Code < union[j].Code
		}
		return union[i].Meaning < union[j].Meaning
	})
	return union
}

func renderCommand(b *strings.Builder, c core.Command) {
	// Heading stays a bare ``### `name` `` so the CLI-surface test
	// (internal/cmd/surface_test.go) can parse every verb; Deferred/HumanOnly
	// markers ride on the Phases line as bold tags instead.
	fmt.Fprintf(b, "\n### `%s`\n\n", c.Name)

	fmt.Fprintf(b, "```\n%s\n```\n", c.Usage)

	if c.Description != "" {
		fmt.Fprintf(b, "\n%s\n", c.Description)
	}

	if len(c.AllowedPhases) > 0 {
		phases := make([]string, len(c.AllowedPhases))
		for i, p := range c.AllowedPhases {
			phases[i] = string(p)
		}
		line := fmt.Sprintf("**Phases:** %s.", strings.Join(phases, " · "))
		if c.Deferred {
			line += " **Deferred.**"
		}
		if c.HumanOnly {
			line += " **Human only.**"
		}
		fmt.Fprintf(b, "\n%s\n", line)
	}

	if len(c.Flags) > 0 {
		flags := append([]core.Flag(nil), c.Flags...)
		sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
		b.WriteString("\n| Flag | Value | Description |\n|---|---|---|\n")
		for _, f := range flags {
			fmt.Fprintf(b, "| `--%s` | %s | %s |\n", f.Name, flagValue(f), f.Description)
		}
	}

	if len(c.Examples) > 0 {
		b.WriteString("\n**Examples:**\n\n```bash\n")
		for _, ex := range c.Examples {
			b.WriteString(ex + "\n")
		}
		b.WriteString("```\n")
	}
}

// flagValue renders the value column: prefer the human-readable Values note,
// else the declared type, else "bool".
func flagValue(f core.Flag) string {
	switch {
	case f.Values != "":
		return f.Values
	case f.Type != "":
		return f.Type
	default:
		return "bool"
	}
}
