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

// reqBullet matches a requirement header bullet, e.g. "- **R1** When …" or
// "- R12: …". The leading list marker is stripped before matching.
var reqBullet = regexp.MustCompile(`^\*{0,2}R(\d+)\b`)

// CriterionIDs enumerates the acceptance-criterion ids declared in an EARS
// requirements document, in document order. It reuses the same bullet-oriented
// reading as the EARS gate so there is a single source of truth for what counts
// as a requirement (spec 04 R2, design note "one parser, no second source").
//
// A requirement is a top-level bullet whose text begins with an "R<n>" id. Its
// acceptance criteria are the more-indented sub-bullets beneath it, numbered
// 1..k in order. A requirement with no sub-bullets is itself a single criterion
// "<r>.1", so the flat one-bullet-per-requirement style still yields addressable
// ids.
func CriterionIDs(requirementsDoc string) []CriterionID {
	var ids []CriterionID
	curReq := 0
	curIndent := 0
	subCount := 0

	flush := func() {
		// A requirement that declared no sub-bullets is one criterion, "<r>.1".
		if curReq != 0 && subCount == 0 {
			ids = append(ids, CriterionID{Req: curReq, Sub: 1})
		}
	}

	for _, line := range strings.Split(requirementsDoc, "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		trimmed := strings.TrimSpace(line)
		if !isBullet(trimmed) {
			continue
		}
		content := strings.TrimSpace(strings.TrimLeft(trimmed, "-*+"))
		if m := reqBullet.FindStringSubmatch(content); m != nil {
			flush()
			curReq, _ = strconv.Atoi(m[1])
			curIndent = indent
			subCount = 0
			continue
		}
		if curReq != 0 && indent > curIndent {
			subCount++
			ids = append(ids, CriterionID{Req: curReq, Sub: subCount})
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
