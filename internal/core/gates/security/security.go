package security

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/core/gates"
)

type Gate struct{}

func New() Gate {
	return Gate{}
}

func (Gate) Name() string {
	return "security"
}

func (Gate) Run(ctx gates.CheckCtx) []gates.Finding {
	allowlist, findings := loadAllowlist(ctx.Root)
	for _, task := range ctx.Tasks {
		haystack := strings.Join([]string{
			task.ID,
			task.Role,
			task.Files,
			strings.Join(task.DependsOn, " "),
			task.Verify,
			task.Acceptance,
		}, "\n")
		for _, hit := range scanText(haystack) {
			if allowlist.allows(hit.Pattern) {
				continue
			}
			findings = append(findings, gates.Finding{
				Severity: gates.Error,
				Message:  fmt.Sprintf("%s contains %s security pattern %q", task.ID, hit.Scanner, hit.Pattern),
			})
		}
	}
	return findings
}
