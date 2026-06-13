package core

import (
	"fmt"
	"os"
	"strings"
)

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
	}

	// Force: reset to default
	if force || existingContent == "" {
		result := markerBegin() + "\n" + template + "\n" + markerEnd() + "\n"
		return AtomicWrite(path, result)
	}

	// Check if markers already exist
	beginMarker := markerBegin()
	endMarker := markerEnd()

	if strings.Contains(existingContent, beginMarker) && strings.Contains(existingContent, endMarker) {
		// Markers present: replace content between them, preserve rest
		beginIdx := strings.Index(existingContent, beginMarker)
		endIdx := strings.Index(existingContent, endMarker)

		if beginIdx < endIdx {
			before := existingContent[:beginIdx]
			after := existingContent[endIdx+len(endMarker):]
			merged := before + beginMarker + "\n" + template + "\n" + endMarker + after
			return AtomicWrite(path, merged)
		}
	}

	// No markers: append template with markers
	var result string
	if !strings.HasSuffix(existingContent, "\n") {
		result = existingContent + "\n"
	} else {
		result = existingContent
	}
	result += "\n" + beginMarker + "\n" + template + "\n" + endMarker + "\n"

	return AtomicWrite(path, result)
}
