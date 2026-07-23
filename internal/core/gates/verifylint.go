package gates

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// verifyLint flags shell patterns in verify cells that behave unreliably under
// the deterministic runner (spec R8.1): job-control `kill %N` (sh -c runs
// without an interactive job table), and a trailing `&` that backgrounds the
// command without capturing `$!` (the verify exit code then never observes the
// backgrounded work). It is a pure, deterministic string lint at warning
// severity — it reports, it never blocks — armed for plain `specd check` and
// the tasks-phase approval. Clean commands pass silently.
func verifyLint(ctx CheckCtx) []Finding {
	if !verifyLintArmed(ctx.ApproveTarget) {
		return nil
	}
	var findings []Finding
	for _, task := range ctx.Tasks {
		cmd := strings.TrimSpace(task.Verify)
		if cmd == "" {
			continue
		}
		if strings.Contains(cmd, "kill %") {
			findings = append(findings, Finding{Severity: Warn, Message: fmt.Sprintf("%s verify uses job-control `kill %%N`; sh -c has no job table — capture the PID (`$!`) and kill that instead", task.ID)})
		}
		if strings.HasSuffix(cmd, "&") && !strings.HasSuffix(cmd, "&&") && !strings.Contains(cmd, "$!") {
			findings = append(findings, Finding{Severity: Warn, Message: fmt.Sprintf("%s verify ends with a backgrounding `&` without capturing `$!`; the verify exit code never observes the backgrounded command", task.ID)})
		}
		findings = append(findings, runSelectorFindings(ctx.Root, task, cmd)...)
	}
	return findings
}

var testFuncRe = regexp.MustCompile(`(?m)^func (Test\w+)`)

// runSelectorFindings couples a Go `-run` test selector to the row's declared
// test files (spec R2.1/R2.2). A selector naming a test needs at least one
// declared *_test.go in the row or the tasks gate refuses (R2.1); when those
// files are readable, a selector matching no `func Test...` in them is refused
// too (R2.2). Both are Error severity — a verify line that can never execute a
// named test must not pass approval. Pure over the row plus the declared files
// under root; an empty root skips the R2.2 file read (parity: no root ⇒ no
// findings beyond the file-presence check).
func runSelectorFindings(root string, task core.TaskRow, cmd string) []Finding {
	selector, ok := goTestRunSelector(cmd)
	if !ok {
		return nil
	}
	paths, err := core.TaskDeclaredPaths(task)
	if err != nil {
		return nil // a malformed files cell is the files gate's refusal, not this one
	}
	var testFiles []string
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			testFiles = append(testFiles, p)
		}
	}
	if len(testFiles) == 0 {
		return []Finding{{Severity: Error, Message: fmt.Sprintf("%s verify runs `-run %s` but declares no *_test.go file; a run selector naming a test must declare its test file in the row", task.ID, selector)}}
	}
	if root == "" {
		return nil
	}
	// go test -run matches each slash-separated name element unanchored; the
	// top-level Test name is the first segment. ponytail: match the first segment
	// only — subtest-path selectors resolve on the parent Test, which is enough
	// to prove the row declares the test that runs.
	re, err := regexp.Compile(strings.SplitN(selector, "/", 2)[0])
	if err != nil {
		return nil // an invalid -run regexp is out of this gate's scope
	}
	for _, p := range testFiles {
		data, readErr := os.ReadFile(filepath.Join(root, p))
		if readErr != nil {
			continue
		}
		for _, m := range testFuncRe.FindAllSubmatch(data, -1) {
			if re.MatchString(string(m[1])) {
				return nil
			}
		}
	}
	return []Finding{{Severity: Error, Message: fmt.Sprintf("%s verify `-run %s` matches no Test function in the declared test files %v", task.ID, selector, testFiles)}}
}

// goTestRunSelector extracts the `-run <pattern>` (or `-run=<pattern>`) value
// from a `go test` command, stripping one layer of surrounding quotes.
func goTestRunSelector(cmd string) (string, bool) {
	if !strings.Contains(cmd, "go test") {
		return "", false
	}
	fields := strings.Fields(cmd)
	for i, f := range fields {
		if v, ok := strings.CutPrefix(f, "-run="); ok {
			return strings.Trim(v, `"'`), true
		}
		if f == "-run" && i+1 < len(fields) {
			return strings.Trim(fields[i+1], `"'`), true
		}
	}
	return "", false
}

// verifyLintArmed arms the lint for plain `specd check` (empty target) and the
// tasks-phase approval, where verify cells are authored. Later transitions
// re-run `specd check` anyway; earlier phases have no tasks table to lint.
func verifyLintArmed(target string) bool {
	switch target {
	case "", string(core.StatusTasks):
		return true
	}
	return false
}
