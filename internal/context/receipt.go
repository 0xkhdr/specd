package context

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const ReceiptSchemaVersion = "1"

// Receipt is content-free context identity. It records only digests, totals,
// and machine provenance; source paths, selected bytes, prompts, and secrets
// never enter this contract.
type Receipt struct {
	SchemaVersion          string   `json:"schema_version"`
	ManifestDigest         string   `json:"manifest_digest"`
	ConfigDigest           string   `json:"config_digest"`
	PaletteDigest          string   `json:"palette_digest"`
	SkillDigests           []string `json:"skill_digests"`
	RequiredContextDigests []string `json:"required_context_digests"`
	RequiredTokens         int      `json:"required_tokens"`
	OptionalTokens         int      `json:"optional_tokens"`
	CreatedFrom            string   `json:"created_from"`
	Provenance             string   `json:"provenance"`
	ReceiptDigest          string   `json:"receipt_digest"`
}

func BuildReceipt(m MachineManifest) (Receipt, error) {
	if err := ValidateMachineManifest(m); err != nil {
		return Receipt{}, err
	}
	if m.ManifestDigest == "" || m.ManifestDigest != MachineManifestDigest(m) {
		return Receipt{}, fmt.Errorf("manifest_digest is missing or stale")
	}
	r := Receipt{
		SchemaVersion: ReceiptSchemaVersion, ManifestDigest: m.ManifestDigest,
		ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest,
		RequiredTokens: m.RequiredTokens, OptionalTokens: m.OptionalTokens,
		CreatedFrom: "context_manifest:" + m.ManifestDigest,
		Provenance:  "local deterministic selection",
	}
	for _, item := range m.Items {
		digest := selectedDigest(item)
		if item.Required {
			r.RequiredContextDigests = append(r.RequiredContextDigests, digest)
		}
		if item.Kind == "skill" {
			r.SkillDigests = append(r.SkillDigests, digest)
		}
	}
	r.RequiredContextDigests = sortedUniqueStrings(r.RequiredContextDigests)
	r.SkillDigests = sortedUniqueStrings(r.SkillDigests)
	r.ReceiptDigest = receiptDigest(r)
	if err := ValidateReceipt(r); err != nil {
		return Receipt{}, err
	}
	return r, nil
}

func ParseReceipt(raw []byte) (Receipt, error) {
	var r Receipt
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Receipt{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Receipt{}, fmt.Errorf("receipt must contain exactly one JSON object")
	}
	if err := ValidateReceipt(r); err != nil {
		return Receipt{}, err
	}
	return r, nil
}

func ValidateReceipt(r Receipt) error {
	if r.SchemaVersion != ReceiptSchemaVersion {
		return fmt.Errorf("unsupported receipt schema_version %q", r.SchemaVersion)
	}
	if !isSHA256(r.ManifestDigest) || !isSHA256(r.ConfigDigest) || !isSHA256(r.PaletteDigest) {
		return fmt.Errorf("receipt manifest, config, and palette digests must be lowercase SHA-256")
	}
	for _, digest := range append(append([]string(nil), r.RequiredContextDigests...), r.SkillDigests...) {
		if !isSHA256(digest) {
			return fmt.Errorf("receipt selected digest %q must be lowercase SHA-256", digest)
		}
	}
	if r.RequiredTokens < 0 || r.OptionalTokens < 0 || r.CreatedFrom == "" || r.Provenance == "" {
		return fmt.Errorf("receipt totals, created_from, and provenance are invalid")
	}
	if !isSHA256(r.ReceiptDigest) || r.ReceiptDigest != receiptDigest(r) {
		return fmt.Errorf("receipt_digest is missing or stale")
	}
	return nil
}

// ReceiptStaleness reports governing identity changes while preserving the
// old receipt for audit. Optional non-skill context may change without making
// evidence stale; required context and selected skills may not.
func ReceiptStaleness(r Receipt, current MachineManifest) []string {
	if err := ValidateReceipt(r); err != nil {
		return []string{"historical receipt invalid: " + err.Error()}
	}
	now, err := BuildReceipt(current)
	if err != nil {
		return []string{"current manifest invalid: " + err.Error()}
	}
	var reasons []string
	if r.ConfigDigest != now.ConfigDigest {
		reasons = append(reasons, "config digest changed")
	}
	if r.PaletteDigest != now.PaletteDigest {
		reasons = append(reasons, "palette digest changed")
	}
	if !equalStrings(r.RequiredContextDigests, now.RequiredContextDigests) {
		reasons = append(reasons, "required context digests changed")
	}
	if !equalStrings(r.SkillDigests, now.SkillDigests) {
		reasons = append(reasons, "selected skill digests changed")
	}
	return reasons
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func selectedDigest(item MachineItem) string {
	if item.RepresentationDigest != "" {
		return item.RepresentationDigest
	}
	return item.SourceDigest
}

func receiptDigest(r Receipt) string {
	r.ReceiptDigest = ""
	r.RequiredContextDigests = sortedUniqueStrings(r.RequiredContextDigests)
	r.SkillDigests = sortedUniqueStrings(r.SkillDigests)
	raw, _ := json.Marshal(r)
	return core.Digest(raw)
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
