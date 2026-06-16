package core

import (
	"strings"
	"testing"
)

func TestCommitTaskLink(t *testing.T) {
	t.Run("parse refs deterministic, ordinal-sorted, unique", func(t *testing.T) {
		got := ParseTaskRefs("T10 fix and T2 again, also T2 and INIT12 not a ref")
		if strings.Join(got, ",") != "T2,T10" {
			t.Fatalf("ParseTaskRefs = %v, want [T2 T10]", got)
		}
		if ParseTaskRefs("no refs here") != nil {
			t.Error("expected nil for no refs")
		}
	})

	t.Run("unreferenced commits listed, not dropped", func(t *testing.T) {
		commits := []Commit{
			{SHA: "aaaaaaaa1111", Subject: "T1: build parser"},
			{SHA: "bbbbbbbb2222", Subject: "chore: tidy go.mod"}, // no ref
			{SHA: "cccccccc3333", Subject: "T2 and T3: wire gate"},
		}
		links := LinkCommits(commits)
		if len(links) != 3 {
			t.Fatalf("want 3 links (none dropped), got %d", len(links))
		}
		if strings.Join(links[0].Tasks, ",") != "T1" {
			t.Errorf("commit0 tasks = %v", links[0].Tasks)
		}
		if len(links[1].Tasks) != 0 {
			t.Errorf("commit1 should have empty (non-nil) tasks, got %v", links[1].Tasks)
		}
		if links[1].Tasks == nil {
			t.Error("Tasks must be non-nil for JSON")
		}
		if strings.Join(links[2].Tasks, ",") != "T2,T3" {
			t.Errorf("commit2 tasks = %v", links[2].Tasks)
		}

		unref := UnreferencedCommits(links)
		if len(unref) != 1 || unref[0].SHA != "bbbbbbbb2222" {
			t.Fatalf("unreferenced = %+v, want only the chore commit", unref)
		}
	})

	t.Run("deterministic across runs", func(t *testing.T) {
		commits := []Commit{{SHA: "x", Subject: "T3 T1 T2"}}
		a := LinkCommits(commits)
		b := LinkCommits(commits)
		if strings.Join(a[0].Tasks, ",") != strings.Join(b[0].Tasks, ",") {
			t.Error("non-deterministic output")
		}
		if strings.Join(a[0].Tasks, ",") != "T1,T2,T3" {
			t.Errorf("tasks = %v, want sorted [T1 T2 T3]", a[0].Tasks)
		}
	})
}
