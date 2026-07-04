package core

import (
	"fmt"
	"sort"
	"strings"
)

type ReportTask struct {
	ID          string        `json:"id"`
	Status      TaskRunStatus `json:"status"`
	Role        string        `json:"role,omitempty"`
	Files       string        `json:"files,omitempty"`
	Verify      string        `json:"verify,omitempty"`
	Acceptance  string        `json:"acceptance,omitempty"`
	EvidenceRef string        `json:"evidence_ref,omitempty"`
}

type ReportModel struct {
	Slug     string       `json:"slug"`
	Total    int          `json:"total"`
	Complete int          `json:"complete"`
	Pending  int          `json:"pending"`
	Blocked  int          `json:"blocked"`
	Running  int          `json:"running"`
	Tasks    []ReportTask `json:"tasks"`
}

func BuildReportModel(slug string, tasks []TaskRow, status map[string]TaskRunStatus, evidence map[string]EvidenceRecord) ReportModel {
	model := ReportModel{Slug: slug, Total: len(tasks), Tasks: make([]ReportTask, 0, len(tasks))}
	for _, task := range tasks {
		taskStatus := status[task.ID]
		if taskStatus == "" {
			taskStatus = statusFromMarker(task.Marker)
		}
		record := evidence[task.ID]
		model.Tasks = append(model.Tasks, ReportTask{
			ID:          task.ID,
			Status:      taskStatus,
			Role:        task.Role,
			Files:       task.Files,
			Verify:      task.Verify,
			Acceptance:  task.Acceptance,
			EvidenceRef: record.EvidenceRef,
		})
		switch taskStatus {
		case TaskComplete:
			model.Complete++
		case TaskBlocked:
			model.Blocked++
		case TaskRunning:
			model.Running++
		default:
			model.Pending++
		}
	}
	return model
}

func RenderStatus(model ReportModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "spec: %s\n", model.Slug)
	fmt.Fprintf(&b, "tasks: %d complete, %d running, %d blocked, %d pending, %d total\n", model.Complete, model.Running, model.Blocked, model.Pending, model.Total)
	for _, task := range model.Tasks {
		fmt.Fprintf(&b, "%s %s", task.ID, task.Status)
		if task.Verify != "" {
			fmt.Fprintf(&b, " verify=%q", task.Verify)
		}
		if task.EvidenceRef != "" {
			fmt.Fprintf(&b, " evidence=%s", task.EvidenceRef)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func RenderPRSummary(model ReportModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## specd report: %s\n\n", model.Slug)
	fmt.Fprintf(&b, "- complete: %d/%d\n", model.Complete, model.Total)
	fmt.Fprintf(&b, "- running: %d\n", model.Running)
	fmt.Fprintf(&b, "- blocked: %d\n", model.Blocked)
	fmt.Fprintf(&b, "- pending: %d\n\n", model.Pending)
	b.WriteString("| task | status | verify |\n|---|---|---|\n")
	for _, task := range model.Tasks {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", task.ID, task.Status, escapeTable(task.Verify))
	}
	return b.String()
}

func statusFromMarker(marker string) TaskRunStatus {
	switch marker {
	case "✅":
		return TaskComplete
	case "🚧":
		return TaskRunning
	case "⛔":
		return TaskBlocked
	default:
		return TaskPending
	}
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

func SortedReportTaskIDs(model ReportModel) []string {
	ids := make([]string, 0, len(model.Tasks))
	for _, task := range model.Tasks {
		ids = append(ids, task.ID)
	}
	sort.Strings(ids)
	return ids
}
