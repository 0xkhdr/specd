package core

import (
	"fmt"
	"strings"
)

type CoverageFinding struct {
	Requirement string
	Message     string
}

func HasTaskTrace(tasks []TaskRow) bool {
	for _, task := range tasks {
		if len(task.Refs) > 0 || task.Kind != "" || task.Risk != "" || task.Context != "" || task.Evidence != "" || task.Checks != "" {
			return true
		}
	}
	return false
}

func AnalyzeCoverage(requirements RequirementsDoc, design DesignDoc, tasks []TaskRow) []CoverageFinding {
	designRefs := map[string]bool{}
	for _, ref := range design.Refs {
		designRefs[strings.TrimSpace(ref)] = true
	}
	taskRefs, deferred := map[string]bool{}, map[string]bool{}
	for _, task := range tasks {
		for _, ref := range task.Refs {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			taskRefs[ref] = true
			if strings.EqualFold(strings.TrimSpace(task.Kind), "deferred") {
				deferred[requirementID(ref)] = true
			}
		}
	}
	var findings []CoverageFinding
	for _, requirement := range requirements.Requirements {
		if !designRefs[requirement.ID] {
			findings = append(findings, CoverageFinding{Requirement: requirement.ID, Message: fmt.Sprintf("%s lacks design coverage", requirement.ID)})
		}
		if !hasRequirementTask(requirement, taskRefs) && !deferred[requirement.ID] {
			findings = append(findings, CoverageFinding{Requirement: requirement.ID, Message: fmt.Sprintf("%s lacks task coverage", requirement.ID)})
		}
		if deferred[requirement.ID] {
			continue
		}
		for _, criterion := range requirement.Criteria {
			if !taskRefs[criterion.ID] {
				findings = append(findings, CoverageFinding{Requirement: criterion.ID, Message: fmt.Sprintf("%s lacks task coverage", criterion.ID)})
			}
		}
	}
	return findings
}

func hasRequirementTask(requirement Requirement, refs map[string]bool) bool {
	if refs[requirement.ID] {
		return true
	}
	for _, criterion := range requirement.Criteria {
		if refs[criterion.ID] {
			return true
		}
	}
	return false
}

func requirementID(ref string) string {
	if i := strings.IndexByte(ref, '.'); i >= 0 {
		return ref[:i]
	}
	return ref
}
