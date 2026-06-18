package core

import (
	"fmt"
	"os"
	"strings"
)

// MergeSection replaces the content between begin/end markers in the file at
// path, preserving everything outside the markers. If the markers are absent
// the section is appended; if the file does not exist it is created holding only
// the section. This is idempotent: re-running with the same body is a no-op, and
// content the user added outside the markers is always preserved.
func MergeSection(path, begin, end, body string) error {
	existing, _ := os.ReadFile(path)
	ec := string(existing)
	section := begin + "\n" + body + "\n" + end
	if ec == "" {
		return AtomicWrite(path, section+"\n")
	}
	beginIdx, endIdx, present, err := managedSectionBounds(ec, begin, end)
	if err != nil {
		return err
	}
	if present {
		merged := ec[:beginIdx] + section + ec[endIdx+len(end):]
		return AtomicWrite(path, merged)
	}
	if !strings.HasSuffix(ec, "\n") {
		ec += "\n"
	}
	return AtomicWrite(path, ec+"\n"+section+"\n")
}

const agentsMarkerVersion = "v1"

func markerBegin() string {
	return fmt.Sprintf("<!-- SPECD INIT: BEGIN %s (do not edit between markers) -->", agentsMarkerVersion)
}

func markerEnd() string {
	return fmt.Sprintf("<!-- SPECD INIT: END %s -->", agentsMarkerVersion)
}

// MergeAgentsMD merges template content with existing AGENTS.md.
// - If force=true: reset to template with markers (loses customizations)
// - If markers present: replaces content between them, preserves rest
// - If no markers: appends template with markers
// - Preserves content outside markers (unless force=true)
func MergeAgentsMD(path, template string, force bool) error {
	existing, err := os.ReadFile(path)
	var existingContent string
	if err == nil {
		existingContent = string(existing)
	} else if !os.IsNotExist(err) {
		return err
	}

	if force || existingContent == "" {
		result := markerBegin() + "\n" + template + "\n" + markerEnd() + "\n"
		return AtomicWrite(path, result)
	}

	beginMarker := markerBegin()
	endMarker := markerEnd()
	beginIdx, endIdx, present, err := managedSectionBounds(existingContent, beginMarker, endMarker)
	if err != nil {
		return err
	}
	if present {
		before := existingContent[:beginIdx]
		after := existingContent[endIdx+len(endMarker):]
		merged := before + beginMarker + "\n" + template + "\n" + endMarker + after
		return AtomicWrite(path, merged)
	}

	var result string
	if !strings.HasSuffix(existingContent, "\n") {
		result = existingContent + "\n"
	} else {
		result = existingContent
	}
	result += "\n" + beginMarker + "\n" + template + "\n" + endMarker + "\n"

	return AtomicWrite(path, result)
}

func ValidateAgentsMD(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, _, present, err := managedSectionBounds(string(content), markerBegin(), markerEnd())
	return present, err
}

func managedSectionBounds(content, begin, end string) (beginIdx, endIdx int, present bool, err error) {
	beginCount := strings.Count(content, begin)
	endCount := strings.Count(content, end)
	if beginCount == 0 && endCount == 0 {
		return 0, 0, false, nil
	}
	if beginCount != 1 || endCount != 1 {
		return 0, 0, false, fmt.Errorf("managed marker section is malformed: found %d begin and %d end markers", beginCount, endCount)
	}
	beginIdx = strings.Index(content, begin)
	endIdx = strings.Index(content, end)
	if endIdx < beginIdx {
		return 0, 0, false, fmt.Errorf("managed marker section is malformed: end marker precedes begin marker")
	}
	return beginIdx, endIdx, true, nil
}
