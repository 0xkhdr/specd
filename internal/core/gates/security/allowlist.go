package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowEntry is one reasoned suppression in .specd/security/allow.json. It pins
// the exact fingerprint (file + rule + content hash) and requires a non-empty
// human reason. An entry moving lines still matches; editing the flagged content
// changes the fingerprint and re-surfaces the finding — the point of a reasoned
// allowlist (R2).
type allowEntry struct {
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
}

type allowlist struct {
	byFingerprint map[string]struct{}
}

func (a allowlist) allows(fingerprint string) bool {
	_, ok := a.byFingerprint[fingerprint]
	return ok
}

// loadAllowlist reads .specd/security/allow.json. A missing file is not an error
// (empty allowlist). A malformed file or any entry missing its reason invalidates
// the whole load and fails closed: the returned allowlist suppresses nothing and
// an error finding is emitted (R2).
func loadAllowlist(root string) (allowlist, []Finding) {
	path := filepath.Join(root, ".specd", "security", "allow.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return allowlist{byFingerprint: map[string]struct{}{}}, nil
	}
	if err != nil {
		return failClosed(), []Finding{{Scanner: "allowlist", Rule: "load", Severity: "error", Excerpt: fmt.Sprintf("allowlist read failed: %v", err)}}
	}
	var entries []allowEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return failClosed(), []Finding{{Scanner: "allowlist", Rule: "parse", Severity: "error", Excerpt: fmt.Sprintf("allowlist parse failed: %v", err)}}
	}
	byFP := make(map[string]struct{}, len(entries))
	for i, e := range entries {
		fp := strings.TrimSpace(e.Fingerprint)
		if fp == "" {
			return failClosed(), []Finding{{Scanner: "allowlist", Rule: "fingerprint", Severity: "error", Excerpt: fmt.Sprintf("allowlist entry %d missing fingerprint", i)}}
		}
		if strings.TrimSpace(e.Reason) == "" {
			return failClosed(), []Finding{{Scanner: "allowlist", Rule: "reason", Severity: "error", Excerpt: fmt.Sprintf("allowlist entry %s missing reason", fp)}}
		}
		byFP[fp] = struct{}{}
	}
	return allowlist{byFingerprint: byFP}, nil
}

func failClosed() allowlist {
	return allowlist{byFingerprint: map[string]struct{}{}}
}
