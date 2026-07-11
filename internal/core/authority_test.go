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
	FinalizeAuthority(&a)
	if err := AuthorizeTool(a, "review", nil, a.IssuedAt, "execute", true); err == nil {
		t.Fatal("read-only write accepted")
	}
}
