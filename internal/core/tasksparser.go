package core

import (
	"bytes"
	"fmt"
	"strings"
)

type TaskRow struct {
	ID         string
	Marker     string
	Role       string
	Files      string
	DependsOn  []string
	Verify     string
	Acceptance string
}

type TaskRunStatus string

const (
	TaskPending  TaskRunStatus = "pending"
	TaskRunning  TaskRunStatus = "running"
	TaskComplete TaskRunStatus = "complete"
	TaskBlocked  TaskRunStatus = "blocked"
)

func ParseTasksMd(raw []byte) (TasksMd, error) {
	doc := TasksMd{Raw: append([]byte(nil), raw...)}
	seen := map[string]bool{}
	tables, err := parseMarkdownTables(raw, func(header []string, rows [][]string) error {
		indexes, ok := validateTaskHeader(header)
		if !ok {
			return nil
		}
		for _, cells := range rows {
			marker, id := splitMarkedTaskID(cell(cells, indexes["id"]))
			if id == "" {
				continue
			}
			if seen[id] {
				return formatDuplicateTask(id)
			}
			seen[id] = true
			doc.Tasks = append(doc.Tasks, TaskRow{
				ID:         id,
				Marker:     marker,
				Role:       cell(cells, indexes["role"]),
				Files:      cell(cells, indexes["files"]),
				DependsOn:  splitTaskList(cell(cells, indexes["depends-on"])),
				Verify:     strings.Trim(cell(cells, indexes["verify"]), "`"),
				Acceptance: cell(cells, indexes["acceptance"]),
			})
		}
		return nil
	})
	if err != nil {
		return TasksMd{}, err
	}
	doc.Tables = tables
	return doc, nil
}

func RewriteTaskStatusLine(raw []byte, id string, marker string) ([]byte, error) {
	lines := bytes.SplitAfter(raw, []byte{'\n'})
	out := make([]byte, 0, len(raw)+len(marker)+1)
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimRight(string(line), "\n")
		cells := parsePipeRow(trimmed)
		if cells == nil {
			out = append(out, line...)
			continue
		}
		_, taskID := splitMarkedTaskID(cells[0])
		if taskID != id {
			out = append(out, line...)
			continue
		}
		if changed {
			return nil, fmt.Errorf("duplicate task id %q", id)
		}
		cells[0] = strings.TrimSpace(marker + " " + id)
		newLine := "| " + strings.Join(cells, " | ") + " |"
		if bytes.HasSuffix(line, []byte{'\n'}) {
			newLine += "\n"
		}
		out = append(out, newLine...)
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return out, nil
}

func splitMarkedTaskID(value string) (string, string) {
	value = strings.Trim(strings.TrimSpace(value), "`")
	if value == "" {
		return "", ""
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return "", ""
	}
	for i, part := range parts {
		part = strings.Trim(part, "`")
		if strings.HasPrefix(part, "T") {
			marker := strings.Join(parts[:i], " ")
			return marker, part
		}
	}
	if strings.HasPrefix(parts[0], "T") {
		return "", parts[0]
	}
	return "", value
}
