package core

import (
	"fmt"
	"regexp"
	"strings"
)

// Requirement is a parsed `R<n>` group from requirements.md — a stable ID, a
// human title, its acceptance criteria, and optional author metadata. The
// parser is pure and byte-stable: identical input bytes always yield identical
// records (R1.3) and the author's source bytes are never rewritten.
type Requirement struct {
	ID       string
	Title    string
	Criteria []Criterion
	Owner    string
	Priority string
	Risk     string
	Edges    []string // edge/failure behaviors the author declared
}

// Criterion is a parsed `R<n>.<m>` acceptance clause with its EARS shape split
// into trigger ("When …") and response ("… shall …") when present.
type Criterion struct {
	ID       string
	Clause   string // the full normalized clause text after the ID
	Trigger  string // the "When <trigger>," portion, empty if not EARS-shaped
	Response string // the "shall <response>" portion, empty if not EARS-shaped
}

// RequirementsDoc is the normalized view of a requirements.md file. Raw holds a
// defensive copy of the author's bytes (never mutated by the parser).
type RequirementsDoc struct {
	Raw          []byte
	Requirements []Requirement
	Exclusions   []string // "## Non-goals" bullets — document-level exclusions
}

var (
	reReqHeading  = regexp.MustCompile(`^#{2,4}\s+(R\d+)\b(.*)$`)
	reCriterion   = regexp.MustCompile(`^[-*]\s+(R\d+\.\d+)\s*:\s*(.*)$`)
	reMetaBullet  = regexp.MustCompile(`^[-*]\s+(owner|priority|risk|edge|failure)\s*:\s*(.*)$`)
	rePlainBullet = regexp.MustCompile(`^[-*]\s+(.*)$`)
)

// ParseRequirements parses requirements.md bytes into a normalized doc. It never
// rewrites the author's bytes; semantic problems (missing/duplicate/malformed
// IDs) are surfaced by ValidateRequirements, not here. The returned error is
// reserved for input the parser cannot read as text (currently none).
func ParseRequirements(raw []byte) (RequirementsDoc, error) {
	doc := RequirementsDoc{Raw: append([]byte(nil), raw...)}
	var cur *Requirement
	inExclusions := false

	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			// A new section heading ends the current requirement and toggles the
			// exclusions ("Non-goals") region.
			if m := reReqHeading.FindStringSubmatch(trimmed); m != nil {
				doc.Requirements = append(doc.Requirements, Requirement{
					ID:    m[1],
					Title: requirementTitle(m[2]),
				})
				cur = &doc.Requirements[len(doc.Requirements)-1]
				inExclusions = false
				continue
			}
			cur = nil
			inExclusions = strings.Contains(strings.ToLower(trimmed), "non-goal")
			continue
		}

		if inExclusions {
			if m := rePlainBullet.FindStringSubmatch(trimmed); m != nil {
				doc.Exclusions = append(doc.Exclusions, strings.TrimSpace(m[1]))
			}
			continue
		}

		if cur == nil {
			continue
		}

		if m := reCriterion.FindStringSubmatch(trimmed); m != nil {
			clause := strings.TrimSpace(m[2])
			trigger, response := parseEARS(clause)
			cur.Criteria = append(cur.Criteria, Criterion{
				ID:       m[1],
				Clause:   clause,
				Trigger:  trigger,
				Response: response,
			})
			continue
		}
		if m := reMetaBullet.FindStringSubmatch(trimmed); m != nil {
			value := strings.TrimSpace(m[2])
			switch m[1] {
			case "owner":
				cur.Owner = value
			case "priority":
				cur.Priority = value
			case "risk":
				cur.Risk = value
			case "edge", "failure":
				cur.Edges = append(cur.Edges, value)
			}
		}
	}
	return doc, nil
}

// ReqFinding is an addressable requirements defect. ID names the offending
// requirement or criterion (empty for a document-level finding); Message states
// exactly what is wrong so the author can fix it without guessing (R1.2).
type ReqFinding struct {
	ID      string
	Message string
}

// ValidateRequirements reports missing, duplicate, malformed, or conflicting
// requirement IDs, criterion IDs, and EARS clauses in a structured
// requirements doc. It is pure: no disk, no clock. Callers with an unstructured
// doc (len(Requirements) == 0 and content present) get a single "no
// requirements found" finding, which the EARS gate suppresses for docs that use
// the plain bullet shape.
func ValidateRequirements(doc RequirementsDoc) []ReqFinding {
	var findings []ReqFinding
	if len(doc.Requirements) == 0 {
		return []ReqFinding{{Message: "no structured requirements found"}}
	}
	seenReq := map[string]bool{}
	seenCrit := map[string]bool{}
	for _, r := range doc.Requirements {
		if seenReq[r.ID] {
			findings = append(findings, ReqFinding{ID: r.ID, Message: "duplicate requirement id"})
		}
		seenReq[r.ID] = true
		if strings.TrimSpace(r.Title) == "" {
			findings = append(findings, ReqFinding{ID: r.ID, Message: "requirement title is required"})
		}
		for _, c := range r.Criteria {
			if !strings.HasPrefix(c.ID, r.ID+".") {
				findings = append(findings, ReqFinding{ID: c.ID, Message: fmt.Sprintf("criterion does not belong to requirement %s", r.ID)})
			}
			if seenCrit[c.ID] {
				findings = append(findings, ReqFinding{ID: c.ID, Message: "duplicate criterion id"})
			}
			seenCrit[c.ID] = true
			if c.Trigger == "" || c.Response == "" {
				findings = append(findings, ReqFinding{ID: c.ID, Message: "criterion is not EARS-shaped (When …, the system shall …)"})
			}
		}
	}
	return findings
}

// requirementTitle extracts the human title from the text after `### R<n>`,
// dropping a leading em-dash or hyphen separator.
func requirementTitle(rest string) string {
	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, "—")
	rest = strings.TrimPrefix(rest, "-")
	return strings.TrimSpace(rest)
}

// parseEARS splits an EARS-shaped clause into its trigger and response. A clause
// without "shall" is not EARS-shaped and yields two empty strings (the caller
// treats that as malformed). The trailing ", system"/", the system" marker
// before "shall" is stripped from the trigger.
func parseEARS(clause string) (trigger, response string) {
	idx := strings.Index(clause, "shall")
	if idx < 0 {
		return "", ""
	}
	left := strings.TrimSpace(clause[:idx])
	response = strings.TrimSpace(clause[idx+len("shall"):])
	for _, suffix := range []string{", the system", ", system"} {
		if strings.HasSuffix(left, suffix) {
			left = strings.TrimSuffix(left, suffix)
			break
		}
	}
	left = strings.TrimSpace(left)
	trigger = strings.TrimSpace(strings.TrimPrefix(left, "When"))
	return trigger, response
}
