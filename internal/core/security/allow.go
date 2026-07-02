package security

import (
	"encoding/json"
	"fmt"
	"strings"
)

// allow.go implements the secrets allowlist read from
// .specd/security/allow.json. Every entry MUST carry a non-empty reason — an
// allowlist without justification is how real leaks get waved through, so a
// reasonless entry is a hard parse error (spec §4).

// AllowEntry is one allowlisted value and the mandatory justification for it.
type AllowEntry struct {
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// Allowlist is the parsed, validated set of allowlisted secret values.
type Allowlist struct {
	entries []AllowEntry
}

// Allows reports whether value is allowlisted (exact match or substring of a
// finding). The zero Allowlist allows nothing.
func (a Allowlist) Allows(value string) bool {
	for _, e := range a.entries {
		if e.Value != "" && strings.Contains(value, e.Value) {
			return true
		}
	}
	return false
}

// Entries exposes the validated entries (for reporting/introspection).
func (a Allowlist) Entries() []AllowEntry { return a.entries }

// ParseAllowlist parses and validates the allow.json bytes. An absent file is
// represented by empty input (nil/zero-length) → an empty allowlist, no error.
// Every present entry must have a non-empty value and a non-empty reason.
func ParseAllowlist(data []byte) (Allowlist, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return Allowlist{}, nil
	}
	var doc struct {
		Allow []AllowEntry `json:"allow"`
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return Allowlist{}, fmt.Errorf("invalid security allowlist: %w", err)
	}
	for i, e := range doc.Allow {
		if strings.TrimSpace(e.Value) == "" {
			return Allowlist{}, fmt.Errorf("security allowlist entry %d has an empty value", i)
		}
		if strings.TrimSpace(e.Reason) == "" {
			return Allowlist{}, fmt.Errorf("security allowlist entry %d (%q) has no reason — allowlisting a secret requires a justification", i, e.Value)
		}
	}
	return Allowlist{entries: doc.Allow}, nil
}
