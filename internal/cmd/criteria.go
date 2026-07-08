package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

// requirementCoverage is one requirement's criterion tally: how many of its
// acceptance criteria currently hold a passing record.
type requirementCoverage struct {
	Req     int `json:"req"`
	Total   int `json:"total"`
	Passing int `json:"passing"`
}

// criterionCoverage derives per-requirement acceptance-criterion coverage:
// current passing records (recorded after the last requirements approval) over
// total declared criteria (spec 04 R5). It returns nil when the spec declares
// no criteria so callers can skip the section entirely.
func criterionCoverage(root, slug string) ([]requirementCoverage, error) {
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	reqDoc, err := os.ReadFile(filepath.Join(dir, "requirements.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	ids := gates.CriterionIDs(string(reqDoc))
	if len(ids) == 0 {
		return nil, nil
	}
	records, err := core.LoadCriteria(core.CriteriaPath(root, slug))
	if err != nil {
		return nil, err
	}
	passing := core.CurrentPassing(records, requirementsApprovedAt(root, slug))

	byReq := map[int]*requirementCoverage{}
	var order []int
	for _, id := range ids {
		cov, ok := byReq[id.Req]
		if !ok {
			cov = &requirementCoverage{Req: id.Req}
			byReq[id.Req] = cov
			order = append(order, id.Req)
		}
		cov.Total++
		if passing[id.String()] {
			cov.Passing++
		}
	}
	sort.Ints(order)
	out := make([]requirementCoverage, 0, len(order))
	for _, r := range order {
		out = append(out, *byReq[r])
	}
	return out, nil
}

// renderCriterionCoverage formats coverage for the human status/report output.
// Returns "" when there is nothing to show.
func renderCriterionCoverage(cov []requirementCoverage) string {
	if len(cov) == 0 {
		return ""
	}
	var b strings.Builder
	total, passing := 0, 0
	b.WriteString("\nAcceptance criteria coverage:\n")
	for _, c := range cov {
		fmt.Fprintf(&b, "  R%d  %d/%d\n", c.Req, c.Passing, c.Total)
		total += c.Total
		passing += c.Passing
	}
	fmt.Fprintf(&b, "  total %d/%d criteria passing\n", passing, total)
	return b.String()
}

// loadSpecConfig loads project config, discarding diagnostics — callers only
// need the resolved values (invalid config surfaces elsewhere via `check`).
func loadSpecConfig(root string) core.Config {
	cfg, _ := core.LoadConfig(core.ConfigPaths{Project: filepath.Join(root, "project.yml")}, getenv())
	return cfg
}

// unmetCriteria returns the acceptance-criterion ids that lack a current passing
// record (spec 04 R6 input). An id is met when its latest record after the last
// requirements approval is a pass.
func unmetCriteria(root, slug, reqDoc string) []string {
	ids := gates.CriterionIDs(reqDoc)
	if len(ids) == 0 {
		return nil
	}
	records, err := core.LoadCriteria(core.CriteriaPath(root, slug))
	if err != nil {
		// Unreadable ledger ⇒ nothing is attested ⇒ everything is unmet.
		records = nil
	}
	passing := core.CurrentPassing(records, requirementsApprovedAt(root, slug))
	var unmet []string
	for _, id := range ids {
		if !passing[id.String()] {
			unmet = append(unmet, id.String())
		}
	}
	return unmet
}

// requirementsApprovedAt returns the timestamp of the latest requirements-phase
// approval, or the zero time if requirements were never approved. This anchors
// which criterion records count as "current" (spec 04 R5): re-approving
// requirements moves the anchor forward and invalidates stale attestations.
func requirementsApprovedAt(root, slug string) time.Time {
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return time.Time{}
	}
	raw, ok := state.Records["approval:requirements"]
	if !ok {
		return time.Time{}
	}
	var rec core.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return time.Time{}
	}
	at, err := time.Parse(time.RFC3339, rec.Timestamp)
	if err != nil {
		return time.Time{}
	}
	return at
}
