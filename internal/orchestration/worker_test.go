package orchestration

import (
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestWorkerClaimValidatesCapabilityAndPins(t *testing.T) {
	m := validMission()
	w := WorkerV1{WorkerID: "worker-1", Host: "local", Roles: []string{"craftsman"}, Capabilities: []string{"edit", "verify"}}
	l, err := ClaimMission(m, w, ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}, m.IssuedAt, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if l.WorkerID != w.WorkerID || l.State != LeaseActive || l.LeaseID == "" {
		t.Fatalf("lease = %+v", l)
	}
	w.Roles = []string{"scout"}
	if _, err := ClaimMission(m, w, ClaimEcho{MissionID: m.MissionID}, m.IssuedAt, time.Minute); err == nil || !strings.Contains(err.Error(), "ROLE") {
		t.Fatalf("role mismatch err = %v", err)
	}
}

func TestWorkerClaimConflict(t *testing.T) {
	m := validMission()
	active := Lease{LeaseID: "l", MissionID: m.MissionID, TaskID: m.TaskID, Attempt: 1, WorkerID: "other", IssuedAt: m.IssuedAt, ExpiresAt: m.ExpiresAt, PolicyDigest: m.PolicyDigest, State: LeaseActive}
	if err := CheckClaimConflict([]Lease{active}, m, m.IssuedAt); err == nil {
		t.Fatal("conflicting live claim accepted")
	}
}

func TestCompleteAuthorityIsNarrow(t *testing.T) {
	m := validMission()
	w := WorkerV1{WorkerID: "worker-1", Host: "local", Roles: []string{m.Role}}
	e := ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}
	lease, err := ClaimMission(m, w, e, m.IssuedAt, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.AuthorizeTool(lease.Authority, "complete-task", m.DeclaredFiles, m.IssuedAt, "execute", true); err != nil {
		t.Fatalf("complete-task denied: %v", err)
	}
	if err := core.AuthorizeTool(lease.Authority, "task", m.DeclaredFiles, m.IssuedAt, "execute", true); err == nil {
		t.Fatal("generic task mutation authorized")
	}
}

// echoFor builds a matching claim echo for a mission.
func echoFor(m MissionV1) ClaimEcho {
	return ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}
}

// TestWorkerOutOfScopeRefusesUnnamedWorker pins spec R6.4: a mission the plan
// pinned to worker w1 is refused as an out-of-scope class refusal when claimed
// by w2, accepted when claimed by w1, and a dash/empty worker imposes no
// restriction.
func TestWorkerOutOfScopeRefusesUnnamedWorker(t *testing.T) {
	m := validMission()
	m.Worker = "w1"
	echo := echoFor(m)

	w2 := WorkerV1{WorkerID: "w2", Host: "local", Roles: []string{m.Role}}
	_, err := ClaimMission(m, w2, echo, m.IssuedAt, time.Minute)
	refusal, ok := core.AsRefusal(err)
	if !ok || refusal.Code != "WORKER_OUT_OF_SCOPE" {
		t.Fatalf("want WORKER_OUT_OF_SCOPE refusal, got %v", err)
	}
	if refusal.Retryable || refusal.Category != "scope" {
		t.Fatalf("out-of-scope must be a non-retryable scope class: %+v", refusal)
	}
	for _, want := range []string{"w1", "w2", m.TaskID} {
		if !strings.Contains(refusal.Error(), want) {
			t.Fatalf("refusal %q missing %q", refusal.Error(), want)
		}
	}

	w1 := WorkerV1{WorkerID: "w1", Host: "local", Roles: []string{m.Role}}
	if _, err := ClaimMission(m, w1, echo, m.IssuedAt, time.Minute); err != nil {
		t.Fatalf("named worker refused its own mission: %v", err)
	}

	for _, worker := range []string{"", "-"} {
		md := validMission()
		md.Worker = worker
		if _, err := ClaimMission(md, w2, echoFor(md), md.IssuedAt, time.Minute); err != nil {
			t.Fatalf("host-chooses worker %q imposed a restriction: %v", worker, err)
		}
	}
}
