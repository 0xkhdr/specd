package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func LoadEvals(path string) ([]EvidenceEnvelopeV1, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []EvidenceEnvelopeV1
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var record EvidenceEnvelopeV1
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("EVAL_STORE_MALFORMED: %w", err)
		}
		if err := ValidateEvidenceEnvelope(record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

// ImportEvalsToStore validates an adapter artifact and appends every accepted
// record to the store, refusing anything already present (R3.2 duplicate). It
// returns import findings (the artifact was rejected as a whole and nothing was
// written) or nil on success. Duplicate detection against the existing store is
// done up front so a partly-duplicate artifact never leaves a partial write.
func ImportEvalsToStore(path string, raw []byte, expect ImportExpect) ([]ImportFinding, error) {
	records, findings := ImportEvals(raw, expect)
	if len(findings) > 0 {
		return findings, nil
	}
	existing, err := LoadEvals(path)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, e := range existing {
		have[e.EvidenceID] = true
	}
	for i, r := range records {
		if have[r.EvidenceID] {
			findings = append(findings, ImportFinding{Index: i, EvidenceID: r.EvidenceID, Code: "EVAL_IMPORT_DUPLICATE", Message: "evidence_id already in store"})
		}
	}
	if len(findings) > 0 {
		return findings, nil
	}
	stored, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var next bytes.Buffer
	next.Write(stored)
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		next.Write(line)
		next.WriteByte('\n')
	}
	if len(records) > 0 {
		if err := AtomicWrite(path, next.String()); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func AppendEval(path string, record EvidenceEnvelopeV1) error {
	if err := ValidateEvidenceEnvelope(record); err != nil {
		return err
	}
	records, err := LoadEvals(path)
	if err != nil {
		return err
	}
	for _, existing := range records {
		if existing.EvidenceID == record.EvidenceID {
			return fmt.Errorf("EVAL_ID_DUPLICATE: %s", record.EvidenceID)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	line, _ := json.Marshal(record)
	return AtomicWrite(path, string(raw)+string(line)+"\n")
}
