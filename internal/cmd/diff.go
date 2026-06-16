package cmd

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// ArtifactChange is one artifact file that changed between two git refs.
type ArtifactChange struct {
	Path   string `json:"path"`   // repo-relative path
	Status string `json:"status"` // "added" | "modified" | "deleted" | "renamed"
}

// RunDiff shows how a spec's on-disk artifacts changed between two git refs. It
// is strictly read-only — a thin, deterministic wrapper over `git diff
// --name-status` scoped to the spec directory. `--from` is required; `--to`
// defaults to the working tree. Bad refs or a non-git repo are reported as
// errors, never panics, and a spec with no changes is an empty (not failing)
// result. Text by default; a typed JSON object under SPECD_JSON.
func RunDiff(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd diff <slug> --from <ref> [--to <ref>]")
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	from := strings.TrimSpace(args.Str("from"))
	if from == "" {
		return usageExit("usage: specd diff <slug> --from <ref> [--to <ref>]")
	}
	to := strings.TrimSpace(args.Str("to"))

	// Scope the diff to the spec's artifact directory (repo-relative pathspec).
	pathspec := ".specd/specs/" + slug

	gitArgs := []string{"-C", root, "diff", "--name-status", "--no-color", from}
	if to != "" {
		gitArgs = append(gitArgs, to)
	}
	gitArgs = append(gitArgs, "--", pathspec)

	out, err := exec.Command("git", gitArgs...).CombinedOutput()
	if err != nil {
		return specdExit(core.GateError(fmt.Sprintf("git diff failed (is this a git repo, and are %q/%q valid refs?): %s", from, to, strings.TrimSpace(string(out)))))
	}

	changes := parseNameStatus(string(out))

	if core.IsJSONMode() {
		if changes == nil {
			changes = []ArtifactChange{}
		}
		if err := core.PrintJSON(struct {
			Spec    string           `json:"spec"`
			From    string           `json:"from"`
			To      string           `json:"to"`
			Changes []ArtifactChange `json:"changes"`
		}{slug, from, toOrWorkingTree(to), changes}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	fmt.Printf("diff — %s  %s..%s  (%d artifact change%s)\n", slug, from, toOrWorkingTree(to), len(changes), plural(len(changes)))
	for _, c := range changes {
		fmt.Printf("  %-9s %s\n", c.Status, c.Path)
	}
	return core.ExitOK
}

func toOrWorkingTree(to string) string {
	if to == "" {
		return "(working tree)"
	}
	return to
}

// parseNameStatus turns `git diff --name-status` output into a sorted, stable
// slice of ArtifactChange. Unknown status letters are passed through verbatim so
// the output never silently drops a change. Output is sorted by path for
// determinism regardless of git's internal ordering.
func parseNameStatus(raw string) []ArtifactChange {
	var changes []ArtifactChange
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		code := fields[0]
		// Renames/copies are "R100"/"C75" with old and new paths; report the new path.
		path := fields[len(fields)-1]
		changes = append(changes, ArtifactChange{Path: path, Status: statusWord(code)})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func statusWord(code string) string {
	if code == "" {
		return "changed"
	}
	switch code[0] {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	default:
		return strings.ToLower(code)
	}
}
