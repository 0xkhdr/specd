package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// runTaskOverride handles `specd task <id> --override --reason <text>`: a human
// clearance of an escalated task (spec 06 R3/R4). It resets the verify-failure
// ratchet but does NOT complete the task — the task still needs a passing verify
// record afterward (the no-bypass invariant). A missing reason is a usage error
// (exit 2); overriding a task that is not escalated is refused (exit 2).
func runTaskOverride(root, id string, flags map[string]string) error {
	reason := strings.TrimSpace(flags["reason"])
	if reason == "" {
		return fmt.Errorf("%w: task --override requires --reason <text>", ErrUsage)
	}
	slug, err := resolveSpecForTask(root, id)
	if err != nil {
		return err
	}

	var priorCount int
	_, err = core.WithSpecLock(root, func() (struct{}, error) {
		count, err := taskFailCount(root, slug, id)
		if err != nil {
			return struct{}{}, err
		}
		if !core.IsEscalated(count, escalationMaxFails(root)) {
			return struct{}{}, fmt.Errorf("%w: task %s is not escalated (%d consecutive verify fails); override only clears an escalated task", ErrUsage, id, count)
		}
		priorCount = count
		record := core.OverrideRecord{TaskID: id, Reason: reason, PriorFailCount: count}
		return struct{}{}, core.AppendOverride(core.OverridePath(root, slug), record)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "override recorded for %s %s (cleared %d consecutive verify fails); run `specd verify %s %s` to re-attempt\n", slug, id, priorCount, slug, id)
	return nil
}

// resolveSpecForTask finds the single spec whose tasks.md declares task id.
// Ambiguity (the same id in more than one spec) or absence is a usage error so
// an override never lands against the wrong spec.
func resolveSpecForTask(root, id string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(core.SpecdDir(root), "specs"))
	if err != nil {
		return "", err
	}
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		spec, err := loadSpec(root, entry.Name())
		if err != nil {
			continue
		}
		for _, task := range spec.Tasks {
			if task.ID == id {
				matches = append(matches, entry.Name())
				break
			}
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%w: no spec declares task %s", ErrUsage, id)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%w: task %s is ambiguous across specs %v; disambiguate by clearing it per spec", ErrUsage, id, matches)
	}
}
