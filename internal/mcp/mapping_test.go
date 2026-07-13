package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/adapter"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestMCPMapPreservesMissionEnvelope(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	mission := orchestration.MissionV1{ProtocolVersion: "1", SessionID: "s", MissionID: "m", SpecSlug: "demo", TaskID: "T13", Attempt: 1, Role: "craftsman", AuthorityRef: "auth", DeclaredFiles: []string{"a.go"}, Acceptance: []string{"R10.1"}, Verify: "go test", ContextRef: "ctx.json", ContextDigest: "ctx", ConfigDigest: "config", PaletteDigest: "palette", PolicyDigest: "policy", SubjectHead: "head", RouteClass: "local", RouteReason: "test", Limits: orchestration.MissionLimits{MaxAttempts: 1, TimeoutSeconds: 30}, IssuedAt: now, ExpiresAt: now.Add(time.Minute), Status: orchestration.MissionPending}
	req, err := adapter.MissionRequest(mission, "request", "correlation", "mcp")
	if err != nil {
		t.Fatal(err)
	}
	call, err := MissionToolCall(req, "brain.next")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(call)
	for _, pin := range []string{"authority_ref", "spec_slug", "task_id", "mission_id", "subject_head", "acceptance", "verify"} {
		if !jsonContainsKey(raw, pin) {
			t.Fatalf("MCP mapping lost %q: %s", pin, raw)
		}
	}
}

func jsonContainsKey(raw []byte, key string) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	return containsKey(value, key)
}

func containsKey(value any, key string) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, ok := typed[key]; ok {
			return true
		}
		for _, child := range typed {
			if containsKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsKey(child, key) {
				return true
			}
		}
	}
	return false
}
