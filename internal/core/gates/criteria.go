package gates

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// criteriaArmed reports whether the per-acceptance-criterion evidence ratchet
// must run for this approval: either config armed it explicitly
// (criteria.required) or the production lifecycle profile requires current
// criterion evidence (spec 01 R7.2). Default profile with the switch off keeps
// the ratchet disabled (R7.1).
func criteriaArmed(ctx CheckCtx) bool {
	return ctx.CriteriaRequired || ctx.ProductionProfile
}

// CriterionID names a single acceptance criterion of a requirement, addressed
// as "<req>.<sub>" (e.g. "1.2" = the second criterion of requirement R1).
type CriterionID struct {
	Req int
	Sub int
}

// String renders the "<req>.<sub>" address used on the command line and in
// evidence records.
func (c CriterionID) String() string {
	return fmt.Sprintf("%d.%d", c.Req, c.Sub)
}

// reqBullet matches a requirement bullet, e.g. "- **R1** When …" or "- R12: …",
// capturing an optional explicit criterion suffix so "- R2.2: …" is read as the
// second criterion of R2 rather than as a second requirement R2. The leading
// list marker is stripped before matching.
var reqBullet = regexp.MustCompile(`^\*{0,2}R(\d+)(?:\.(\d+))?\b`)

// CriterionIDs enumerates the acceptance-criterion ids declared in an EARS
// requirements document, in document order. It reuses the same bullet-oriented
// reading as the EARS gate so there is a single source of truth for what counts
// as a requirement (spec 04 R2, design note "one parser, no second source").
//
// A requirement is a top-level bullet whose text begins with an "R<n>" id. Two
// authoring styles address correctly:
//
//   - implicit — acceptance criteria are the more-indented sub-bullets beneath
//     the requirement bullet, numbered 1..k in document order;
//   - explicit — criteria are labelled inline as "- R<r>.<n>: …", in which case
//     the label is authoritative and the id is taken from it verbatim.
//
// A requirement that declares criteria neither way is itself a single criterion
// "<r>.1", so the flat one-bullet-per-requirement style still yields addressable
// ids.
//
// Ids are deduplicated. This matters because the result drives a gate: two
// entries sharing an id would be satisfied by one evidence record, letting a
// spec pass the criteria ratchet with a criterion genuinely unattested.
func CriterionIDs(requirementsDoc string) []CriterionID {
	var ids []CriterionID
	seen := map[CriterionID]bool{}
	curReq := 0
	curIndent := 0
	subCount := 0
	labelled := false

	emit := func(c CriterionID) {
		if seen[c] {
			return
		}
		seen[c] = true
		ids = append(ids, c)
	}
	flush := func() {
		// A requirement that declared no criteria either way is one, "<r>.1".
		if curReq != 0 && subCount == 0 && !labelled {
			emit(CriterionID{Req: curReq, Sub: 1})
		}
	}
	open := func(req, indent int) {
		flush()
		curReq = req
		curIndent = indent
		subCount = 0
		labelled = false
	}

	for _, line := range strings.Split(requirementsDoc, "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		trimmed := strings.TrimSpace(line)
		if !isBullet(trimmed) {
			continue
		}
		content := strings.TrimSpace(strings.TrimLeft(trimmed, "-*+"))
		if m := reqBullet.FindStringSubmatch(content); m != nil {
			req, _ := strconv.Atoi(m[1])
			if m[2] == "" {
				open(req, indent)
				continue
			}
			// Explicitly labelled criterion. Consecutive labels under the same
			// requirement extend it rather than reopening it.
			if req != curReq {
				open(req, indent)
			}
			labelled = true
			sub, _ := strconv.Atoi(m[2])
			emit(CriterionID{Req: req, Sub: sub})
			continue
		}
		if curReq != 0 && indent > curIndent {
			subCount++
			emit(CriterionID{Req: curReq, Sub: subCount})
		}
	}
	flush()
	return ids
}

// HasCriterion reports whether id (e.g. "1.2") is a valid criterion address in
// the given requirements document.
func HasCriterion(requirementsDoc, id string) bool {
	for _, c := range CriterionIDs(requirementsDoc) {
		if c.String() == id {
			return true
		}
	}
	return false
}

func isBullet(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '-', '*', '+':
		return len(trimmed) > 1 && trimmed[1] == ' '
	}
	return false
}
