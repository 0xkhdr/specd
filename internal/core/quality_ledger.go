package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// QualityLedgerEntry is a redacted, append-only quality learning record.
// Provenance is immutable: promotion points at prior evidence, never copied data.
type QualityLedgerEntry struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Taxonomy     string `json:"taxonomy"`
	EvidenceRef  string `json:"evidence_ref"`
	SourceDigest string `json:"source_digest"`
	CreatedAt    string `json:"created_at"`
}

func ValidateQualityLedgerEntry(e QualityLedgerEntry) error {
	if e.ID == "" || e.Taxonomy == "" || e.EvidenceRef == "" || e.SourceDigest == "" || e.CreatedAt == "" {
		return errors.New("QUALITY_LEDGER_REQUIRED_FIELD")
	}
	if e.Kind != "failure" && e.Kind != "promotion" {
		return fmt.Errorf("QUALITY_LEDGER_KIND_UNKNOWN: %q", e.Kind)
	}
	if err := validateEvidenceRef(e.EvidenceRef); err != nil {
		return err
	}
	for _, value := range []string{e.ID, e.Taxonomy, e.EvidenceRef, e.CreatedAt} {
		lower := strings.ToLower(value)
		if strings.ContainsAny(value, "\r\n") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "token") {
			return errors.New("QUALITY_LEDGER_REDACTION_REQUIRED")
		}
	}
	if len(e.SourceDigest) != 64 {
		return errors.New("QUALITY_LEDGER_SOURCE_DIGEST_INVALID")
	}
	return nil
}

func AppendQualityLedger(path string, entry QualityLedgerEntry) error {
	if err := ValidateQualityLedgerEntry(entry); err != nil {
		return err
	}
	entries, err := LoadQualityLedger(path)
	if err != nil {
		return err
	}
	for _, old := range entries {
		if old.ID == entry.ID {
			return fmt.Errorf("QUALITY_LEDGER_DUPLICATE: %s", entry.ID)
		}
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return AppendFile(path, string(raw)+"\n")
}

func LoadQualityLedger(path string) ([]QualityLedgerEntry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []QualityLedgerEntry
	seen := map[string]bool{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var entry QualityLedgerEntry
		if err := json.Unmarshal(s.Bytes(), &entry); err != nil {
			return nil, err
		}
		if err := ValidateQualityLedgerEntry(entry); err != nil {
			return nil, err
		}
		if seen[entry.ID] {
			return nil, fmt.Errorf("QUALITY_LEDGER_DUPLICATE: %s", entry.ID)
		}
		seen[entry.ID] = true
		out = append(out, entry)
	}
	return out, s.Err()
}
