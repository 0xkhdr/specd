package core

import (
	"testing"
	"time"
)

func validAuthority() AuthorityV1 {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return AuthorityV1{SchemaVersion: AuthoritySchemaVersion, ActorID: "controller", WorkerID: "w1", SpecID: "demo", TaskID: "T1", Phase: "execute", Role: "craftsman", Mode: "write", AllowedTools: []ToolAuthority{{ID: "verify"}}, DeniedTools: []string{"submit"}, DeclaredReadPaths: []string{"a.go"}, DeclaredWritePaths: []string{"a.go"}, NetworkPolicy: "deny", SandboxProfile: "production", BaselineRevision: "abc", IssuedAt: now, ExpiresAt: now.Add(time.Hour), PolicyDigest: "policy"}
}

func TestAuthorityCanonicalDigestAndValidation(t *testing.T) {
	a, b := validAuthority(), validAuthority()
	if err := FinalizeAuthority(&a); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeAuthority(&b); err != nil {
		t.Fatal(err)
	}
	if a.Digest == "" || a.Digest != b.Digest {
		t.Fatalf("digest mismatch")
	}
	b.Role = "scout"
	if ValidateAuthority(b, b.IssuedAt, "execute") == nil {
		t.Fatal("edited packet accepted")
	}
}

func TestAuthorityToolPolicy(t *testing.T) {
	a := validAuthority()
	a.AllowedTools = []ToolAuthority{{ID: "verify"}, {ID: "status"}}
	if err := FinalizeAuthority(&a); err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeTool(a, "verify", nil, a.IssuedAt, "execute", true); err != nil {
		t.Fatal(err)
	}
	a.Mode = "read_only"
	a.Role = "validator"
	a.AllowedTools = append(a.AllowedTools, ToolAuthority{ID: "review"})
	a.Digest = ""
	if err := FinalizeAuthority(&a); err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeTool(a, "review", nil, a.IssuedAt, "execute", true); err == nil {
		t.Fatal("read-only write accepted")
	}
}

func TestAuthorityGrantsNarrowTaskCompletion(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, err := BuildAuthority(TaskRow{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"a.go"}}, "controller", "w1", "demo", "execute", "abc", "policy", "production", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeTool(a, "complete-task", []string{"a.go"}, now, "execute", true); err != nil {
		t.Fatalf("complete-task denied: %v", err)
	}
	if err := AuthorizeTool(a, "task", []string{"a.go"}, now, "execute", true); err == nil {
		t.Fatal("broad task mutation authorized")
	}
}
