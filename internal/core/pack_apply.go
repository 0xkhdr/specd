package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// PackApplyResult reports what an ApplyPack call did. Written lists the
// repo-relative paths created; Skipped lists declared files that already existed
// (only populated when force is false).
type PackApplyResult struct {
	Pack    string
	Written []string
	Skipped []string
}

// ApplyPack writes a resolved pack's files under root. It is transactional: it
// pre-checks the whole plan (paths are re-validated, collisions detected) before
// writing anything, and if any write fails it removes every file it created in
// this call — so a failed apply never leaves a partial scaffold. Without force,
// a pre-existing target is a hard error (fail-closed) rather than an overwrite,
// keeping the apply all-or-nothing. Vars are substituted into file content.
func ApplyPack(root string, p *Pack, force bool) (PackApplyResult, error) {
	res := PackApplyResult{Pack: p.Name}

	// Plan phase: validate paths and detect collisions with no side effects.
	type planned struct {
		abs, rel, content string
	}
	plan := make([]planned, 0, len(p.Files))
	for _, f := range p.Files {
		if err := validatePackPath(f.Path); err != nil {
			return res, err
		}
		abs := filepath.Join(root, filepath.FromSlash(f.Path))
		if _, err := os.Stat(abs); err == nil {
			if !force {
				return res, GateError(fmt.Sprintf("pack %q: %s already exists (use --force to overwrite) — nothing written", p.Name, f.Path))
			}
			res.Skipped = append(res.Skipped, f.Path)
		}
		plan = append(plan, planned{abs: abs, rel: f.Path, content: ApplyVars(f.Content, p.Vars)})
	}

	// Apply phase: write each file, rolling back creations on the first failure.
	var created []string
	rollback := func() {
		for _, c := range created {
			_ = os.Remove(c)
		}
	}
	for _, pl := range plan {
		preexisting := false
		if _, err := os.Stat(pl.abs); err == nil {
			preexisting = true
		}
		if err := AtomicWrite(pl.abs, pl.content); err != nil {
			rollback()
			return PackApplyResult{Pack: p.Name}, GateError(fmt.Sprintf("pack %q: write %s failed (%v) — rolled back", p.Name, pl.rel, err))
		}
		// Only roll back files this call newly created; never delete a file the
		// user already had (force-overwrites are left in place on rollback rather
		// than destroyed).
		if !preexisting {
			created = append(created, pl.abs)
		}
		res.Written = append(res.Written, pl.rel)
	}
	return res, nil
}
