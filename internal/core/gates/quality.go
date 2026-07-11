package gates

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// QualityGate returns the opt-in Domain 04 production policy gate. Callers
// append it with CoreRegistryWith, preserving existing core registry order.
func QualityGate() Gate { return gateFunc{name: "quality", run: qualityGate} }

func qualityGate(ctx CheckCtx) []Finding {
	if !ctx.QualityPolicyRequired && len(ctx.QualityPolicies) == 0 {
		return nil
	}
	var findings []Finding
	taskByID := make(map[string]core.TaskRow, len(ctx.Tasks))
	for _, task := range ctx.Tasks {
		taskByID[task.ID] = task
		if ctx.QualityPolicyRequired && core.IsWriteRole(task.Role) {
			if _, ok := ctx.QualityPolicies[task.ID]; !ok {
				findings = append(findings, qualityError("QUALITY_POLICY_REQUIRED", task.ID, "write task has no quality policy"))
			}
		}
	}

	ids := make([]string, 0, len(ctx.QualityPolicies))
	for id := range ctx.QualityPolicies {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		policy := ctx.QualityPolicies[id]
		if policy.TaskID == "" {
			policy.TaskID = id
		}
		var knownChecks map[string]bool
		if len(policy.Checks) > 0 {
			knownChecks = core.QualityCheckIDs(policy)
		}
		for _, finding := range core.ValidateQualityPolicy(policy, knownChecks) {
			findings = append(findings, qualityError(finding.Code, policy.TaskID, strings.TrimPrefix(finding.Message, finding.Code+": ")))
		}
		for _, finding := range core.ValidateCriteria(policy, ctx.KnownCriteria) {
			findings = append(findings, qualityError(finding.Code, policy.TaskID, strings.TrimPrefix(finding.Message, finding.Code+": ")))
		}
		status := core.EvaluateQualityPolicy(policy, ctx.Evals, ctx.QualitySubject)
		for _, req := range status.Missing {
			findings = append(findings, qualityError("QUALITY_EVIDENCE_MISSING", policy.TaskID, fmt.Sprintf("missing %s/%s", req.EvidenceClass, req.CheckID)))
		}
		for _, req := range status.Stale {
			findings = append(findings, qualityError("QUALITY_EVIDENCE_STALE", policy.TaskID, fmt.Sprintf("stale %s/%s", req.EvidenceClass, req.CheckID)))
		}
		if task, ok := taskByID[id]; ok {
			findings = append(findings, verifyQuality(task)...)
		}
	}
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings
}

func verifyQuality(task core.TaskRow) []Finding {
	if !core.IsWriteRole(task.Role) || (task.Risk != "high" && task.Risk != "critical") {
		return nil
	}
	cmd := strings.TrimSpace(task.Verify)
	if core.IsTrivialVerify(cmd, core.DefaultTrivialVerify) {
		return []Finding{qualityError("VERIFY_TRIVIAL", task.ID, "production-risk write task uses trivial verify")}
	}
	if compileOnlyVerify(cmd) {
		return []Finding{qualityError("VERIFY_COMPILE_ONLY", task.ID, "production-risk write task uses compile-only verify")}
	}
	return nil
}

func compileOnlyVerify(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	return strings.HasPrefix(cmd, "go build") || strings.Contains(cmd, "go test -run '^$'") || strings.Contains(cmd, "go test -run \"^$\"")
}

func qualityError(code, taskID, detail string) Finding {
	return Finding{Severity: Error, Message: fmt.Sprintf("%s: %s: %s", code, taskID, detail)}
}
