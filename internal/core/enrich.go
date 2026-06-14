package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// EnrichVersion is recorded in enrich.json so freshness checks can tell which
// enrichment contract produced a given result. Bump on any change to the target
// set or marker scheme so older enrich.json files are flagged stale.
const EnrichVersion = "1.0.0"

// EnrichMarkerVersion versions the managed marker block written into steering
// files. It is independent of EnrichVersion.
const EnrichMarkerVersion = "v1"

// EnrichTargets are the steering files an AI agent may enrich. boot.md owns the
// deterministic "Detected stack" block in tech.md; enrichment fills the
// inference-heavy remainder. Keep this list sorted by key.
var EnrichTargets = map[string]string{
	"product":   "product.md",
	"structure": "structure.md",
	"tech":      "tech.md",
}

// EnrichTargetKeys returns the valid target keys, sorted.
func EnrichTargetKeys() []string {
	keys := make([]string, 0, len(EnrichTargets))
	for k := range EnrichTargets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func enrichMarkerBegin() string {
	return fmt.Sprintf("<!-- SPECD ENRICH: BEGIN %s (agent-authored; validated by `specd enrich status`) -->", EnrichMarkerVersion)
}

func enrichMarkerEnd() string {
	return fmt.Sprintf("<!-- SPECD ENRICH: END %s -->", EnrichMarkerVersion)
}

// EnrichTargetPath returns the absolute path of a target steering file. ok is
// false for an unknown target key.
func EnrichTargetPath(root, target string) (string, bool) {
	file, ok := EnrichTargets[target]
	if !ok {
		return "", false
	}
	return filepath.Join(SteeringDir(root), file), true
}

func enrichRecordPath(root string) string { return filepath.Join(SpecdDir(root), "enrich.json") }

// EnrichRecord is the .specd/enrich.json sidecar: the durable record of which
// targets an agent enriched, and the boot state they were authored against. It
// mirrors boot.json's role for the boot-freshness gate.
type EnrichRecord struct {
	Targets       []string `json:"targets"`
	BootDetector  string   `json:"bootDetectorVersion"`
	BootHash      string   `json:"bootHash"`
	Sources       []string `json:"sources"`
	GeneratedAt   string   `json:"generatedAt"`
	EnrichVersion string   `json:"enrichVersion"`
}

// LoadEnrichRecord reads .specd/enrich.json. ok is false when it is absent.
func LoadEnrichRecord(root string) (EnrichRecord, bool) {
	raw := ReadOrNull(enrichRecordPath(root))
	if raw == nil {
		return EnrichRecord{}, false
	}
	var rec EnrichRecord
	if err := json.Unmarshal([]byte(*raw), &rec); err != nil {
		return EnrichRecord{}, false
	}
	return rec, true
}

func saveEnrichRecord(root string, rec EnrichRecord) error {
	sort.Strings(rec.Targets)
	sort.Strings(rec.Sources)
	b, _ := json.MarshalIndent(rec, "", "  ")
	return AtomicWrite(enrichRecordPath(root), string(b)+"\n")
}

// bootHash returns a stable fingerprint of the stored boot.json (timestamp
// excluded) so enrichment can detect when boot detection has drifted underneath
// it. Empty string when boot.json is absent or invalid.
func bootHash(root string) string {
	raw := ReadOrNull(filepath.Join(SpecdDir(root), "boot.json"))
	if raw == nil {
		return ""
	}
	var res BootResult
	if err := json.Unmarshal([]byte(*raw), &res); err != nil {
		return ""
	}
	res.GeneratedAt = ""
	canon, _ := json.Marshal(res)
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:])
}

// ApplyEnrichSection writes agent-authored markdown into the managed ENRICH
// block of a target steering file (idempotent via MergeSection) and updates the
// enrich.json record to mark the target enriched against the current boot state.
// It requires a present, parseable boot.json.
func ApplyEnrichSection(root, target, body string) error {
	path, ok := EnrichTargetPath(root, target)
	if !ok {
		return UsageError(fmt.Sprintf("unknown enrich target %q (valid: %s)", target, strings.Join(EnrichTargetKeys(), ", ")))
	}
	hash := bootHash(root)
	if hash == "" {
		return NotFoundError("no valid .specd/boot.json — run `specd boot` before enriching.")
	}
	body = strings.TrimRight(body, "\n")
	if strings.TrimSpace(body) == "" {
		return UsageError("refusing to apply empty enrichment content for target " + target)
	}
	if err := MergeSection(path, enrichMarkerBegin(), enrichMarkerEnd(), body); err != nil {
		return GateError(fmt.Sprintf("merge %s: %v", path, err))
	}

	rec, _ := LoadEnrichRecord(root)
	if !contains(rec.Targets, target) {
		rec.Targets = append(rec.Targets, target)
	}
	rec.BootDetector = BootDetectorVersion
	rec.BootHash = hash
	rec.Sources = enrichSources(root)
	rec.GeneratedAt = NowISO()
	rec.EnrichVersion = EnrichVersion
	return saveEnrichRecord(root, rec)
}

// EnrichFreshness is the verdict of the enrich-freshness gate.
type EnrichFreshness struct {
	Stale  bool     `json:"stale"`
	Issues []string `json:"issues"`
}

// CheckEnrichFreshness validates that the enrichment still reflects the repo:
// every claimed target still carries its ENRICH block, no recorded evidence
// source has vanished, the boot detector/hash has not drifted, and every target
// has actually been enriched. Returns a NotFoundError when enrich.json is absent.
func CheckEnrichFreshness(root string) (EnrichFreshness, error) {
	rec, ok := LoadEnrichRecord(root)
	if !ok {
		return EnrichFreshness{}, NotFoundError("no .specd/enrich.json — run `specd enrich` first.")
	}

	issues := []string{}

	hash := bootHash(root)
	if hash == "" {
		issues = append(issues, "boot.json missing or invalid — run `specd boot` then re-enrich")
	} else if rec.BootHash != "" && rec.BootHash != hash {
		issues = append(issues, "boot detection drifted since enrichment — review and re-run `specd enrich`")
	}
	if rec.BootDetector != BootDetectorVersion {
		issues = append(issues, fmt.Sprintf("detector version drift: enrich.json=%s, current=%s — re-run `specd enrich`", rec.BootDetector, BootDetectorVersion))
	}

	begin := enrichMarkerBegin()
	for _, target := range EnrichTargetKeys() {
		path, _ := EnrichTargetPath(root, target)
		has := false
		if raw := ReadOrNull(path); raw != nil {
			has = strings.Contains(*raw, begin)
		}
		if !has {
			if contains(rec.Targets, target) {
				issues = append(issues, fmt.Sprintf("%s: recorded as enriched but ENRICH block is missing — re-run `specd enrich`", EnrichTargets[target]))
			} else {
				issues = append(issues, fmt.Sprintf("%s: not yet enriched — run `specd enrich plan` for the brief", EnrichTargets[target]))
			}
		}
	}

	for _, s := range rec.Sources {
		if !FileExists(filepath.Join(root, s)) {
			issues = append(issues, "recorded evidence source no longer exists: "+s)
		}
	}

	return EnrichFreshness{Stale: len(issues) > 0, Issues: issues}, nil
}
