package core

import (
	"bytes"
	"fmt"
	"strings"
)

// MarkdownTable stores one parsed pipe table while retaining the original bytes.
type MarkdownTable struct {
	Header []string
	Rows   [][]string
}

// TasksMd is the parsed representation of a tasks.md file. SerializeTasksMd
// returns Raw unchanged unless a caller deliberately builds a new value.
type TasksMd struct {
	Raw    []byte
	Tasks  []TaskRow
	Tables []MarkdownTable
}

// SerializeTasksMd preserves the source bytes exactly. The task DAG uses
// parsed rows for decisions, never rewritten markdown.
func SerializeTasksMd(doc TasksMd) []byte {
	out := make([]byte, len(doc.Raw))
	copy(out, doc.Raw)
	return out
}

func parsePipeRow(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	line = strings.TrimPrefix(strings.TrimSuffix(line, "|"), "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false
		}
		for _, r := range cell {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
	}
	return true
}

func canonicalHeader(cells []string) []string {
	out := make([]string, len(cells))
	for i, cell := range cells {
		out[i] = strings.ToLower(strings.TrimSpace(cell))
	}
	return out
}

func headerIndex(header []string, name string) int {
	for i, cell := range header {
		if cell == name {
			return i
		}
	}
	return -1
}

func cell(cells []string, index int) string {
	if index < 0 || index >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[index])
}

func splitTaskList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" || strings.EqualFold(value, "none") {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		field = strings.TrimSpace(strings.Trim(field, "`"))
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
	}
	return out
}

func parseMarkdownTables(raw []byte, visit func(header []string, rows [][]string) error) ([]MarkdownTable, error) {
	lines := bytes.Split(raw, []byte{'\n'})
	var tables []MarkdownTable
	for i := 0; i < len(lines); i++ {
		header := parsePipeRow(string(lines[i]))
		if header == nil || i+1 >= len(lines) {
			continue
		}
		separator := parsePipeRow(string(lines[i+1]))
		if !isSeparatorRow(separator) {
			continue
		}
		table := MarkdownTable{Header: canonicalHeader(header)}
		i += 2
		for ; i < len(lines); i++ {
			row := parsePipeRow(string(lines[i]))
			if row == nil {
				break
			}
			table.Rows = append(table.Rows, row)
		}
		tables = append(tables, table)
		if err := visit(table.Header, table.Rows); err != nil {
			return nil, err
		}
	}
	return tables, nil
}

func validateTaskHeader(header []string) (map[string]int, bool) {
	indexes := map[string]int{
		"id":         headerIndex(header, "id"),
		"role":       headerIndex(header, "role"),
		"files":      headerIndex(header, "files"),
		"depends-on": headerIndex(header, "depends-on"),
		"verify":     headerIndex(header, "verify"),
		"acceptance": headerIndex(header, "acceptance"),
	}
	for _, index := range indexes {
		if index < 0 {
			return indexes, false
		}
	}
	return indexes, true
}

func formatDuplicateTask(id string) error {
	return fmt.Errorf("duplicate task id %q", id)
}
