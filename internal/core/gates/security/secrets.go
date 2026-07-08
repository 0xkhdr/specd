package security

import (
	"math"
	"regexp"
	"strings"
)

// secretsScanner flags known-format credentials and high-entropy string
// literals in tracked files (R1). Format rules carry near-zero false-positive
// rates; the entropy rule is deliberately conservative (high threshold, long
// minimum) so the gate stays quiet on ordinary source.
type secretsScanner struct{}

func (secretsScanner) Name() string { return "secrets" }

// Format rules: prefix + length + charset. Kept intentionally strict.
var secretFormatRules = []struct {
	rule string
	re   *regexp.Regexp
}{
	{"aws-access-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"github-pat", regexp.MustCompile(`ghp_[0-9A-Za-z]{36}`)},
	{"pem-private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
}

// entropyCandidate matches long unbroken base64/base64url/hex tokens. '=' is
// excluded so a `key=value` assignment splits into key and value rather than
// merging into one non-base64 blob; trailing base64 padding is trimmed in
// entropySuspicious.
var entropyCandidate = regexp.MustCompile(`[A-Za-z0-9+/_-]{24,}`)

const (
	// base64MaxEntropy is log2(64); a high threshold keeps FP low.
	base64EntropyThreshold = 4.6
	hexEntropyThreshold    = 3.6
)

func (secretsScanner) Scan(files []TrackedFile) []Finding {
	var findings []Finding
	for _, file := range files {
		for lineIdx, line := range strings.Split(string(file.Content), "\n") {
			lineNo := lineIdx + 1
			for _, fr := range secretFormatRules {
				for _, m := range fr.re.FindAllString(line, -1) {
					findings = append(findings, Finding{
						Scanner:     "secrets",
						Rule:        fr.rule,
						File:        file.Path,
						Line:        lineNo,
						Fingerprint: fingerprint(fr.rule, file.Path, m),
						Excerpt:     redact(m),
					})
				}
			}
			for _, tok := range entropyCandidate.FindAllString(line, -1) {
				if isFormatMatched(line, tok) {
					continue // already reported by a format rule
				}
				if !entropySuspicious(tok) {
					continue
				}
				findings = append(findings, Finding{
					Scanner:     "secrets",
					Rule:        "high-entropy-string",
					File:        file.Path,
					Line:        lineNo,
					Fingerprint: fingerprint("high-entropy-string", file.Path, tok),
					Excerpt:     redact(tok),
				})
			}
		}
	}
	return findings
}

// isFormatMatched avoids double-reporting a token that a format rule already
// flagged (e.g. an AKIA key is also high-entropy).
func isFormatMatched(line, tok string) bool {
	for _, fr := range secretFormatRules {
		if loc := fr.re.FindString(line); loc != "" && strings.Contains(tok, loc) {
			return true
		}
	}
	return false
}

func entropySuspicious(tok string) bool {
	trimmed := strings.Trim(tok, "=")
	if len(trimmed) < 24 {
		return false
	}
	switch {
	case isHex(trimmed):
		return len(trimmed) >= 32 && shannon(trimmed) >= hexEntropyThreshold
	case isBase64ish(trimmed):
		return shannon(trimmed) >= base64EntropyThreshold
	default:
		return false
	}
}

// isBase64ish accepts standard and URL-safe base64 alphabets (letters, digits,
// +, /, -, _).
func isBase64ish(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '+' || r == '/' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// shannon returns the Shannon entropy (bits per symbol) of s.
func shannon(s string) float64 {
	if s == "" {
		return 0
	}
	counts := map[rune]float64{}
	for _, r := range s {
		counts[r]++
	}
	n := float64(len(s))
	var h float64
	for _, c := range counts {
		p := c / n
		h -= p * math.Log2(p)
	}
	return h
}
