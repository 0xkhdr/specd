package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EarsPattern identifies which EARS (Easy Approach to Requirements Syntax)
// form an acceptance criterion matched.
type EarsPattern string

// The EarsPattern values name the EARS forms MatchEars recognizes, in
// match-precedence order.
const (
	EarsUnwanted        EarsPattern = "unwanted"
	EarsEventDriven     EarsPattern = "event-driven"
	EarsStateDriven     EarsPattern = "state-driven"
	EarsOptionalFeature EarsPattern = "optional-feature"
	EarsUbiquitous      EarsPattern = "ubiquitous"
)

// earsPatterns is the single source of truth for EARS forms: each row carries
// the pattern name, the human-readable template (what authors must write), and
// the regex the gate enforces. MatchEars enforces; EarsForms publishes the
// templates for the authoring brief — both read this one table so the brief can
// never drift from what the gate accepts.
var earsPatterns = []struct {
	name EarsPattern
	form string
	re   *regexp.Regexp
}{
	{EarsUnwanted, "IF <condition> THEN THE SYSTEM SHALL <response>", regexp.MustCompile(`(?i)^IF .+ THEN THE SYSTEM SHALL .+`)},
	{EarsEventDriven, "WHEN <trigger> THE SYSTEM SHALL <response>", regexp.MustCompile(`(?i)^WHEN .+ THE SYSTEM SHALL .+`)},
	{EarsStateDriven, "WHILE <state> THE SYSTEM SHALL <response>", regexp.MustCompile(`(?i)^WHILE .+ THE SYSTEM SHALL .+`)},
	{EarsOptionalFeature, "WHERE <feature> THE SYSTEM SHALL <response>", regexp.MustCompile(`(?i)^WHERE .+ THE SYSTEM SHALL .+`)},
	{EarsUbiquitous, "THE SYSTEM SHALL <response>", regexp.MustCompile(`(?i)^THE SYSTEM SHALL .+`)},
}

// EarsForms returns the canonical EARS templates in match-precedence order,
// derived from the same table MatchEars enforces.
func EarsForms() []string {
	out := make([]string, len(earsPatterns))
	for i, p := range earsPatterns {
		out[i] = p.form
	}
	return out
}

// MatchEars tests line against each known EARS pattern in precedence order
// and returns the first one that matches, or false if none do.
func MatchEars(line string) (EarsPattern, bool) {
	for _, p := range earsPatterns {
		if p.re.MatchString(line) {
			return p.name, true
		}
	}
	return "", false
}

// EarsIssue is a single requirements.md lint finding: the 1-based source
// line and a human-readable message describing the problem.
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

// LintEars scans requirements.md text for "## Requirement N" blocks and
// flags each one missing a **User story:** line, missing acceptance criteria,
// or containing a criterion that doesn't match any EARS pattern. It also
// flags the document as a whole when no requirement sections are found.
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

// reqHeaderNumRe captures the numeric id from a `## Requirement N` header so
// criteria can be addressed as "<req>.<idx>" — the same key space the verify
// `--criterion <req>.<n>` route and State.Acceptance already use.
var reqHeaderNumRe = regexp.MustCompile(`(?i)^##\s+Requirement\s+(\d+)`)

// Criterion is a single acceptance criterion with a deterministic, mapping-stable
// ID. IDs are "<requirement>.<index>" where index is the 1-based position of the
// criterion within its requirement's acceptance list (matching the markdown
// ordinal in well-formed requirements.md).
type Criterion struct {
	ID      string      // "<req>.<idx>", e.g. "1.2"
	Req     int         // requirement number
	Index   int         // 1-based position within the requirement's acceptance list
	Text    string      // criterion prose
	Line    int         // 1-based line in requirements.md
	Pattern EarsPattern // matched EARS pattern ("" when none matched)
	EarsOK  bool        // whether Text matched an EARS pattern
}

// ExtractCriteria walks requirements.md and returns every acceptance criterion
// with a stable ID. It is read-only and side-effect free; the acceptance gate
// being off simply means callers never invoke it, so EARS linting and existing
// numbering are unaffected (T2: no change when the gate is off).
func ExtractCriteria(text string) []Criterion {
	lines := splitLines(StripHTMLComments(text))
	var out []Criterion
	curReq := 0
	idx := 0
	inAcceptance := false
	for i, line := range lines {
		lineNo := i + 1
		if m := reqHeaderNumRe.FindStringSubmatch(line); m != nil {
			curReq, _ = strconv.Atoi(m[1])
			idx = 0
			inAcceptance = false
			continue
		}
		// A Requirement header without a parseable number still resets context
		// so criteria under it are not misattributed to the previous requirement.
		if reqHeaderRe.MatchString(line) {
			curReq = 0
			idx = 0
			inAcceptance = false
			continue
		}
		if curReq == 0 {
			continue
		}
		if acceptanceRe.MatchString(line) {
			inAcceptance = true
			continue
		}
		if m := criterionRe.FindStringSubmatch(line); m != nil && inAcceptance {
			idx++
			crit := strings.TrimSpace(m[1])
			pat, ok := MatchEars(crit)
			out = append(out, Criterion{
				ID:      fmt.Sprintf("%d.%d", curReq, idx),
				Req:     curReq,
				Index:   idx,
				Text:    crit,
				Line:    lineNo,
				Pattern: pat,
				EarsOK:  ok,
			})
		}
	}
	return out
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
