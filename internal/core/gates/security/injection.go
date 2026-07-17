package security

import (
	"regexp"
	"strings"
)

// injectionScanner flags prompt-injection heuristics in tracked text/markdown
// (R3).
type injectionScanner struct{}

func (injectionScanner) Name() string { return "injection" }

func (injectionScanner) Exclude(input ScanInputV1) bool {
	return excludedScannerPath(input.Path, "testdata", "vendor", ".git", ".specd/security")
}

// Scannable text extensions. Injection payloads hide in prose, not binaries.
var textExtensions = map[string]struct{}{
	".md": {}, ".markdown": {}, ".txt": {}, ".rst": {},
	".adoc": {}, ".mdx": {}, ".text": {},
}

var injectionPhraseRules = []struct {
	rule string
	re   *regexp.Regexp
}{
	{"override-instructions", regexp.MustCompile(`(?i)ignore (all |the )?(previous|prior|above) (instructions|prompts?)`)},
	{"override-instructions", regexp.MustCompile(`(?i)disregard (all |the )?(previous|prior|above|earlier)`)},
	{"role-override", regexp.MustCompile(`(?i)you are now (a |an )?[a-z]`)},
	{"hidden-instruction", regexp.MustCompile(`(?i)<!--\s*(system|assistant|instruction)`)},
	{"tool-exfil", regexp.MustCompile(`(?i)(exfiltrate|send).{0,20}(secret|token|credential|api[ _-]?key)`)},
}

// Zero-width and directional-control code points used to smuggle hidden text:
// ZWSP, ZWNJ, ZWJ, word-joiner, BOM/ZWNBSP, and the LRE..RLO bidi overrides.
// Declared as integers so the source file itself stays free of the very
// characters this rule hunts (which also keeps the file valid UTF-8/ASCII).
var zeroWidthRunes = []rune{
	0x200B, 0x200C, 0x200D, 0x2060, 0xFEFF,
	0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
}

func (s injectionScanner) Scan(files []ScanInputV1) []Finding {
	var findings []Finding
	for _, file := range files {
		if s.Exclude(file) || !isTextFile(file.Path) {
			continue
		}
		for lineIdx, line := range strings.Split(string(file.Content), "\n") {
			lineNo := lineIdx + 1
			for _, pr := range injectionPhraseRules {
				if m := pr.re.FindString(line); m != "" {
					findings = append(findings, Finding{
						Scanner:     "injection",
						Rule:        pr.rule,
						File:        file.Path,
						Line:        lineNo,
						Fingerprint: fingerprint(pr.rule, file.Path, strings.ToLower(strings.TrimSpace(m))),
						Excerpt:     injectionExcerpt(pr.rule),
					})
				}
			}
			if hasZeroWidth(line) {
				findings = append(findings, Finding{
					Scanner:     "injection",
					Rule:        "zero-width-smuggling",
					File:        file.Path,
					Line:        lineNo,
					Fingerprint: fingerprint("zero-width-smuggling", file.Path, line),
					Excerpt:     "zero-width/bidi control character(s)",
				})
			}
		}
	}
	return findings
}

func injectionExcerpt(rule string) string {
	switch rule {
	case "override-instructions":
		return "prompt override marker"
	case "role-override":
		return "role override marker"
	case "hidden-instruction":
		return "hidden instruction marker"
	case "tool-exfil":
		return "tool exfiltration marker"
	default:
		return "injection marker"
	}
}

func isTextFile(path string) bool {
	dot := strings.LastIndex(path, ".")
	if dot < 0 {
		return false
	}
	_, ok := textExtensions[strings.ToLower(path[dot:])]
	return ok
}

func hasZeroWidth(line string) bool {
	for _, zw := range zeroWidthRunes {
		if strings.ContainsRune(line, zw) {
			return true
		}
	}
	return false
}
