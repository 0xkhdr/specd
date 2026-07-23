package gates

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

func CheckScope(changed, declared []string) error {
	for _, path := range changed {
		matched := false
		for _, pattern := range declared {
			if pattern == path {
				matched = true
				break
			}
			if ok, _ := filepath.Match(pattern, path); ok {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("outside_scope: %s is not declared", path)
		}
	}
	return nil
}

// productionKinds are the code-producing work kinds whose declared files are
// expected to be able to produce the behaviour their acceptance names (spec
// R5.2). docs/test/chore/spike/deferred carry no such obligation.
var productionKinds = map[string]bool{"feature": true, "fix": true, "refactor": true}

// acceptancePathRe matches a repo path token that names a Go source file
// (contains a slash and ends in .go), e.g. `internal/core/gates/scope.go`. It
// deliberately ignores non-path prose with slashes like `class/check-id` or
// `R5.1/R5.2` because those do not end in `.go`.
var acceptancePathRe = regexp.MustCompile(`[A-Za-z0-9_./-]+/[A-Za-z0-9_./-]+\.go`)

// acceptanceReach surfaces two scope-versus-acceptance mismatches at the
// tasks-phase readiness (spec R5). R5.1: for every requirement id a task cites,
// it scans the repository's Go sources under ctx.Root and — when that id is
// already referenced only in files the row does not declare — warns that the
// criterion may be unreachable within declared scope, naming the id and the
// referencing files. R5.2: a production-kind task whose acceptance names a Go
// path no declared file can produce yields a distinct scope-versus-acceptance
// error rather than a generic out-of-scope refusal mid-execution. Pure over the
// rows plus the Go sources under root; an empty root skips the R5.1 scan and an
// empty CheckCtx yields nothing (parity).
// ponytail: requirement ids are spec-relative, so a whole-repo scan can collide
// across specs (two specs each have an R5.1). The check is a warning and its
// unit is one id token; upgrade to spec-qualified ids if the collision noise
// ever needs to gate.
func acceptanceReach(ctx CheckCtx) []Finding {
	if !acceptanceReachArmed(ctx.ApproveTarget) {
		return nil
	}
	var findings []Finding
	findings = append(findings, acceptanceScopeFindings(ctx.Tasks)...) // R5.2 (no disk)
	if ctx.Root == "" {
		return findings
	}
	refToFiles := scanRefsInGoSources(ctx.Root, ctx.Tasks) // R5.1
	for _, task := range ctx.Tasks {
		declared := declaredSet(task)
		for _, id := range sortedUniqueRefs(task.Refs) {
			files := refToFiles[id]
			if len(files) == 0 {
				continue
			}
			inScope := false
			var outside []string
			for _, f := range files {
				if declared[f] {
					inScope = true
				} else {
					outside = append(outside, f)
				}
			}
			if !inScope {
				findings = append(findings, Finding{Severity: Warn, Message: fmt.Sprintf("%s acceptance cites %s but it is referenced only outside the task's declared files (%s); the criterion may be unreachable within declared scope", task.ID, id, strings.Join(outside, ", "))})
			}
		}
	}
	return findings
}

// acceptanceReachArmed arms the check for every readiness phase, mirroring
// dispatch-parity/quality-declaration so plain check and immediate approval
// cannot disagree.
func acceptanceReachArmed(target string) bool {
	switch target {
	case "", string(core.StatusRequirements), string(core.StatusDesign), string(core.StatusTasks), string(core.StatusExecuting), string(core.StatusVerifying), string(core.StatusComplete):
		return true
	}
	return false
}

// acceptanceScopeFindings implements R5.2: a production-kind row whose
// acceptance names a Go path that no declared file can produce (no declared
// file equals it or shares its directory) is a distinct scope-versus-acceptance
// error. Pure over the row — no disk.
func acceptanceScopeFindings(tasks []core.TaskRow) []Finding {
	var findings []Finding
	for _, task := range tasks {
		if !productionKinds[strings.ToLower(strings.TrimSpace(task.Kind))] {
			continue
		}
		paths, err := core.TaskDeclaredPaths(task)
		if err != nil {
			continue
		}
		var bad []string
		for _, tok := range acceptancePathRe.FindAllString(task.Acceptance, -1) {
			if !declaredCanProduce(tok, paths) {
				bad = append(bad, tok)
			}
		}
		sort.Strings(bad)
		for _, tok := range dedupe(bad) {
			findings = append(findings, Finding{Severity: Error, Message: fmt.Sprintf("%s scope-vs-acceptance: kind %q acceptance names %s but no declared file can produce it (declared: %s)", task.ID, task.Kind, tok, strings.Join(paths, ", "))})
		}
	}
	return findings
}

// declaredCanProduce reports whether any declared path equals the acceptance
// token or shares its directory (same package can produce the behaviour).
func declaredCanProduce(tok string, declared []string) bool {
	dir := path.Dir(tok)
	for _, p := range declared {
		if p == tok || path.Dir(p) == dir {
			return true
		}
	}
	return false
}

func declaredSet(task core.TaskRow) map[string]bool {
	paths, err := core.TaskDeclaredPaths(task)
	if err != nil {
		return nil
	}
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return set
}

// scanRefsInGoSources walks Go sources under root once and records, per cited
// requirement id, the repo-relative files that reference it. Skips VCS,
// vendored, and .specd trees; sorted, deterministic output.
func scanRefsInGoSources(root string, tasks []core.TaskRow) map[string][]string {
	patterns := map[string]*regexp.Regexp{}
	for _, task := range tasks {
		for _, id := range task.Refs {
			id = strings.TrimSpace(id)
			if id == "" || patterns[id] != nil {
				continue
			}
			patterns[id] = regexp.MustCompile(`\b` + regexp.QuoteMeta(id) + `\b`)
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	out := map[string][]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".specd", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		for id, re := range patterns {
			if re.Match(data) {
				out[id] = append(out[id], rel)
			}
		}
		return nil
	})
	for id := range out {
		sort.Strings(out[id])
	}
	return out
}

func sortedUniqueRefs(refs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range refs {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func dedupe(sorted []string) []string {
	var out []string
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}
