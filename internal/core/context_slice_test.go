package core

import (
	"strings"
	"testing"
)

const sliceTasksFixture = `# Tasks — Demo

## Wave 0
- [ ] T1 — First task
  - why: because
  - role: builder
  - files: a.go

- [x] T2 — Second task
  - role: builder
  - files: b.go

## Wave 1
- [ ] T3 — Third task
  - role: reviewer
`

func TestTaskSlice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		id        string
		wantFound bool
		want      string
	}{
		{
			name:      "first_task_stops_at_next_task",
			id:        "T1",
			wantFound: true,
			want:      "- [ ] T1 — First task\n  - why: because\n  - role: builder\n  - files: a.go",
		},
		{
			name:      "checked_task_stops_at_wave_header",
			id:        "T2",
			wantFound: true,
			want:      "- [x] T2 — Second task\n  - role: builder\n  - files: b.go",
		},
		{
			name:      "last_task_runs_to_eof",
			id:        "T3",
			wantFound: true,
			want:      "- [ ] T3 — Third task\n  - role: reviewer",
		},
		{name: "unknown_task_not_found", id: "T9", wantFound: false, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found := TaskSlice(sliceTasksFixture, tt.id)
			if found != tt.wantFound {
				t.Fatalf("TaskSlice(%q) found = %v, want %v", tt.id, found, tt.wantFound)
			}
			if got != tt.want {
				t.Errorf("TaskSlice(%q) =\n%q\nwant\n%q", tt.id, got, tt.want)
			}
		})
	}
}

const sliceReqFixture = `# Requirements — Demo

## Introduction
Intro text.

## Requirement 1 — Login
**User story:** As a user, I want to log in.

**Acceptance criteria:**
1. WHEN creds valid THE SYSTEM SHALL grant access

## Requirement 2 — Logout
**Acceptance criteria:**
1. WHEN logout clicked THE SYSTEM SHALL end session

## Requirement 3 — Audit
**Acceptance criteria:**
1. THE SYSTEM SHALL log events
`

func TestCoveredRequirements(t *testing.T) {
	t.Parallel()

	t.Run("selects_and_orders_by_number", func(t *testing.T) {
		t.Parallel()
		got, found := CoveredRequirements(sliceReqFixture, []int{3, 1})
		if !found {
			t.Fatal("expected found=true")
		}
		want := "## Requirement 1 — Login\n**User story:** As a user, I want to log in.\n\n**Acceptance criteria:**\n1. WHEN creds valid THE SYSTEM SHALL grant access\n\n## Requirement 3 — Audit\n**Acceptance criteria:**\n1. THE SYSTEM SHALL log events"
		if got != want {
			t.Errorf("CoveredRequirements =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("does_not_bleed_into_introduction", func(t *testing.T) {
		t.Parallel()
		got, found := CoveredRequirements(sliceReqFixture, []int{1})
		if !found {
			t.Fatal("expected found=true")
		}
		if got != "## Requirement 1 — Login\n**User story:** As a user, I want to log in.\n\n**Acceptance criteria:**\n1. WHEN creds valid THE SYSTEM SHALL grant access" {
			t.Errorf("requirement 1 block leaked: %q", got)
		}
	})

	t.Run("unknown_ids_not_found", func(t *testing.T) {
		t.Parallel()
		got, found := CoveredRequirements(sliceReqFixture, []int{9})
		if found || got != "" {
			t.Errorf("CoveredRequirements(9) = (%q, %v), want (\"\", false)", got, found)
		}
	})
}

const sliceDesignFixture = `# Design — Demo

## Overview
Top level overview.

## Architecture
Arch intro.

### Components
Component detail.

### Data Flow
Flow detail.

## Testing
Test plan.
`

func TestDesignSection(t *testing.T) {
	t.Parallel()

	t.Run("section_runs_to_same_level_heading", func(t *testing.T) {
		t.Parallel()
		got, found := DesignSection(sliceDesignFixture, []string{"Architecture"})
		if !found {
			t.Fatal("expected found=true")
		}
		want := "## Architecture\nArch intro.\n\n### Components\nComponent detail.\n\n### Data Flow\nFlow detail."
		if got != want {
			t.Errorf("DesignSection =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("case_insensitive_subheading", func(t *testing.T) {
		t.Parallel()
		got, found := DesignSection(sliceDesignFixture, []string{"components"})
		if !found {
			t.Fatal("expected found=true")
		}
		if got != "### Components\nComponent detail." {
			t.Errorf("DesignSection(components) = %q", got)
		}
	})

	t.Run("multiple_sections_in_document_order", func(t *testing.T) {
		t.Parallel()
		got, found := DesignSection(sliceDesignFixture, []string{"Testing", "Overview"})
		if !found {
			t.Fatal("expected found=true")
		}
		want := "## Overview\nTop level overview.\n\n## Testing\nTest plan."
		if got != want {
			t.Errorf("DesignSection =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("unknown_heading_not_found", func(t *testing.T) {
		t.Parallel()
		got, found := DesignSection(sliceDesignFixture, []string{"Nope"})
		if found || got != "" {
			t.Errorf("DesignSection(Nope) = (%q, %v), want (\"\", false)", got, found)
		}
	})
}

const sliceMemoryFixture = `# Memory — Demo

## first-key
**Pattern:** one
**Detail:** a

## second-key
**Pattern:** two
**Detail:** b

## third-key
**Pattern:** three
**Detail:** c
`

func TestRecentMemory(t *testing.T) {
	t.Parallel()

	t.Run("returns_last_n_entries", func(t *testing.T) {
		t.Parallel()
		got, found := RecentMemory(sliceMemoryFixture, 2)
		if !found {
			t.Fatal("expected found=true")
		}
		want := "## second-key\n**Pattern:** two\n**Detail:** b\n\n## third-key\n**Pattern:** three\n**Detail:** c"
		if got != want {
			t.Errorf("RecentMemory(2) =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("n_exceeding_count_returns_all", func(t *testing.T) {
		t.Parallel()
		got, found := RecentMemory(sliceMemoryFixture, 99)
		if !found {
			t.Fatal("expected found=true")
		}
		if got == "" || !strings.Contains(got, "## first-key") {
			t.Errorf("RecentMemory(99) should include all entries: %q", got)
		}
	})

	t.Run("non_positive_n_not_found", func(t *testing.T) {
		t.Parallel()
		if got, found := RecentMemory(sliceMemoryFixture, 0); found || got != "" {
			t.Errorf("RecentMemory(0) = (%q, %v), want (\"\", false)", got, found)
		}
	})

	t.Run("comment_only_memory_has_no_entries", func(t *testing.T) {
		t.Parallel()
		doc := "# Memory — Demo\n\n<!--\n## not-a-real-entry\n-->\n"
		if got, found := RecentMemory(doc, 3); found || got != "" {
			t.Errorf("RecentMemory(comment-only) = (%q, %v), want (\"\", false)", got, found)
		}
	})
}
