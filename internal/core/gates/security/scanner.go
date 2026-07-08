// Package security implements the opt-in deterministic security gate (spec 05).
// It scans the operator's own tracked files for accidentally committed secrets,
// prompt-injection payloads, and typosquatted dependencies. Every scanner is a
// pure function of tracked file contents + embedded rule data + the allowlist:
// no network, no LLM, stable finding order (R7).
package security

import (
	"crypto/sha256"
	"encoding/hex"
)

// TrackedFile is one working-tree file the gate feeds to scanners. Path is the
// repo-relative slash path (git ls-files semantics); Content is the raw bytes.
type TrackedFile struct {
	Path    string
	Content []byte
}

// Finding is one scanner hit before severity resolution. Fingerprint pins the
// finding to (rule + path + matched content) so a reasoned allowlist entry
// survives the match moving lines but is invalidated by editing the match (R2).
// Excerpt is always redacted — a secrets scanner that prints secrets into CI
// logs creates the leak it exists to prevent (R1).
type Finding struct {
	Scanner     string `json:"scanner"`
	Rule        string `json:"rule"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Fingerprint string `json:"fingerprint"`
	Excerpt     string `json:"excerpt"`
	// Severity is filled by the gate from config; empty until resolved.
	Severity string `json:"severity,omitempty"`
	// Allowlisted is set by the gate when an allow.json entry suppresses the
	// finding. Recorded (not dropped) so reports show what was waived (R6).
	Allowlisted bool `json:"allowlisted,omitempty"`
}

// Scanner is the pure contract every detector satisfies. Scan must return
// findings in a stable order for identical input.
type Scanner interface {
	Name() string
	Scan(files []TrackedFile) []Finding
}

// fingerprint is the SHA-256 of rule id + relative path + matched content. The
// path is included so the same secret in two files is two distinct waivable
// findings; the content is included so an edit to the match invalidates a prior
// allowlist entry.
func fingerprint(rule, path, matched string) string {
	h := sha256.New()
	h.Write([]byte(rule))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(matched))
	return hex.EncodeToString(h.Sum(nil))
}

// redact shows at most the first and last 4 characters of a candidate secret,
// masking the middle. Short candidates are fully masked (R1).
func redact(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "…" + s[len(s)-4:]
}
