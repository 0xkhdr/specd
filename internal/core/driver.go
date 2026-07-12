package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const DriverProtocolVersion = "1"

// DispatchV1 is the transport-neutral, digest-pinned task packet. It contains
// identities and references only; repository bytes and worker output never
// cross this boundary.
type DispatchV1 struct {
	ProtocolVersion string   `json:"protocol_version"`
	Root            string   `json:"root"`
	SpecSlug        string   `json:"spec_slug"`
	TaskID          string   `json:"task_id"`
	Role            string   `json:"role"`
	DeclaredFiles   []string `json:"declared_files"`
	Acceptance      []string `json:"acceptance"`
	Verify          string   `json:"verify"`
	ContextRef      string   `json:"context_ref"`
	ContextDigest   string   `json:"context_digest"`
	ConfigDigest    string   `json:"config_digest"`
	PaletteDigest   string   `json:"palette_digest"`
	AuthorityRef    string   `json:"authority_ref"`
	SubjectHead     string   `json:"subject_head"`
	EnvelopeDigest  string   `json:"envelope_digest,omitempty"`
}

func CanonicalizeDispatchV1(d *DispatchV1) {
	sort.Strings(d.DeclaredFiles)
	sort.Strings(d.Acceptance)
}

func DispatchDigest(d DispatchV1) string {
	d.EnvelopeDigest = ""
	CanonicalizeDispatchV1(&d)
	raw, _ := json.Marshal(d)
	return Digest(raw)
}

func ValidateDispatchV1(d DispatchV1) error {
	if d.ProtocolVersion != DriverProtocolVersion {
		return fmt.Errorf("DISPATCH_VERSION_UNSUPPORTED")
	}
	if d.Root == "" || d.SpecSlug == "" || d.TaskID == "" || d.Role == "" || d.Verify == "" || d.ContextRef == "" || d.ContextDigest == "" || d.ConfigDigest == "" || d.PaletteDigest == "" || d.AuthorityRef == "" || d.SubjectHead == "" {
		return fmt.Errorf("DISPATCH_REQUIRED_PIN_MISSING")
	}
	seen := map[string]bool{}
	for _, path := range d.DeclaredFiles {
		if path == "" || seen[path] || path[0] == '/' || path == "." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
			return fmt.Errorf("DISPATCH_FILE_INVALID")
		}
		seen[path] = true
	}
	if d.EnvelopeDigest != "" && d.EnvelopeDigest != DispatchDigest(d) {
		return fmt.Errorf("DISPATCH_DIGEST_MISMATCH")
	}
	return nil
}

type DriverFinding struct {
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Ref            string `json:"ref,omitempty"`
	Message        string `json:"message,omitempty"`
	RecoveryAction string `json:"recovery_action"`
}

type BootstrapV1 struct {
	ProtocolVersion     string          `json:"protocol_version"`
	Root                string          `json:"root"`
	Specs               []string        `json:"specs"`
	ActiveSpec          string          `json:"active_spec,omitempty"`
	Resolution          string          `json:"resolution,omitempty"`
	PaletteDigest       string          `json:"palette_digest"`
	ConfigDigest        string          `json:"config_digest"`
	GuidanceDigest      string          `json:"guidance_digest"`
	ContextSchemaDigest string          `json:"context_schema_digest"`
	Findings            []DriverFinding `json:"findings"`
}

type NextAction struct {
	ID                string   `json:"id"`
	Command           string   `json:"command"`
	Args              []string `json:"args,omitempty"`
	Actor             string   `json:"actor"`
	SideEffect        string   `json:"side_effect"`
	AuthorityRequired bool     `json:"authority_required"`
	AllowedPhases     []Phase  `json:"allowed_phases"`
	SourceRef         string   `json:"source_ref"`
	Reason            string   `json:"reason,omitempty"`
}

