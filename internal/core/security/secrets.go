package security

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// secrets.go detects credentials committed to changed files: known-format
// patterns (cloud keys, private-key PEM blocks, JWTs) plus a Shannon-entropy
// heuristic for high-randomness tokens assigned to secret-looking identifiers.
// All matching is deterministic and line-scoped. Findings for allowlisted values
// (with a mandatory reason) are suppressed.

// knownSecretPattern is one named, compiled credential format. The regex sources
// are written so they never match themselves (e.g. the AWS rule matches AKIA plus
// 16 chars, and this file contains no such 20-char literal), so the suite reports
// zero findings on its own source.
type knownSecretPattern struct {
	rule string
	re   *regexp.Regexp
}

var knownSecretPatterns = []knownSecretPattern{
	{"aws-access-key-id", regexp.MustCompile(`\b(?:AKIA|ASIA|AGPA|AIDA|AROA|ANPA)[0-9A-Z]{16}\b`)},
	{"google-api-key", regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`)},
	{"slack-token", regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`)},
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[0-9A-Za-z]{36}\b`)},
	{"private-key-pem", regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
}

// secretAssignmentRe captures a value assigned to a secret-looking key
// (password, token, secret, apikey, ...). The captured value feeds the entropy
// check so we only entropy-scan plausibly-secret assignments — this keeps the
// false-positive rate down (plan risk 2).
var secretAssignmentRe = regexp.MustCompile(`(?i)\b(pass(?:word|wd)?|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret|auth)\b\s*[:=]\s*["']?([^"'\s]{16,})["']?`)

// entropyThreshold is the Shannon-entropy (bits/char) above which an assigned
// value is treated as a likely credential. 4.0 clears typical english/config text
// while catching random base64/hex tokens.
const entropyThreshold = 4.0

// ScanSecrets returns secret findings across the changed files. Pure over the
// supplied contents; allowlisted values (matched by exact substring, reason
// mandatory) are suppressed.
func ScanSecrets(files []ChangedFile, allow Allowlist) []Finding {
	var out []Finding
	for _, f := range files {
		for i, line := range strings.Split(f.Content, "\n") {
			lineNo := i + 1
			for _, p := range knownSecretPatterns {
				if m := p.re.FindString(line); m != "" && !allow.Allows(m) {
					out = append(out, Finding{
						Scanner: "secrets", File: f.Path, Line: lineNo, Rule: p.rule,
						Message: fmt.Sprintf("possible %s committed to %s", p.rule, f.Path),
					})
				}
			}
			if m := secretAssignmentRe.FindStringSubmatch(line); m != nil {
				val := m[2]
				if !allow.Allows(val) && shannonEntropy(val) >= entropyThreshold && !looksLikePlaceholder(val) {
					out = append(out, Finding{
						Scanner: "secrets", File: f.Path, Line: lineNo, Rule: "high-entropy-assignment",
						Message: fmt.Sprintf("high-entropy value assigned to a secret-looking key in %s (entropy %.2f)", f.Path, shannonEntropy(val)),
					})
				}
			}
		}
	}
	return out
}

// looksLikePlaceholder suppresses obvious non-secret values (env-var
// interpolation, templated placeholders, repeated x/asterisk redactions) that
// would otherwise trip the entropy check.
func looksLikePlaceholder(v string) bool {
	if strings.Contains(v, "${") || strings.Contains(v, "{{") || strings.HasPrefix(v, "$(") {
		return true
	}
	lower := strings.ToLower(v)
	for _, p := range []string{"example", "changeme", "placeholder", "your-", "xxxxxxxx", "redacted", "dummy", "sample"} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// A value that is a single repeated character (****, xxxx) is a redaction.
	if isRepeatedChar(v) {
		return true
	}
	return false
}

func isRepeatedChar(v string) bool {
	if v == "" {
		return false
	}
	for i := 1; i < len(v); i++ {
		if v[i] != v[0] {
			return false
		}
	}
	return true
}

// shannonEntropy returns the per-character Shannon entropy (bits) of s.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	counts := map[rune]float64{}
	for _, r := range s {
		counts[r]++
	}
	n := float64(len([]rune(s)))
	var h float64
	for _, c := range counts {
		p := c / n
		h -= p * math.Log2(p)
	}
	return h
}
