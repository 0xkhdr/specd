package security

import (
	"fmt"
	"regexp"
	"strings"
)

// slopsquat.go flags dependency names that are a tiny edit away from a popular
// package — the "slopsquatting" pattern where an agent hallucinates a plausible
// but wrong (and possibly malicious) package name. Manifests are parsed with the
// stdlib only (no ecosystem tooling); each declared name is edit-distance-checked
// against a shipped, static popular-package list (embedded at build, refreshed per
// release — no network fetch, invariant 2/3). An exact match is fine; a near-miss
// (distance 1–2, same first letter) is the finding.

// popularPackages is the shipped near-miss corpus, keyed by ecosystem. It is
// deliberately small and high-signal — the most-typosquatted names — so the check
// stays fast and low-noise. Refreshed per release.
var popularPackages = map[string][]string{
	"npm": {
		"react", "react-dom", "lodash", "express", "axios", "chalk", "commander",
		"request", "webpack", "babel", "eslint", "typescript", "moment", "jsonwebtoken",
		"mongoose", "dotenv", "cross-env", "colors", "debug", "yargs",
	},
	"pypi": {
		"requests", "numpy", "pandas", "flask", "django", "urllib3", "setuptools",
		"pytest", "boto3", "scipy", "pillow", "sqlalchemy", "click", "jinja2",
		"cryptography", "pyyaml", "beautifulsoup4", "matplotlib", "tensorflow",
	},
	"go": {
		"github.com/stretchr/testify", "github.com/gorilla/mux", "github.com/gin-gonic/gin",
		"github.com/spf13/cobra", "github.com/sirupsen/logrus", "google.golang.org/grpc",
	},
}

var (
	npmDepRe    = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*"[^"]*"\s*,?\s*$`)
	pyReqRe     = regexp.MustCompile(`^\s*([A-Za-z0-9._-]+)\s*(?:[=<>!~]=|$)`)
	goRequireRe = regexp.MustCompile(`^\s*([a-z0-9.\-/]+)\s+v[0-9]`)
)

// ScanSlopsquat parses each recognized manifest in files and reports declared
// dependency names that are a near-miss of a popular package. Pure over the
// supplied contents.
func ScanSlopsquat(files []ChangedFile) []Finding {
	var out []Finding
	for _, f := range files {
		base := f.Path
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		switch {
		case base == "package.json":
			out = append(out, scanNames(f, parsePackageJSON(f.Content), "npm")...)
		case base == "requirements.txt":
			out = append(out, scanNames(f, parseRequirements(f.Content), "pypi")...)
		case base == "go.mod":
			out = append(out, scanNames(f, parseGoMod(f.Content), "go")...)
		}
	}
	return out
}

// named is a declared dependency and the line it appeared on.
type named struct {
	name string
	line int
}

func scanNames(f ChangedFile, names []named, ecosystem string) []Finding {
	var out []Finding
	popular := popularPackages[ecosystem]
	for _, n := range names {
		if near, dist := nearestPopular(n.name, popular); near != "" && dist > 0 {
			out = append(out, Finding{
				Scanner: "slopsquat", File: f.Path, Line: n.line, Rule: "typosquat",
				Message: fmt.Sprintf("dependency %q is edit-distance %d from popular %s package %q — verify it is intentional (possible hallucinated/typosquatted package)", n.name, dist, ecosystem, near),
			})
		}
	}
	return out
}

// nearestPopular returns the closest popular name within the allowed edit
// distance, and the distance. Distance 0 (exact match) returns the name with
// dist 0 so the caller can distinguish "known-good" from "near-miss". A name
// farther than the threshold returns ("", 0).
func nearestPopular(name string, popular []string) (string, int) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", 0
	}
	threshold := 1
	if len(name) >= 8 {
		threshold = 2
	}
	best, bestDist := "", threshold+1
	for _, p := range popular {
		pl := strings.ToLower(p)
		if pl == name {
			return p, 0 // exact — not a finding
		}
		// Only compare names sharing a first character to avoid absurd matches and
		// keep the check O(1)-ish per candidate.
		if pl[0] != name[0] {
			continue
		}
		d := levenshtein(name, pl)
		if d < bestDist {
			best, bestDist = p, d
		}
	}
	if best == "" || bestDist > threshold {
		return "", 0
	}
	return best, bestDist
}

// levenshtein is the classic edit distance (stdlib-only, two-row DP).
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
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

// parsePackageJSON extracts dependency names from the dependencies /
// devDependencies objects of a package.json, without a JSON parse (so a partial
// or comment-bearing file still yields names). Line-scoped.
func parsePackageJSON(content string) []named {
	var out []named
	inDeps := false
	depthAtDeps := 0
	depth := 0
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if strings.Contains(trimmed, "\"dependencies\"") || strings.Contains(trimmed, "\"devDependencies\"") || strings.Contains(trimmed, "\"optionalDependencies\"") || strings.Contains(trimmed, "\"peerDependencies\"") {
			inDeps = true
			depthAtDeps = depth
			continue
		}
		if inDeps && depth < depthAtDeps {
			inDeps = false
		}
		if inDeps {
			if m := npmDepRe.FindStringSubmatch(line); m != nil {
				out = append(out, named{name: m[1], line: i + 1})
			}
		}
	}
	return out
}

// parseRequirements extracts package names from a requirements.txt, ignoring
// comments, blank lines, options (-r, --hash), and env markers.
func parseRequirements(content string) []named {
	var out []named
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		if m := pyReqRe.FindStringSubmatch(trimmed); m != nil {
			out = append(out, named{name: m[1], line: i + 1})
		}
	}
	return out
}

// parseGoMod extracts module paths from require directives (block and single).
func parseGoMod(content string) []named {
	var out []named
	inBlock := false
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "require ("):
			inBlock = true
			continue
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		case strings.HasPrefix(trimmed, "require "):
			trimmed = strings.TrimPrefix(trimmed, "require ")
		case !inBlock:
			continue
		}
		if m := goRequireRe.FindStringSubmatch(trimmed); m != nil {
			out = append(out, named{name: m[1], line: i + 1})
		}
	}
	return out
}
