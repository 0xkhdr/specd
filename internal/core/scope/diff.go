package scope

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Change struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path,omitempty"`
	Kind         string `json:"kind"`
	OldMode      string `json:"old_mode,omitempty"`
	NewMode      string `json:"new_mode,omitempty"`
}
type Diff struct {
	Baseline string   `json:"baseline"`
	Changes  []Change `json:"changes"`
	Paths    []string `json:"paths"`
}

func Derive(root, baseline string) (Diff, error) {
	if strings.TrimSpace(baseline) == "" || baseline == "unknown" {
		return Diff{}, fmt.Errorf("DIFF_BASELINE_INVALID")
	}
	out, err := exec.Command("git", "-C", root, "diff", "--name-status", "-z", "--find-renames", baseline, "--").Output()
	if err != nil {
		return Diff{}, fmt.Errorf("derive tracked diff: %w", err)
	}
	parts := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	var changes []Change
	for i := 0; i < len(parts) && len(parts[i]) > 0; i++ {
		status := string(parts[i])
		i++
		if i >= len(parts) {
			return Diff{}, fmt.Errorf("malformed git diff")
		}
		p := string(parts[i])
		kind := status[:1]
		c := Change{Path: p, Kind: kind}
		if kind == "R" || kind == "C" {
			c.PreviousPath = p
			i++
			if i >= len(parts) {
				return Diff{}, fmt.Errorf("malformed rename")
			}
			c.Path = string(parts[i])
		}
		changes = append(changes, c)
	}
	untracked, err := exec.Command("git", "-C", root, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return Diff{}, fmt.Errorf("derive untracked diff: %w", err)
	}
	for _, b := range bytes.Split(bytes.TrimSuffix(untracked, []byte{0}), []byte{0}) {
		if len(b) > 0 {
			changes = append(changes, Change{Path: string(b), Kind: "untracked"})
		}
	}
	modeOut, _ := exec.Command("git", "-C", root, "diff", "--summary", baseline, "--").Output()
	for _, line := range strings.Split(string(modeOut), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 6 && fields[0] == "mode" && fields[1] == "change" && fields[3] == "=>" {
			for i := range changes {
				if changes[i].Path == fields[5] {
					changes[i].OldMode, changes[i].NewMode = fields[2], fields[4]
				}
			}
		}
	}
	set := map[string]bool{}
	for i := range changes {
		p, err := Normalize(changes[i].Path)
		if err != nil {
			return Diff{}, err
		}
		changes[i].Path = p
		set[p] = true
		if changes[i].PreviousPath != "" {
			old, err := Normalize(changes[i].PreviousPath)
			if err != nil {
				return Diff{}, err
			}
			changes[i].PreviousPath = old
			set[old] = true
		}
		if err := rejectEscape(root, p); err != nil {
			return Diff{}, err
		}
		if isSubmodule(root, p) {
			return Diff{}, fmt.Errorf("submodule %s cannot be task scope", p)
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return Diff{Baseline: baseline, Changes: changes, Paths: paths}, nil
}

func isSubmodule(root, rel string) bool {
	out, err := exec.Command("git", "-C", root, "ls-files", "-s", "--", rel).Output()
	return err == nil && strings.HasPrefix(string(out), "160000 ")
}

func rejectEscape(root, rel string) error {
	full := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(full)
		if err != nil {
			return fmt.Errorf("symlink %s: %w", rel, err)
		}
		absRoot, _ := filepath.Abs(root)
		absTarget, _ := filepath.Abs(target)
		if absTarget != absRoot && !strings.HasPrefix(absTarget, absRoot+string(os.PathSeparator)) {
			return fmt.Errorf("symlink %s escapes repository", rel)
		}
	}
	return nil
}
