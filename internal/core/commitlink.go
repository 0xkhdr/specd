package core

import (
	"regexp"
	"sort"
)

// Commit is the minimal view of a git commit the PR summary links against.
type Commit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// CommitLink is one commit with the task IDs it references. A commit that
// references no task is still a CommitLink (Tasks empty) — unreferenced commits
// are listed, never dropped, so reviewers can see work that escaped the task
// graph.
type CommitLink struct {
	SHA     string   `json:"sha"`
	Subject string   `json:"subject"`
	Tasks   []string `json:"tasks"`
}

// taskRefRe matches a task id token (T followed by digits) on a word boundary,
// so "T12" in "fix T12: parser" is found but "INIT12" is not.
var taskRefRe = regexp.MustCompile(`\bT\d+\b`)

// ParseTaskRefs returns the unique task IDs referenced in s, sorted by ordinal
// (T2 before T10) for deterministic output.
func ParseTaskRefs(s string) []string {
	matches := taskRefRe.FindAllString(s, -1)
	if len(matches) == 0 {
		return nil
	}
	set := map[string]bool{}
	var out []string
	for _, m := range matches {
		if !set[m] {
			set[m] = true
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return ordinal(out[i]) < ordinal(out[j]) })
	return out
}

// LinkCommits maps each commit to the task IDs in its subject, preserving input
// order. The result is fully deterministic for a given input. Tasks is non-nil
// (empty slice) so JSON consumers never see null.
func LinkCommits(commits []Commit) []CommitLink {
	out := make([]CommitLink, 0, len(commits))
	for _, c := range commits {
		tasks := ParseTaskRefs(c.Subject)
		if tasks == nil {
			tasks = []string{}
		}
		out = append(out, CommitLink{SHA: c.SHA, Subject: c.Subject, Tasks: tasks})
	}
	return out
}

// UnreferencedCommits returns the links whose commit references no task.
func UnreferencedCommits(links []CommitLink) []CommitLink {
	var out []CommitLink
	for _, l := range links {
		if len(l.Tasks) == 0 {
			out = append(out, l)
		}
	}
	return out
}
