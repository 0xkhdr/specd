package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

// specComplete is the shared completion predicate (spec 12): a spec is complete
// when every gate is green and every task is complete — the same check `submit`
// enforces (one predicate, no second definition of "done").
func specComplete(root, slug string) bool {
	spec, err := loadSpec(root, slug)
	if err != nil {
		return false
	}
	gateFailures := gateFailureMessages(gates.CoreRegistry().Run(buildCheckCtx(root, slug, spec, "")))
	model, err := reportModel(root, slug)
	if err != nil {
		return false
	}
	return len(core.SubmitBlockers(model, gateFailures)) == 0
}

// specExists reports whether a spec workspace exists for slug.
func specExists(root, slug string) bool {
	info, err := os.Stat(filepath.Join(core.SpecdDir(root), "specs", slug))
	return err == nil && info.IsDir()
}

// runLink records that <from> depends on <to> (to must complete first). A
// nonexistent slug fails closed (exit 2); a link that would create a cycle in the
// cross-spec graph is refused (exit 1) with the cycle path (spec 12 R1, R2).
func runLink(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return fmt.Errorf("%w: specd link <from-slug> <to-slug>", ErrUsage)
	}
	from, to := args[0], args[1]
	kind := core.LinkKind(strings.TrimSpace(flags["kind"]))
	if kind == "" {
		kind = core.LinkKindFollows
	}
	if !kind.Valid() {
		return fmt.Errorf("%w: unknown link kind %q", ErrUsage, kind)
	}
	reason := strings.TrimSpace(flags["reason"])
	if from == to {
		return fmt.Errorf("%w: a spec cannot depend on itself", ErrUsage)
	}
	// Validate both slugs before any existence check: a valid-but-nonexistent
	// slug must not short-circuit before an unsafe traversal slug is rejected
	// (the traversal guard has to fire regardless of argument order).
	for _, slug := range []string{from, to} {
		if err := core.ValidateSlug(slug); err != nil {
			return fmt.Errorf("%w: %v", ErrUsage, err)
		}
	}
	for _, slug := range []string{from, to} {
		if !specExists(root, slug) {
			return fmt.Errorf("%w: no such spec %q", ErrUsage, slug)
		}
	}
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		path := core.ProgramPath(root)
		program, err := core.LoadProgram(path)
		if err != nil {
			return struct{}{}, err
		}
		if program.HasLink(from, to) {
			return struct{}{}, nil // idempotent
		}
		if cycle := program.WouldCycle(from, to); cycle != nil {
			return struct{}{}, fmt.Errorf("link refused: would create a cycle: %s", strings.Join(cycle, " → "))
		}
		if err := program.AddTypedLink(from, to, kind, reason); err != nil {
			return struct{}{}, fmt.Errorf("%w: %v", ErrUsage, err)
		}
		return struct{}{}, core.SaveProgram(path, program)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "linked %s → %s (kind=%s; %s depends on %s)\n", from, to, kind, from, to)
	return nil
}

// runUnlink removes a from→to link; removing a nonexistent link fails closed
// (exit 2) (spec 12 R3).
func runUnlink(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return fmt.Errorf("%w: specd unlink <from-slug> <to-slug>", ErrUsage)
	}
	from, to := args[0], args[1]
	removed := false
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		path := core.ProgramPath(root)
		program, err := core.LoadProgram(path)
		if err != nil {
			return struct{}{}, err
		}
		if !program.RemoveLink(from, to) {
			return struct{}{}, nil
		}
		removed = true
		return struct{}{}, core.SaveProgram(path, program)
	})
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("%w: no link %s → %s to remove", ErrUsage, from, to)
	}
	fmt.Fprintf(os.Stdout, "unlinked %s → %s\n", from, to)
	return nil
}

// renderProgram builds the `status --program` view: every spec with its phase,
// its dependency links, and the actionable frontier (spec 12 R4).
func renderProgram(root string) (string, error) {
	specs := core.ListSpecs(root)
	program, err := core.LoadProgram(core.ProgramPath(root))
	if err != nil {
		return "", err
	}
	complete := func(slug string) bool { return specComplete(root, slug) }

	var b strings.Builder
	b.WriteString("program specs:\n")
	for _, slug := range specs {
		phase := "unknown"
		if state, err := core.LoadState(core.StatePath(root, slug)); err == nil {
			phase = string(state.Phase)
		}
		done := ""
		if complete(slug) {
			done = " (complete)"
		}
		fmt.Fprintf(&b, "  %s  phase=%s%s\n", slug, phase, done)
		for _, dep := range program.Deps(slug) {
			mark := "pending"
			if complete(dep) {
				mark = "complete"
			}
			fmt.Fprintf(&b, "    depends on %s [%s]\n", dep, mark)
		}
	}
	frontier := program.Frontier(specs, complete)
	fmt.Fprintf(&b, "program frontier (actionable now): %s\n", strings.Join(frontier, ", "))
	return b.String(), nil
}
