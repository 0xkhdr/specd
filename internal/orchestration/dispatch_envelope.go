package orchestration

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

// DispatchEnvelopeV1 is the orchestration name for the shared core wire
// contract. Keeping the type in core prevents CLI, MCP, and remote hosts from
// growing different pinning rules.
type DispatchEnvelopeV1 = core.DispatchV1

func NewDispatchEnvelope(root string, m MissionV1) (DispatchEnvelopeV1, error) {
	e := DispatchEnvelopeV1{
		ProtocolVersion: core.DriverProtocolVersion,
		Root:            root, SpecSlug: m.SpecSlug, TaskID: m.TaskID, Role: m.Role,
		DeclaredFiles: append([]string(nil), m.DeclaredFiles...),
		Acceptance:    append([]string(nil), m.Acceptance...), Verify: m.Verify,
		ContextRef: m.ContextRef, ContextDigest: m.ContextDigest,
		ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest,
		AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead,
	}
	core.CanonicalizeDispatchV1(&e)
	e.EnvelopeDigest = core.DispatchDigest(e)
	if err := core.ValidateDispatchV1(e); err != nil {
		return DispatchEnvelopeV1{}, fmt.Errorf("dispatch envelope: %w", err)
	}
	return e, nil
}

// PinDispatchEnvelope copies immutable checksum into mission. Mission remains
// pending; this function grants no completion authority.
func PinDispatchEnvelope(root string, m *MissionV1) (DispatchEnvelopeV1, error) {
	if m == nil {
		return DispatchEnvelopeV1{}, fmt.Errorf("dispatch envelope: nil mission")
	}
	e, err := NewDispatchEnvelope(root, *m)
	if err != nil {
		return DispatchEnvelopeV1{}, err
	}
	m.DispatchDigest = e.EnvelopeDigest
	return e, nil
}

func ValidateDispatchEnvelope(e DispatchEnvelopeV1) error {
	return core.ValidateDispatchV1(e)
}

func DispatchEnvelopeDigest(e DispatchEnvelopeV1) string {
	return core.DispatchDigest(e)
}
