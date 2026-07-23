package gates

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const (
	// commandPaletteFile is the canonical command-declaration file: verbs and
	// flags are declared once here and every dispatch/help/docs surface reads it.
	commandPaletteFile = "internal/core/commands.go"
	// gendocsSourceFile generates docs/command-reference.md from the palette;
	// keeping it in scope preserves palette↔reference parity.
	gendocsSourceFile = "tools/gendocs/main.go"
)

// paletteScope keeps a command-surface task's declared files consistent with the
// palette (spec R4.1/R4.3). A row that declares a CLI handler under
// internal/cmd/ is adding or changing a verb/flag, so it must also declare the
// canonical palette file (R4.1) and the generated-docs source (R4.3); otherwise
// a functional verb/flag could ship undeclared or with a stale command
// reference. Both gaps refuse the tasks gate, naming the row and the missing
// file. Pure over the row's declared paths; an empty CheckCtx yields no
// findings.
func paletteScope(ctx CheckCtx) []Finding {
	if !paletteScopeArmed(ctx.ApproveTarget) {
		return nil
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		paths, err := core.TaskDeclaredPaths(task)
		if err != nil {
			continue // a malformed files cell is the files gate's refusal, not this one
		}
		if !declaresCommandHandler(paths) {
			continue
		}
		declared := make(map[string]bool, len(paths))
		for _, p := range paths {
			declared[p] = true
		}
		if !declared[commandPaletteFile] {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s declares a CLI handler under internal/cmd/ but not the canonical command palette %s; a verb/flag-adding row must declare the palette so the new surface is validated at approval", task.ID, commandPaletteFile)})
		}
		if !declared[gendocsSourceFile] {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s declares a CLI handler under internal/cmd/ but not the generated-docs source %s; a verb/flag-adding row must declare it so the command reference stays in palette parity", task.ID, gendocsSourceFile)})
		}
	}
	return findings
}

// paletteScopeArmed arms the check ONLY at the tasks→executing approval, where
// command-surface scope is authored and R4.1/R4.3 belong. It deliberately does
// NOT arm at plain check (target ""), so specComplete's full-registry run does
// not Error on any historical spec whose plan legitimately declares an
// internal/cmd handler; the tasks-phase approval is where a verb/flag-adding row
// must declare the palette and gendocs source.
func paletteScopeArmed(target string) bool {
	return target == string(core.StatusTasks)
}

// declaresCommandHandler reports whether any declared path is a CLI handler
// source file. ponytail: a non-test .go file under internal/cmd/ is treated as
// a verb/flag registrar — the deterministic planning-time proxy for "registers
// a verb/flag" (the file may not exist on disk yet). This is conservative: a row
// that only edits an existing handler is also flagged, because there is no clean
// pure signal to derive the candidate verb from the filename — handlers are not
// named after verbs (runNew lives in lifecycle.go, runReport in report.go), so a
// path→palette-verb map would be brittle guesswork. Armed only at the
// tasks→executing approval (paletteScopeArmed), so the blast radius is the one
// gate where declaring the palette + gendocs source is the intended policy.
// Upgrade path: parse the file for a core.Commands/dispatch registration once
// new-file rows carry on-disk content at approval.
func declaresCommandHandler(paths []string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, "internal/cmd/") && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			return true
		}
	}
	return false
}

// UndocumentedFlags returns the handler-recognized flags absent from the command
// palette (spec R4.2): a flag a handler reads that no Command documents is
// functional but undocumented and must fail a deterministic lint. Pure and
// order-stable; documented is core.PaletteFlagNames().
func UndocumentedFlags(recognized []string, documented map[string]bool) []string {
	var out []string
	for _, f := range recognized {
		if !documented[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}
