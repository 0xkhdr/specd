package core

import (
	"regexp"
	"strings"
)

// DesignDoc is the normalized view of a design.md decision contract (spec 01
// R2). It captures the metadata a human design approval must pin: the
// requirement references that trace the design to approved requirements, the
// module boundaries and interfaces it commits to, the invariants it preserves,
// its failure and integration modes, the alternatives weighed, the chosen
// disposition, and the human owner accountable for the choice. The parser is
// pure and byte-stable: Raw is a defensive copy the parser never rewrites.
type DesignDoc struct {
	Raw    []byte
	Refs   []string          // requirement ids (R<n>/R<n>.<m>) the design traces to
	Fields map[string]string // decision-metadata label -> author value
}

// designFields are the decision-metadata labels a production design contract
// must declare with a non-empty value (spec 01 R2.1).
var designFields = []string{
	"boundaries",
	"interfaces",
	"invariants",
	"failure",
	"integration",
	"alternatives",
	"disposition",
	"owner",
}

var (
	reDesignRefs  = regexp.MustCompile(`(?i)^[-*]\s+(?:refs?|references)\s*:\s*(.*)$`)
	reDesignField = regexp.MustCompile(`(?i)^[-*]\s+([a-z-]+)\s*:\s*(.*)$`)
	reReqRefToken = regexp.MustCompile(`\bR\d+(?:\.\d+)?\b`)
)

// ParseDesign parses design.md bytes into a normalized decision contract. It
// never rewrites the author's bytes; completeness is judged by ValidateDesign,
// not here. Labels match case-insensitively. Requirement references are read
// only from an explicit `references:` bullet, so design.md prose that
// merely contains an `R<n>` token is never misread as a trace (keeps default
// design.md files without them valid).
func ParseDesign(raw []byte) DesignDoc {
	doc := DesignDoc{Raw: append([]byte(nil), raw...), Fields: map[string]string{}}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if m := reDesignRefs.FindStringSubmatch(trimmed); m != nil {
			doc.Refs = append(doc.Refs, reReqRefToken.FindAllString(m[1], -1)...)
			continue
		}
		if m := reDesignField.FindStringSubmatch(trimmed); m != nil {
			label := strings.ToLower(m[1])
			if _, seen := doc.Fields[label]; !seen {
				if value := strings.TrimSpace(m[2]); value != "" {
					doc.Fields[label] = value
				}
			}
		}
	}
	return doc
}

// Digest returns the content address of the parsed source bytes — the digest an
// approval record pins so a later amendment can detect design drift (spec 01
// R2.1 "and digest").
func (d DesignDoc) Digest() string { return Digest(d.Raw) }

// DesignFinding is an addressable design-contract defect (spec 01 R2.2). Ref
// names the offending requirement reference, empty for a document-level defect.
type DesignFinding struct {
	Ref     string
	Message string
}

// ValidateDesign reports design-contract defects. An unknown requirement
// reference is always refused: a design tracing to a requirement that does not
// exist is a real defect (spec 01 R2.2). When requireContract is set — the
// production design profile (spec 01 R7.2) — the design must additionally
// declare every decision-metadata field and at least one resolvable requirement
// reference; under the default profile the contract fields are optional so
// minimal design.md files keep approving (R7.1). Pure: no disk, no clock.
func ValidateDesign(doc DesignDoc, knownReqIDs map[string]bool, requireContract bool) []DesignFinding {
	var findings []DesignFinding
	for _, ref := range doc.Refs {
		if !knownReqIDs[ref] && !knownReqIDs[requirementOf(ref)] {
			findings = append(findings, DesignFinding{Ref: ref, Message: "design references unknown requirement " + ref})
		}
	}
	if !requireContract {
		return findings
	}
	if len(doc.Refs) == 0 {
		findings = append(findings, DesignFinding{Message: "design declares no requirement references"})
	}
	for _, field := range designFields {
		if strings.TrimSpace(doc.Fields[field]) == "" {
			findings = append(findings, DesignFinding{Message: "design contract field " + field + " is required"})
		}
	}
	return findings
}

// RequirementIDSet parses requirements.md bytes and returns the set of every
// requirement id and criterion id, for resolving design references. An empty or
// unparseable doc yields an empty set (the design gate then flags any declared
// reference as unresolved — fail closed).
func RequirementIDSet(raw string) map[string]bool {
	set := map[string]bool{}
	doc, _ := ParseRequirements([]byte(raw))
	for _, r := range doc.Requirements {
		set[r.ID] = true
		for _, c := range r.Criteria {
			set[c.ID] = true
		}
	}
	return set
}

// requirementOf reduces a criterion id (R<n>.<m>) to its parent requirement id
// (R<n>); a bare requirement id is returned unchanged.
func requirementOf(ref string) string {
	if i := strings.IndexByte(ref, '.'); i >= 0 {
		return ref[:i]
	}
	return ref
}
