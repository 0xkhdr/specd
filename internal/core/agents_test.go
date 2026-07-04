package core

import (
	"strings"
	"testing"
)

func TestAgentRegistry(t *testing.T) {
	hosts := AgentHosts()
	if len(hosts) < 2 {
		t.Fatalf("AgentHosts returned %d hosts", len(hosts))
	}
	for _, host := range hosts {
		if host.Name == "" || host.Detect == "" || host.Verify == "" {
			t.Fatalf("incomplete host: %+v", host)
		}
	}
}

func TestAgentsMergePreservesUser(t *testing.T) {
	existing := "user note\n\n" + agentsBegin + "\nold\n" + agentsEnd + "\n\nkeep me\n"
	got := MergeAgents(existing, "new")
	if !strings.Contains(got, "user note") || !strings.Contains(got, "keep me") {
		t.Fatalf("MergeAgents lost user content: %q", got)
	}
	if strings.Contains(got, "old") || !strings.Contains(got, "new") {
		t.Fatalf("MergeAgents did not replace managed block: %q", got)
	}
}
