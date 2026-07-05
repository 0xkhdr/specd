package security

import (
	_ "embed"
	"strings"
)

// slopsquatScanner parses dependency manifests and flags module names within a
// small edit distance of a popular package — the typosquat / "slopsquat" attack
// where an LLM or a typo pulls golang.org/x/tolls instead of .../tools (R4).
type slopsquatScanner struct{}

func (slopsquatScanner) Name() string { return "slopsquat" }

//go:embed popular_go_packages.txt
var popularPackagesRaw string

// popularPackages is the parsed, deduped reference list.
var popularPackages = parsePopularList(popularPackagesRaw)

func parsePopularList(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func (slopsquatScanner) Scan(files []TrackedFile) []Finding {
	var findings []Finding
	for _, file := range files {
		if !strings.HasSuffix(file.Path, "go.mod") {
			continue // extensible: package.json, requirements.txt (design note R4)
		}
		for _, dep := range parseGoMod(string(file.Content)) {
			for _, pop := range popularPackages {
				if dep.path == pop {
					break // exact match is the real package, never a finding
				}
				if withinTypoDistance(dep.path, pop) {
					findings = append(findings, Finding{
						Scanner:     "slopsquat",
						Rule:        "typosquat",
						File:        file.Path,
						Line:        dep.line,
						Fingerprint: fingerprint("typosquat", file.Path, dep.path),
						Excerpt:     dep.path + " ~ " + pop,
					})
					break // report against the nearest popular name once
				}
			}
		}
	}
	return findings
}

type goModDep struct {
	path string
	line int
}

// parseGoMod extracts required module paths and their 1-based line numbers,
// handling both single-line `require x v1` and block `require (\n x v1 \n)`.
func parseGoMod(content string) []goModDep {
	var deps []goModDep
	inBlock := false
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		lineNo := i + 1
		switch {
		case line == "require (":
			inBlock = true
		case inBlock && line == ")":
			inBlock = false
		case inBlock:
			if p := firstField(line); p != "" && !strings.HasPrefix(line, "//") {
				deps = append(deps, goModDep{path: p, line: lineNo})
			}
		case strings.HasPrefix(line, "require "):
			if p := firstField(strings.TrimPrefix(line, "require ")); p != "" {
				deps = append(deps, goModDep{path: p, line: lineNo})
			}
		}
	}
	return deps
}

func firstField(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// withinTypoDistance reports whether candidate is a plausible typosquat of pop:
// Damerau-Levenshtein ≤ 1 for short names, ≤ 2 for names of length ≥ 8. Distance
// 0 (handled by the caller) is excluded.
func withinTypoDistance(candidate, pop string) bool {
	d := damerauLevenshtein(candidate, pop)
	if d == 0 {
		return false
	}
	threshold := 1
	if len(pop) >= 8 {
		threshold = 2
	}
	return d <= threshold
}

// damerauLevenshtein returns the optimal-string-alignment distance (adjacent
// transposition costs 1).
func damerauLevenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min3(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				if t := d[i-2][j-2] + 1; t < d[i][j] {
					d[i][j] = t
				}
			}
		}
	}
	return d[la][lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
