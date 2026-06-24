package contextpkg

import "regexp"

// Private copies of the Markdown helpers the slicers need. The originals live in
// internal/core (ears.go, md.go, tasksparser.go) and stay untouched; an
// internal/mdutil leaf to share them is deferred to backlog. Keeping copies here
// preserves the contextpkg → spec (+ stdlib) only import boundary.

var (
	htmlCommentRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	reqHeaderNumRe = regexp.MustCompile(`(?i)^##\s+Requirement\s+(\d+)`)
	taskRE         = regexp.MustCompile(`^- \[( |x)\] (T\d+) — (.*)$`)
	waveRE         = regexp.MustCompile(`^## Wave (\d+)\s*$`)
)

// StripHTMLComments blanks out HTML comment spans while preserving byte offsets
// and newlines (so line numbers survive).
func StripHTMLComments(text string) string {
	return htmlCommentRe.ReplaceAllStringFunc(text, func(m string) string {
		result := make([]byte, len(m))
		for i, c := range []byte(m) {
			if c == '\n' {
				result[i] = '\n'
			} else {
				result[i] = ' '
			}
		}
		return string(result)
	})
}

// splitLines splits on '\n' without dropping trailing empty strings, preserving
// line numbers.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
