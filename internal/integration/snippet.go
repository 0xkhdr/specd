package integration

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

func Snippet(host, slug, taskID string) string {
	return ModeSnippet(host, core.RequestModeManaged, slug, taskID, core.AssuranceAdvisory)
}

// ModeSnippet projects the canonical request route into host adapter prose.
func ModeSnippet(host string, mode core.RequestMode, slug, taskID string, assurance core.AssuranceLevel) string {
	if host == "" {
		host = "agent"
	}
	return fmt.Sprintf("%s: %s", host, core.RequestModeGuide(mode, slug, taskID, assurance))
}
