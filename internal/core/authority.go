package core

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"

	corescope "github.com/0xkhdr/specd/internal/core/scope"
)

const AuthoritySchemaVersion = "1"

type ToolAuthority struct {
	ID             string   `json:"id"`
	ArgConstraints []string `json:"arg_constraints,omitempty"`
}
type AuthorityV1 struct {
	SchemaVersion      string          `json:"schema_version"`
	ActorID            string          `json:"actor_id"`
	WorkerID           string          `json:"worker_id"`
	SpecID             string          `json:"spec_id"`
	TaskID             string          `json:"task_id"`
	Phase              string          `json:"phase"`
	Role               string          `json:"role"`
	Mode               string          `json:"mode"`
	AllowedTools       []ToolAuthority `json:"allowed_tools"`
	DeniedTools        []string        `json:"denied_tools,omitempty"`
	DeclaredReadPaths  []string        `json:"declared_read_paths"`
	DeclaredWritePaths []string        `json:"declared_write_paths"`
	NetworkPolicy      string          `json:"network_policy"`
	SandboxProfile     string          `json:"sandbox_profile"`
	BaselineRevision   string          `json:"baseline_revision"`
	IssuedAt           time.Time       `json:"issued_at"`
	ExpiresAt          time.Time       `json:"expires_at"`
	PolicyDigest       string          `json:"policy_digest"`
	Digest             string          `json:"digest"`
}

func FinalizeAuthority(a *AuthorityV1) error {
	a.Digest = ""
	canonicalAuthority(a)
	if err := validateAuthorityShape(*a); err != nil {
		return err
	}
	raw, _ := json.Marshal(a)
	a.Digest = Digest(raw)
	return nil
}
func ValidateAuthority(a AuthorityV1, now time.Time, phase string) error {
	digest := a.Digest
	a.Digest = ""
	canonicalAuthority(&a)
	raw, _ := json.Marshal(a)
	if digest == "" || digest != Digest(raw) {
		return fmt.Errorf("AUTHORITY_DIGEST_MISMATCH")
	}
	if err := validateAuthorityShape(a); err != nil {
		return err
	}
	if phase != a.Phase {
		return fmt.Errorf("AUTHORITY_PHASE_MISMATCH")
	}
	if now.Before(a.IssuedAt) || !now.Before(a.ExpiresAt) {
		return fmt.Errorf("AUTHORITY_EXPIRED")
	}
	return nil
}
func validateAuthorityShape(a AuthorityV1) error {
	if a.SchemaVersion != AuthoritySchemaVersion || a.ActorID == "" || a.WorkerID == "" || a.SpecID == "" || a.TaskID == "" || a.Phase == "" || !IsKnownRole(a.Role) || a.PolicyDigest == "" || a.BaselineRevision == "" || a.IssuedAt.IsZero() || !a.ExpiresAt.After(a.IssuedAt) {
		return fmt.Errorf("AUTHORITY_REQUIRED_FIELD_INVALID")
	}
	want := "read_only"
	if IsWriteRole(a.Role) {
		want = "write"
	}
	if a.Mode != want {
		return fmt.Errorf("AUTHORITY_MODE_ROLE_MISMATCH")
	}
	if a.NetworkPolicy != "deny" && a.NetworkPolicy != "allow" {
		return fmt.Errorf("AUTHORITY_NETWORK_INVALID")
	}
	for _, p := range append(append([]string{}, a.DeclaredReadPaths...), a.DeclaredWritePaths...) {
		if _, err := corescope.Normalize(p); err != nil {
			return fmt.Errorf("AUTHORITY_PATH_INVALID: %w", err)
		}
	}
	return nil
}
func canonicalAuthority(a *AuthorityV1) {
	sort.Strings(a.DeniedTools)
	sort.Strings(a.DeclaredReadPaths)
	sort.Strings(a.DeclaredWritePaths)
	sort.SliceStable(a.AllowedTools, func(i, j int) bool { return a.AllowedTools[i].ID < a.AllowedTools[j].ID })
	for i := range a.AllowedTools {
		sort.Strings(a.AllowedTools[i].ArgConstraints)
	}
}
func AuthorizeTool(a AuthorityV1, tool string, paths []string, now time.Time, phase string, mutable bool) error {
	if err := ValidateAuthority(a, now, phase); err != nil {
		return err
	}
	if slices.Contains(a.DeniedTools, tool) {
		return fmt.Errorf("TOOL_DENIED: %s", tool)
	}
	allowed := false
	for _, x := range a.AllowedTools {
		if x.ID == tool {
			allowed = true
		}
	}
	if !allowed {
		return fmt.Errorf("TOOL_DEFAULT_DENY: %s", tool)
	}
	if (a.Role == "validator" && tool == "verify") || (a.Role == "auditor" && tool == "report") {
		mutable = false
	}
	if mutable && a.Mode != "write" {
		return fmt.Errorf("ROLE_WRITE_DENIED: %s", a.Role)
	}
	if mutable {
		for _, p := range paths {
			ok := false
			for _, d := range a.DeclaredWritePaths {
				if p == d {
					ok = true
				}
			}
			if !ok {
				return fmt.Errorf("PATH_WRITE_DENIED: %s", p)
			}
		}
	}
	return nil
}

func BuildAuthority(task TaskRow, actor, worker, slug, phase, baseline, policyDigest, sandbox string, issued, expires time.Time) (AuthorityV1, error) {
	mode := "read_only"
	tools := []string{"status", "context", "check"}
	if IsWriteRole(task.Role) {
		mode = "write"
		tools = append(tools, "verify", "complete-task")
	} else if task.Role == "validator" {
		tools = append(tools, "verify")
	} else if task.Role == "auditor" {
		tools = append(tools, "report")
	}
	allowed := make([]ToolAuthority, 0, len(tools))
	for _, id := range tools {
		allowed = append(allowed, ToolAuthority{ID: id})
	}
	a := AuthorityV1{SchemaVersion: AuthoritySchemaVersion, ActorID: actor, WorkerID: worker, SpecID: slug, TaskID: task.ID, Phase: phase, Role: task.Role, Mode: mode, AllowedTools: allowed, DeniedTools: []string{"approve", "decision", "submit"}, DeclaredReadPaths: append([]string(nil), task.DeclaredFiles...), NetworkPolicy: "deny", SandboxProfile: sandbox, BaselineRevision: baseline, IssuedAt: issued, ExpiresAt: expires, PolicyDigest: policyDigest}
	if mode == "write" {
		a.DeclaredWritePaths = append([]string(nil), task.DeclaredFiles...)
	}
	if err := FinalizeAuthority(&a); err != nil {
		return AuthorityV1{}, err
	}
	return a, nil
}
