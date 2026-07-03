package core

import "testing"

func TestNoDuplicateCommands(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Commands {
		if seen[c.Command] {
			t.Fatalf("duplicate command %q", c.Command)
		}
		seen[c.Command] = true
	}
}

func TestFlagSingleOwner(t *testing.T) {
	allowed := map[string]map[string]bool{
		"sandbox":        {"verify": true},
		"revert-on-fail": {"verify": true},
		"all":            {"next": true, "status": true, "help": true},
		"format":         {"report": true},
		"evidence":       {"verify": true, "task": true, "promote": true},
	}
	seen := map[string]map[string]bool{}
	for _, c := range Commands {
		for _, f := range c.Flags {
			if allowed[f.Name] == nil {
				continue
			}
			if !allowed[f.Name][c.Command] {
				t.Fatalf("--%s unexpectedly owned by %s", f.Name, c.Command)
			}
			if seen[f.Name] == nil {
				seen[f.Name] = map[string]bool{}
			}
			seen[f.Name][c.Command] = true
		}
	}
	for flag, owners := range allowed {
		for owner := range owners {
			if !seen[flag][owner] {
				t.Fatalf("--%s missing owner %s", flag, owner)
			}
		}
	}
}

func TestPaletteCeiling(t *testing.T) {
	daily, total := 0, 0
	for _, c := range Commands {
		total++
		if !c.Hidden {
			daily++
		}
	}
	// v0.2.0 Wave 3 added eval/conductor; Wave 4 added the trust/scale surfaces
	// (orchestrate, review, submit — promote stays Hidden); Wave 5 adds the
	// lifecycle-close surfaces (deploy, observe, ingest); Wave 6 adds the platform
	// surfaces (harness, dashboard, migrate) as Hidden infra so the daily palette
	// stays bounded while total grows. The palette stays deliberately bounded.
	if daily > 24 {
		t.Fatalf("daily palette = %d, want <=24", daily)
	}
	if total > 32 {
		t.Fatalf("total commands = %d, want <=32", total)
	}
}
