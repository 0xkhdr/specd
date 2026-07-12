package context

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReceiptStableAndContentFree(t *testing.T) {
	m := receiptManifest()
	r1, err := BuildReceipt(m)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := BuildReceipt(m)
	if err != nil {
		t.Fatal(err)
	}
	if r1.ReceiptDigest == "" || r1.ReceiptDigest != r2.ReceiptDigest {
		t.Fatalf("unstable receipt: %+v %+v", r1, r2)
	}
	if len(r1.SkillDigests) != 1 || len(r1.RequiredContextDigests) != 1 {
		t.Fatalf("receipt digests = %+v", r1)
	}
	raw, err := json.Marshal(r1)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"super-secret-content", ".specd/secret.md", "items", "prompt", "transcript"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("receipt leaked %q: %s", forbidden, raw)
		}
	}
}

func TestReceiptFreshnessAndHistoricalDecode(t *testing.T) {
	m := receiptManifest()
	r, err := BuildReceipt(m)
	if err != nil {
		t.Fatal(err)
	}
	if stale := ReceiptStaleness(r, m); len(stale) != 0 {
		t.Fatalf("fresh receipt reported stale: %v", stale)
	}
	changed := m
	changed.Items = append([]ItemV2(nil), m.Items...)
	changed.Items[0].RepresentationDigest = strings.Repeat("d", 64)
	changed.ManifestDigest = ManifestV2Digest(changed)
	stale := ReceiptStaleness(r, changed)
	if len(stale) == 0 || stale[0] != "required context digests changed" {
		t.Fatalf("staleness = %v", stale)
	}

	raw, _ := json.Marshal(r)
	decoded, err := ParseReceipt(raw)
	if err != nil || decoded.ReceiptDigest != r.ReceiptDigest {
		t.Fatalf("historical receipt unreadable: %+v %v", decoded, err)
	}
	tampered := r
	tampered.RequiredTokens++
	if err := ValidateReceipt(tampered); err == nil {
		t.Fatal("tampered receipt accepted")
	}
}

func receiptManifest() ManifestV2 {
	m := ManifestV2{
		SchemaVersion: ManifestVersionV2, Kind: manifestKindV2, Root: "/repo", Slug: "demo",
		Action: "execute", Phase: "execute", TaskID: "T1", ConfigDigest: strings.Repeat("a", 64),
		PaletteDigest: strings.Repeat("b", 64), RequiredTokens: 3, OptionalTokens: 5,
		Items: []ItemV2{
			{Kind: "knowledge", Source: ".specd/secret.md", SourceDigest: strings.Repeat("c", 64), RepresentationDigest: strings.Repeat("c", 64), Required: true, LoadMode: "eager", Reason: "super-secret-content", Trust: "knowledge", ContentTrust: ContentTrustUntrustedData, EstimatedTokens: 3},
			{Kind: "skill", Source: ".specd/skills/x/SKILL.md", SourceDigest: strings.Repeat("e", 64), RepresentationDigest: strings.Repeat("e", 64), LoadMode: "lazy", Reason: "skill", Trust: "knowledge", ContentTrust: ContentTrustUntrustedData, EstimatedTokens: 5},
		},
		Provenance: "local deterministic selection",
	}
	CanonicalizeV2(&m)
	m.ManifestDigest = ManifestV2Digest(m)
	return m
}
