package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const RegressionSchemaV1 = "security-regression/v1"

type RegressionExpectedV1 struct {
	Scanner string `json:"scanner"`
	Rule    string `json:"rule"`
}

type RegressionInputV1 struct {
	Path    string   `json:"path"`
	Kind    ScanKind `json:"kind"`
	Trust   string   `json:"trust"`
	Content string   `json:"content"`
}

type RegressionIncidentV1 struct {
	ID         string               `json:"id"`
	Provenance string               `json:"provenance"`
	Input      RegressionInputV1    `json:"input"`
	Expected   RegressionExpectedV1 `json:"expected"`
}

type RegressionCorpusV1 struct {
	SchemaVersion string                 `json:"schema_version"`
	PolicyDigest  string                 `json:"policy_digest"`
	Incidents     []RegressionIncidentV1 `json:"incidents"`
}

type RegressionTrend struct {
	Rule  string `json:"rule"`
	Count int    `json:"count"`
}

// LoadIncidentCorpus validates promoted incident attestations offline. Policy
// digest mismatch makes the entire corpus stale; partial reuse is forbidden.
func LoadIncidentCorpus(path, policyDigest string) (RegressionCorpusV1, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RegressionCorpusV1{}, err
	}
	var corpus RegressionCorpusV1
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&corpus); err != nil {
		return RegressionCorpusV1{}, fmt.Errorf("invalid security regression corpus: %w", err)
	}
	if corpus.SchemaVersion != RegressionSchemaV1 {
		return RegressionCorpusV1{}, fmt.Errorf("unsupported security regression schema %q", corpus.SchemaVersion)
	}
	if corpus.PolicyDigest == "" || corpus.PolicyDigest != policyDigest {
		return RegressionCorpusV1{}, fmt.Errorf("stale policy attestation: corpus %q, current %q", corpus.PolicyDigest, policyDigest)
	}
	if len(corpus.Incidents) == 0 {
		return RegressionCorpusV1{}, fmt.Errorf("security regression corpus has no incidents")
	}
	seen := map[string]bool{}
	for _, incident := range corpus.Incidents {
		if incident.ID == "" || seen[incident.ID] {
			return RegressionCorpusV1{}, fmt.Errorf("incident id is empty or duplicate: %q", incident.ID)
		}
		seen[incident.ID] = true
		if !strings.HasPrefix(incident.Provenance, "redacted:") || looksSensitive(incident.Provenance) {
			return RegressionCorpusV1{}, fmt.Errorf("incident %q provenance is not redacted", incident.ID)
		}
		if incident.Input.Trust != TrustUntrustedData {
			return RegressionCorpusV1{}, fmt.Errorf("incident %q input must be untrusted_data", incident.ID)
		}
		if incident.Expected.Scanner == "" || incident.Expected.Rule == "" || !incidentMatches(incident) {
			return RegressionCorpusV1{}, fmt.Errorf("incident %q expected finding not reproduced", incident.ID)
		}
	}
	return corpus, nil
}

func looksSensitive(s string) bool {
	lower := strings.ToLower(s)
	for _, marker := range []string{"token=", "password=", "secret=", "ghp_", "akia"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func incidentMatches(incident RegressionIncidentV1) bool {
	if incident.Expected.Scanner != "dangerous" {
		return false
	}
	if finding, ok := secretFileFinding(filepath.ToSlash(incident.Input.Path)); ok && finding.Rule == incident.Expected.Rule {
		return true
	}
	lower := strings.ToLower(incident.Input.Content)
	for _, pattern := range dangerPatterns {
		if strings.Contains(lower, pattern.needle) && incident.Expected.Rule == "destructive-shell" {
			return true
		}
	}
	return false
}

func (c RegressionCorpusV1) Trend() []RegressionTrend {
	counts := map[string]int{}
	for _, incident := range c.Incidents {
		counts[incident.Expected.Scanner+"/"+incident.Expected.Rule]++
	}
	out := make([]RegressionTrend, 0, len(counts))
	for rule, count := range counts {
		out = append(out, RegressionTrend{Rule: rule, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rule < out[j].Rule })
	return out
}
