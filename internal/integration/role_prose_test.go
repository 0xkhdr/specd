package integration_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

// specdInvocation matches a `specd <verb>` mention in role prose. Role Markdown
// explains behavior; the capability contract defines authority. Any verb named
// in prose must already be permitted by the contract, so prose can never read as
// a grant (spec agent-protocol-clarity R1.2, R6.2).
var specdInvocation = regexp.MustCompile(`specd ([a-z][a-z-]*)`)

func TestRoleProseMatchesCapability(t *testing.T) {
	for _, role := range core.KnownRoles() {
		t.Run(role, func(t *testing.T) {
			capability, ok := core.RoleCapabilityFor(role)
			if !ok {
				t.Fatalf("role %s has no capability contract", role)
			}
			prose := core.RolePrompt(role)
			if prose == "role:invalid\n" {
				t.Fatalf("role %s has no embedded prose", role)
			}
			for _, line := range strings.Split(prose, "\n") {
				// R1.2 targets prose that *instructs* an agent to run a command.
				// Naming a denied verb to forbid it ("Never call `specd
				// complete-task`") states the boundary rather than crossing it.
				if strings.Contains(line, "Never") || strings.Contains(line, "never") {
					continue
				}
				for _, match := range specdInvocation.FindAllStringSubmatch(line, -1) {
					if verb := match[1]; !core.RoleAllowsOperation(role, verb) {
						t.Errorf("prose names %q, denied by contract (allowed: %v)", verb, capability.AllowedOperations)
					}
				}
			}
			// "read-only" is only honest for a role that writes nothing at all,
			// including harness evidence (R2.2).
			writes := false
			for _, effect := range capability.Effects {
				if effect != core.RoleEffectWorkspaceRead {
					writes = true
				}
			}
			if writes && strings.Contains(strings.ToLower(prose), "read-only") {
				t.Errorf("prose claims read-only but contract declares write effects %v", capability.Effects)
			}
		})
	}
}

// TestShippedProseNeverInstructsHumanOnlyCommand covers the other surface R1.2
// names. Steering is not role-scoped — it describes the whole lifecycle, so
// naming `specd new` (intake) is legitimate. What is never legitimate is
// instructing an agent to run a human-only verb: no role's contract can permit
// one, so the instruction is unfollowable by construction and reads as a grant
// the harness will refuse.
func TestShippedProseNeverInstructsHumanOnlyCommand(t *testing.T) {
	for _, dir := range []string{"roles", "steering"} {
		entries, err := embedtemplates.FS.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			t.Run(dir+"/"+entry.Name(), func(t *testing.T) {
				raw, err := embedtemplates.FS.ReadFile(dir + "/" + entry.Name())
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range strings.Split(string(raw), "\n") {
					// Naming a human-only verb to forbid it states the boundary.
					if strings.Contains(line, "Never") || strings.Contains(line, "never") {
						continue
					}
					for _, match := range specdInvocation.FindAllStringSubmatch(line, -1) {
						if command, ok := core.CommandByName(match[1]); ok && command.HumanOnly {
							t.Errorf("prose instructs `specd %s`, which is human-only: %s", match[1], strings.TrimSpace(line))
						}
					}
				}
			})
		}
	}
}
