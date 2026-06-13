package core

import "testing"

func TestMatchEars(t *testing.T) {
	cases := []struct {
		line   string
		want   EarsPattern
		wantOK bool
	}{
		{"THE SYSTEM SHALL store data", EarsUbiquitous, true},
		{"WHEN user clicks THE SYSTEM SHALL respond", EarsEventDriven, true},
		{"WHILE processing THE SYSTEM SHALL log", EarsStateDriven, true},
		{"IF error THEN THE SYSTEM SHALL retry", EarsUnwanted, true},
		{"WHERE feature_flag THE SYSTEM SHALL enable", EarsOptionalFeature, true},
		{"This is not EARS", "", false},
	}
	for _, c := range cases {
		got, ok := MatchEars(c.line)
		if ok != c.wantOK {
			t.Errorf("MatchEars(%q) ok=%v, want %v", c.line, ok, c.wantOK)
		}
		if ok && got != c.want {
			t.Errorf("MatchEars(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestLintEars_valid(t *testing.T) {
	doc := `## Requirement 1 — Login

**User story:** As a user I want to log in.

**Acceptance criteria:**
1. THE SYSTEM SHALL accept valid credentials.
2. WHEN invalid password THE SYSTEM SHALL return error.
`
	issues := LintEars(doc)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestLintEars_missingUserStory(t *testing.T) {
	doc := `## Requirement 1

**Acceptance criteria:**
1. THE SYSTEM SHALL do something.
`
	issues := LintEars(doc)
	found := false
	for _, i := range issues {
		if i.Line == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected issue for missing user story")
	}
}

func TestLintEars_noRequirements(t *testing.T) {
	issues := LintEars("# Just a title\n\nNo requirements here.")
	if len(issues) == 0 {
		t.Error("expected issue for missing requirements")
	}
}