type DriverGuideV1 struct {
	ProtocolVersion string          `json:"protocol_version"`
	Root            string          `json:"root"`
	SpecSlug        string          `json:"spec_slug"`
	Phase           Phase           `json:"phase"`
	Status          Status          `json:"status"`
	Approvals       []string        `json:"approvals"`
	Frontier        []string        `json:"frontier"`
	Blockers        []DriverFinding `json:"blockers"`
	NextActions     []NextAction    `json:"next_actions"`
	EvidenceRefs    []string        `json:"evidence_refs"`
	Compatibility   string          `json:"compatibility"`
}

func ValidateBootstrapV1(b BootstrapV1) error {
	if b.ProtocolVersion != DriverProtocolVersion {
		return fmt.Errorf("DRIVER_VERSION_UNSUPPORTED: re-bootstrap with compatible driver version")
	}
	if b.Root == "" || b.PaletteDigest == "" || b.ConfigDigest == "" || b.GuidanceDigest == "" || b.ContextSchemaDigest == "" {
		return fmt.Errorf("DRIVER_REQUIRED_FIELD: re-bootstrap to refresh required digests")
	}
	for _, finding := range b.Findings {
		if finding.Code == "" || finding.Severity == "" || finding.RecoveryAction == "" {
			return fmt.Errorf("DRIVER_FINDING_INVALID: regenerate bootstrap findings")
		}
	}
	return nil
}

func CanonicalizeBootstrap(b *BootstrapV1) {
	sort.Strings(b.Specs)
	sort.Slice(b.Findings, func(i, j int) bool {
		if b.Findings[i].Code != b.Findings[j].Code {
			return b.Findings[i].Code < b.Findings[j].Code
		}
		return b.Findings[i].Ref < b.Findings[j].Ref
	})
}

func CanonicalizeDriverGuide(g *DriverGuideV1) {
	sort.Strings(g.Approvals)
	sort.Strings(g.Frontier)
	sort.Strings(g.EvidenceRefs)
	sort.Slice(g.Blockers, func(i, j int) bool { return g.Blockers[i].Code < g.Blockers[j].Code })
	sort.Slice(g.NextActions, func(i, j int) bool { return g.NextActions[i].ID < g.NextActions[j].ID })
}

func DriverDigest(v any) string {
	switch value := v.(type) {
	case BootstrapV1:
		CanonicalizeBootstrap(&value)
		v = value
	case DriverGuideV1:
		CanonicalizeDriverGuide(&value)
		v = value
	}
	raw, _ := json.Marshal(v)
	return Digest(raw)
}

// ProjectDriverGuide derives legal actions from lifecycle + canonical palette.
func ProjectDriverGuide(root, slug string, status Status, approvals, frontier []string, blockers []DriverFinding) DriverGuideV1 {
	phase := PhaseForStatus(status)
	g := DriverGuideV1{ProtocolVersion: DriverProtocolVersion, Root: root, SpecSlug: slug, Phase: phase, Status: status, Approvals: append([]string(nil), approvals...), Frontier: append([]string(nil), frontier...), Blockers: append([]DriverFinding(nil), blockers...), Compatibility: "v1"}
	add := func(id, name string, args []string, actor, effect, reason string) {
		command, ok := CommandByName(name)
		if !ok || command.Deferred || !command.AllowsPhase(phase) {
			return
		}
		g.NextActions = append(g.NextActions, NextAction{ID: id, Command: name, Args: args, Actor: actor, SideEffect: effect, AuthorityRequired: actor == "human" || effect != "read", AllowedPhases: append([]Phase(nil), command.AllowedPhases...), SourceRef: "core.Commands/" + name, Reason: reason})
	}
	add("10-status", "status", []string{slug, "--guide", "--json"}, "agent", "read", "inspect deterministic guidance")
	if len(frontier) > 0 {
		add("20-context", "context", []string{slug, frontier[0], "--json"}, "agent", "read", "load required task context")
		add("30-verify", "verify", []string{slug, frontier[0]}, "agent", "write", "record task evidence")
	}
	if next := NextStatus(status); next != "" {
		add("90-approve", "approve", []string{slug, string(next)}, "human", "approval", "advance only after gates pass")
	}
	CanonicalizeDriverGuide(&g)
	return g
}
