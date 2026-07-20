package context

import "testing"

// R3.1: the required lanes are exactly what a host must acknowledge, so the
// set must be complete, deterministic, and free of optional items.
func TestContextReceiptRequiredDigestsSelectsOnlyRequiredLanes(t *testing.T) {
	manifest := MachineManifest{Items: []MachineItem{
		{Kind: "design", Required: true, RepresentationDigest: "d2"},
		{Kind: "source", Required: false, RepresentationDigest: "optional"},
		{Kind: "requirements", Required: true, RepresentationDigest: "d1"},
		{Kind: "role", Required: true, SourceDigest: "d3"},
	}}
	got := RequiredDigests(manifest)
	if len(got) != 3 {
		t.Fatalf("got %v, want the three required lanes", got)
	}
	for _, digest := range got {
		if digest == "optional" {
			t.Fatalf("optional item leaked into the required set: %v", got)
		}
	}
	// Sorted, so a receipt built twice from one manifest digests identically.
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("required digests are not in deterministic order: %v", got)
		}
	}
}

// An item with no digest cannot be acknowledged, so it must not appear as a
// required lane a host can never satisfy.
func TestContextReceiptRequiredDigestsSkipsDigestlessItems(t *testing.T) {
	manifest := MachineManifest{Items: []MachineItem{
		{Kind: "requirements", Required: true},
		{Kind: "design", Required: true, RepresentationDigest: "d1"},
	}}
	if got := RequiredDigests(manifest); len(got) != 1 || got[0] != "d1" {
		t.Fatalf("got %v, want only the item that carries a digest", got)
	}
}

// RepresentationDigest wins over SourceDigest: the host acknowledges what it
// was actually given, not the file it came from.
func TestContextReceiptRequiredDigestsPrefersRepresentation(t *testing.T) {
	manifest := MachineManifest{Items: []MachineItem{
		{Kind: "source", Required: true, SourceDigest: "source", RepresentationDigest: "representation"},
	}}
	if got := RequiredDigests(manifest); len(got) != 1 || got[0] != "representation" {
		t.Fatalf("got %v, want the representation digest", got)
	}
}
