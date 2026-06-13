package core

import "regexp"

type EarsPattern string

const (
	EarsUnwanted        EarsPattern = "unwanted"
	EarsEventDriven     EarsPattern = "event-driven"
	EarsStateDriven     EarsPattern = "state-driven"
	EarsOptionalFeature EarsPattern = "optional-feature"
	EarsUbiquitous      EarsPattern = "ubiquitous"
)

var earsPatterns = []struct {
	name EarsPattern
	re   *regexp.Regexp
}{
	{EarsUnwanted, regexp.MustCompile(`(?i)^IF .+ THEN THE SYSTEM SHALL .+`)},
	{EarsEventDriven, regexp.MustCompile(`(?i)^WHEN .+ THE SYSTEM SHALL .+`)},
	{EarsStateDriven, regexp.MustCompile(`(?i)^WHILE .+ THE SYSTEM SHALL .+`)},
	{EarsOptionalFeature, regexp.MustCompile(`(?i)^WHERE .+ THE SYSTEM SHALL .+`)},
	{EarsUbiquitous, regexp.MustCompile(`(?i)^THE SYSTEM SHALL .+`)},
}

func MatchEars(line string) (EarsPattern, bool) {
	for _, p := range earsPatterns {
		if p.re.MatchString(line) {
			return p.name, true
		}
	}
	return "", false
}

type EarsIssue struct {
	Line    int
	Message string
}

var (
	reqHeaderRe  = regexp.MustCompile(`(?i)^##\s+Requirement\b`)
	userStoryRe  = regexp.MustCompile(`(?i)\*\*User story:\*\*`)
	criterionRe  = regexp.MustCompile(`^\s*\d+\.\s+(.*)$`)
	acceptanceRe = regexp.MustCompile(`(?i)\*\*Acceptance criteria:\*\*`)
)

type reqBlock struct {
	headerLine   int
	name         string
	hasUserStory bool
	criteria     int
}

func LintEars(text string) []EarsIssue {
	lines := splitLines(StripHTMLComments(text))
	var issues []EarsIssue
	var blocks []reqBlock
	var current *reqBlock
	inAcceptance := false

	for i, line := range lines {
		lineNo := i + 1

		if reqHeaderRe.MatchString(line) {
			b := reqBlock{
				headerLine: lineNo,
				name:       reqHeaderRe.ReplaceAllString(line, ""),
			}
			// trim leading ##
			if len(line) > 3 {
				b.name = trimPrefix(line, "## ")
			}
			blocks = append(blocks, b)
			current = &blocks[len(blocks)-1]
			inAcceptance = false
			continue
		}
		if current == nil {
			continue
		}
		if userStoryRe.MatchString(line) {
			current.hasUserStory = true
			continue
		}
		if acceptanceRe.MatchString(line) {
			inAcceptance = true
			continue
		}
		m := criterionRe.FindStringSubmatch(line)
		if m != nil && inAcceptance {
			current.criteria++
			if _, ok := MatchEars(m[1]); !ok {
				issues = append(issues, EarsIssue{Line: lineNo, Message: `criterion does not match any EARS pattern: "` + m[1] + `"`})
			}
		}
	}

	for _, b := range blocks {
		if !b.hasUserStory {
			issues = append(issues, EarsIssue{Line: b.headerLine, Message: `requirement "` + b.name + `" missing **User story:** line`})
		}
		if b.criteria == 0 {
			issues = append(issues, EarsIssue{Line: b.headerLine, Message: `requirement "` + b.name + `" has no acceptance criteria`})
		}
	}
	if len(blocks) == 0 {
		issues = append(issues, EarsIssue{Line: 1, Message: "no '## Requirement N' sections found"})
	}
	return issues
}

func splitLines(s string) []string {
	// Split without removing trailing empty strings (preserves line numbers).
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

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
