package core

import "strings"

type AgentHost struct {
	Name    string
	Detect  string
	Plan    string
	Install string
	Inspect string
	Verify  string
}

func AgentHosts() []AgentHost {
	return []AgentHost{
		{Name: "codex", Detect: "codex", Plan: "read tasks.md frontier", Install: "none", Inspect: "specd status", Verify: "specd verify"},
		{Name: "claude", Detect: "claude", Plan: "read tasks.md frontier", Install: "none", Inspect: "specd status", Verify: "specd verify"},
	}
}

const (
	agentsBegin = "<!-- specd:agents begin -->"
	agentsEnd   = "<!-- specd:agents end -->"
)

func MergeAgents(existing, generated string) string {
	block := agentsBegin + "\n" + generated + "\n" + agentsEnd
	start := strings.Index(existing, agentsBegin)
	end := strings.Index(existing, agentsEnd)
	if start >= 0 && end >= start {
		end += len(agentsEnd)
		return existing[:start] + block + existing[end:]
	}
	if existing == "" {
		return block + "\n"
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
}
