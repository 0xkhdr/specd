package security

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DepEvidenceSchema pins the version of the offline adapter artifact this gate
// accepts. External vulnerability/provenance facts arrive as a pinned artifact
// produced out of band (scripts/dep-evidence.sh); the gate itself never touches
// the network — it validates and projects the pinned bytes (R6.2).
const DepEvidenceSchema = "dep-evidence/v1"

// DepEvidenceV1 is the offline dependency-evidence artifact. ManifestDigest ties
// the artifact to the exact manifests it was produced for: a mismatch means the
// evidence is stale and fails closed.
type DepEvidenceV1 struct {
	Schema         string            `json:"schema"`
	GeneratedAt    string            `json:"generated_at"`
	ManifestDigest string            `json:"manifest_digest"`
	Findings       []DepEvidenceItem `json:"findings"`
}

// DepEvidenceItem is one advisory/provenance fact from the pinned artifact.
type DepEvidenceItem struct {
	Module   string `json:"module"`
	Severity string `json:"severity"`
	Advisory string `json:"advisory"`
	Summary  string `json:"summary"`
}

// ScanDepEvidence validates a pinned offline artifact against the current
// manifest digest and projects its advisories to findings. Malformed, wrong
// schema, and stale (digest mismatch) all fail closed at error severity — the
// gate never trusts unvalidated or outdated supply-chain evidence.
func ScanDepEvidence(artifact []byte, manifestDigest string) []Finding {
	var ev DepEvidenceV1
	if len(artifact) == 0 || json.Unmarshal(artifact, &ev) != nil || ev.Schema != DepEvidenceSchema {
		return []Finding{evidenceError("evidence-malformed", "dependency evidence artifact is malformed or of an unknown schema")}
	}
	if ev.ManifestDigest != manifestDigest {
		return []Finding{evidenceError("evidence-stale", "dependency evidence does not match the current manifest digest")}
	}
	var findings []Finding
	for _, item := range ev.Findings {
		sev := item.Severity
		if sev != "error" && sev != "warn" && sev != "off" {
			sev = "error" // unknown severity fails closed
		}
		findings = append(findings, Finding{
			Scanner:     "depevidence",
			Rule:        "advisory",
			File:        "go.mod",
			Severity:    sev,
			Fingerprint: fingerprint("advisory", item.Module, item.Advisory),
			Excerpt:     item.Module + " " + item.Advisory,
		})
	}
	return findings
}

func evidenceError(rule, msg string) Finding {
	return Finding{Scanner: "depevidence", Rule: rule, File: "go.mod", Severity: "error", Fingerprint: fingerprint(rule, "go.mod", msg), Excerpt: msg}
}

// ManifestDigest is the sha256 over the concatenated manifest bytes (go.mod then
// go.sum, each skipped when absent). It matches `cat go.mod go.sum | sha256sum`
// so scripts/dep-evidence.sh and this gate pin the same manifest state.
func ManifestDigest(root string) (string, error) {
	var buf []byte
	for _, name := range []string{"go.mod", "go.sum"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		buf = append(buf, b...)
	}
	return digest(buf), nil
}
