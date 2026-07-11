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
