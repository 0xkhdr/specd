package core

import (
	"strings"
	"testing"
)

func TestLintEars_stateMachine(t *testing.T) {
	// Block 1: a numbered line BEFORE the acceptance marker must not be counted
	// as a criterion; the two lines after the marker are. Block 2 is fully valid.
	text := "## Requirement 1: Foo\n" + // 1
		"**User story:** As a user I want X\n" + // 2
		"1. THE SYSTEM SHALL be ignored before marker\n" + // 3 (pre-marker, ignored)
		"**Acceptance criteria:**\n" + // 4
		"1. WHEN x happens THE SYSTEM SHALL y\n" + // 5 (valid)
		"2. this is not an ears criterion\n" + // 6 (invalid → issue)
		"\n" + // 7
		"## Requirement 2: Bar\n" + // 8
		"**User story:** As a user\n" + // 9
		"**Acceptance criteria:**\n" + // 10
		"1. THE SYSTEM SHALL be valid\n" // 11

	issues := LintEars(text)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue, got %d: %v", len(issues), issues)
	}
	if issues[0].Line != 6 {
		t.Errorf("issue line = %d, want 6", issues[0].Line)
	}
	if !strings.Contains(issues[0].Message, "this is not an ears criterion") {
		t.Errorf("issue message = %q, want it to quote the bad criterion", issues[0].Message)
	}
}

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

func TestMatchEars_FormsAndGuards(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		want   EarsPattern
		wantOK bool
	}{
		// One row per documented form.
		{"ubiquitous", "THE SYSTEM SHALL persist state", EarsUbiquitous, true},
		{"event", "WHEN the file changes THE SYSTEM SHALL re-run", EarsEventDriven, true},
		{"state", "WHILE syncing THE SYSTEM SHALL show progress", EarsStateDriven, true},
		{"unwanted", "IF the lock is held THEN THE SYSTEM SHALL wait", EarsUnwanted, true},
		{"optional", "WHERE caching is enabled THE SYSTEM SHALL reuse results", EarsOptionalFeature, true},
		// Case-insensitivity.
		{"lowercase_event", "when the user logs in the system shall greet", EarsEventDriven, true},
		{"mixed_case_ubiquitous", "The System Shall validate input", EarsUbiquitous, true},
		// Combined clause: leading keyword + SHALL anchor; "while" is trigger text.
		{"combined_when_while", "WHEN started, WHILE idle THE SYSTEM SHALL poll", EarsEventDriven, true},
		// False-positive guards: prose that must NOT match any form.
		{"prose_when", "When in doubt, ask the user.", "", false},
		{"prose_system", "The system administrator shall configure the host.", "", false},
		{"empty", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := MatchEars(c.line)
			if ok != c.wantOK {
				t.Fatalf("MatchEars(%q) ok=%v, want %v", c.line, ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Fatalf("MatchEars(%q) = %q, want %q", c.line, got, c.want)
			}
		})
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
