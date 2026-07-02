package security

import (
	"fmt"
	"regexp"
	"strings"
)

// injection.go flags two classic injection anti-patterns with deterministic
// heuristics: SQL built by string concatenation/interpolation, and shell/exec
// calls fed interpolated input. It is advisory by default (plan risk 2) — the
// heuristics are intentionally conservative (they require a SQL keyword or a
// known exec sink) to keep the false-positive rate low.

// sqlStringRe matches a *quoted string that begins a SQL statement* — the quote
// (optionally prefixed by a Python f/format marker) immediately followed by a SQL
// command verb. Anchoring on the quoted verb (rather than a bare keyword) is what
// keeps ordinary English/identifiers like "from" or "update" from tripping the
// gate — the key false-positive lesson (plan risk 2).
var sqlStringRe = regexp.MustCompile(`(?i)[frb]?["'` + "`" + `]\s*(SELECT\s|INSERT\s+INTO|UPDATE\s|DELETE\s+FROM|DROP\s+TABLE|CREATE\s+TABLE|ALTER\s+TABLE)`)

// concatRe detects string concatenation or interpolation on the line: Go/JS `+`
// against a quote, Python `%`/f-string/`.format`, or template `${}`. A SQL string
// is only a finding when the statement is *built* this way.
var concatRe = regexp.MustCompile(`["'` + "`" + `]\s*\+|\+\s*["'` + "`" + `]|%\s*[\(s]|\$\{|\bf["']|\.format\(|\{[a-zA-Z_]`)

// execSinkRe matches a known command-exec sink taking an interpolated argument.
var execSinkRe = regexp.MustCompile(`(?i)\b(exec|system|popen|os\.system|subprocess\.(?:call|run|Popen)|child_process\.(?:exec|execSync)|eval)\s*\(`)

// interpolatedArgRe detects interpolation/concatenation inside a call argument.
var interpolatedArgRe = regexp.MustCompile(`["'` + "`" + `]\s*\+|\+\s*["'` + "`" + `]|\$\{|%s|f["']|\.format\(|` + "`" + `[^` + "`" + `]*\$\{`)

// ScanInjection returns injection findings across the changed files. Pure over
// the supplied contents.
func ScanInjection(files []ChangedFile) []Finding {
	var out []Finding
	for _, f := range files {
		for i, line := range strings.Split(f.Content, "\n") {
			lineNo := i + 1
			if sqlStringRe.MatchString(line) && concatRe.MatchString(line) {
				out = append(out, Finding{
					Scanner: "injection", File: f.Path, Line: lineNo, Rule: "sql-concat",
					Message: fmt.Sprintf("SQL statement built by string concatenation/interpolation in %s — use parameterized queries", f.Path),
				})
			}
			if execSinkRe.MatchString(line) && interpolatedArgRe.MatchString(line) {
				out = append(out, Finding{
					Scanner: "injection", File: f.Path, Line: lineNo, Rule: "exec-interpolation",
					Message: fmt.Sprintf("command exec fed interpolated input in %s — avoid passing untrusted data to a shell", f.Path),
				})
			}
		}
	}
	return out
}
