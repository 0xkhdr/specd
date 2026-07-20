package gates

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core/scope"
)

// DiffScopeInput is everything the diff-scope check reasons over. Like every
// gate in this package the check itself is pure: the caller runs git, resolves
// the baseline, and reads the lease set, so the gate stays a function of its
// inputs and is trivially testable without a repository.
type DiffScopeInput struct {
	// TaskID and Baseline identify the mission whose scope is being checked.
	TaskID   string
	Baseline string

	// Changes is the complete diff against Baseline, including paths under
	// .specd/. It must NOT be pre-filtered: core.DeriveDiff strips .specd/,
	// which is exactly the class R4.3 exists to reject, so a caller that passes
	// its output here silently disables that rule.
	Changes []scope.Change

	// DeclaredPaths is the task's declared files. A path matches either exactly
	// or as a filepath.Match pattern, consistent with CheckScope.
	DeclaredPaths []string

	// BaselineIsAncestor reports whether Baseline is an ancestor of the current
	// HEAD. False means the worktree contains history that predates the mission
	// baseline (R4.2) — a rebase, a reset, or a mission issued against a tree
	// that has since been rewritten.
	BaselineIsAncestor bool

	// BaselineResolvable is false when the baseline commit does not exist in
	// this repository at all (rewritten history, shallow clone).
	BaselineResolvable bool

	// OtherLeaseScopes maps another active lease's id to the paths it holds.
	// Overlap means two missions are authorized to write the same file (R4.4).
	OtherLeaseScopes map[string][]string
}

// harnessProtected are the spec artifacts no task may edit directly, matched on
// basename under .specd/.
//
// The set is deliberately narrower than "everything under .specd/". specd's own
// verbs write runtime state into that tree during a normal task — `verify`
// appends to evidence.jsonl and state.json, `session open` writes
// driver-session.json — and those writes appear in the same worktree diff a
// mission is measured against. Flagging the whole directory would refuse the
// harness's own bookkeeping and make the loop unusable.
//
// What remains are the files specd never writes before this check runs:
// completion writes task markers only after diff-scope passes, and nothing in
// the execute phase rewrites requirements or design. So a change to one of
// these between baseline and completion was made by hand, which is what R4.3
// exists to catch — an agent marking its own task complete, or editing the
// acceptance criteria it is about to be judged against.
//
// ponytail: basename matching, not full-path. A task legitimately named
// tasks.md outside .specd/ is unaffected, but this cannot distinguish two specs'
// artifacts from each other. Tighten to a slug-aware path if a case appears
// where that matters.
var harnessProtected = []string{"tasks.md", "requirements.md", "design.md"}

// harnessProtectedPrefixes are subtrees of .specd/ that no task may edit.
// specd writes these at `init`, never during execution.
var harnessProtectedPrefixes = []string{".specd/roles/", ".specd/steering/"}

const specdPrefix = ".specd/"

// CheckDiffScope compares the whole diff against the declared scope and returns
// one finding per violation (R4.1 to R4.4).
//
// It only ever refuses. It cannot satisfy a gate, mark a task complete, or
// substitute for evidence, so no bypass can be built out of it. Findings are
// returned sorted so the same diff always reports in the same order.
func CheckDiffScope(input DiffScopeInput) []Finding {
	var findings []Finding
	add := func(format string, args ...any) {
		findings = append(findings, Finding{Gate: "diffscope", Severity: Error, Message: fmt.Sprintf(format, args...)})
	}

	if !input.BaselineResolvable {
		add("baseline %s does not resolve in this repository; dispatch a fresh mission for task %s rather than rebasing the existing one",
			input.Baseline, input.TaskID)
		// Every remaining rule is measured against the baseline. Continuing
		// would report violations derived from a reference point that does not
		// exist.
		return findings
	}
	// R4.2: the mission was planned against history the worktree no longer has.
	if !input.BaselineIsAncestor {
		add("baseline %s is not an ancestor of HEAD; the worktree contains changes that predate the mission baseline for task %s",
			input.Baseline, input.TaskID)
	}

	for _, change := range input.Changes {
		// R4.3: harness-owned state is never in scope, declared or not. Checked
		// before the declaration test so a task cannot legalize it by listing
		// the path in its files cell.
		if harnessOwned(change.Path) || harnessOwned(change.PreviousPath) {
			add("%s modifies harness-owned state %s; task markers, roles, and steering are written by specd verbs, never edited directly",
				input.TaskID, change.Path)
			continue
		}
		// Remaining .specd/ paths are the harness's own runtime ledgers, which
		// specd writes during the normal loop. They are neither declarable nor
		// violations — scope simply does not apply to them.
		if strings.HasPrefix(change.Path, specdPrefix) {
			continue
		}
		// R4.1: undeclared modify, create, delete, and rename all refuse. A
		// rename is checked at both ends, since renaming a declared file to an
		// undeclared path is still an undeclared write.
		if !declared(change.Path, input.DeclaredPaths) {
			add("%s is not declared by task %s (%s); declare the path in the task's files cell or narrow the change",
				change.Path, input.TaskID, kindLabel(change.Kind))
			continue
		}
		if change.PreviousPath != "" && !declared(change.PreviousPath, input.DeclaredPaths) {
			add("%s was renamed from undeclared path %s by task %s; both ends of a rename must be declared",
				change.Path, change.PreviousPath, input.TaskID)
		}
	}

	// R4.4: two missions authorized to write the same path race each other, and
	// the loser's work is silently overwritten.
	for _, leaseID := range sortedKeys(input.OtherLeaseScopes) {
		for _, held := range input.OtherLeaseScopes[leaseID] {
			for _, change := range input.Changes {
				if harnessOwned(change.Path) {
					continue
				}
				if change.Path == held || matches(held, change.Path) {
					add("%s overlaps active lease %s, which holds %s; wait for that lease to release or narrow the task scope",
						change.Path, leaseID, held)
				}
			}
		}
	}

	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings
}

// harnessOwned reports whether path is a spec artifact the harness owns. Only
// paths under .specd/ qualify, and only the protected basenames within it: the
// rest of that tree is harness runtime state that specd writes itself.
func harnessOwned(path string) bool {
	if !strings.HasPrefix(path, specdPrefix) {
		return false
	}
	for _, prefix := range harnessProtectedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	base := path[strings.LastIndex(path, "/")+1:]
	return slices.Contains(harnessProtected, base)
}

func declared(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == path || matches(pattern, path) {
			return true
		}
	}
	return false
}

func matches(pattern, path string) bool {
	ok, err := filepath.Match(pattern, path)
	return err == nil && ok
}

// kindLabel renders a git status letter as the word the requirement uses, so a
// refusal says "created" rather than "A".
func kindLabel(kind string) string {
	switch kind {
	case "A":
		return "created"
	case "D":
		return "deleted"
	case "M":
		return "modified"
	case "R":
		return "renamed"
	case "C":
		return "copied"
	case "untracked":
		return "created, untracked"
	}
	return kind
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
