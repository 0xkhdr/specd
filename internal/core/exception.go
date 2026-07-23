package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ExceptionV1 is a time-bound governed deviation. Like decisions, records are
// immutable; revocation and supersession are later records, never deletion.
type ExceptionV1 struct {
	ID                 string           `json:"id"`
	Status             GovernanceStatus `json:"status"`
	Owner              string           `json:"owner"`
	CreatedAt          string           `json:"created_at"`
	ReviewAt           string           `json:"review_at"`
	ExpiresAt          string           `json:"expires_at"`
	Supersedes         string           `json:"supersedes,omitempty"`
	AffectedInvariants []string         `json:"affected_invariants,omitempty"`
	Blocking           bool             `json:"blocking,omitempty"`
}

func (e ExceptionV1) Validate() error {
	return DecisionV1{ID: e.ID, Status: e.Status, Owner: e.Owner, CreatedAt: e.CreatedAt, ReviewAt: e.ReviewAt, ExpiresAt: e.ExpiresAt, Supersedes: e.Supersedes, AffectedInvariants: e.AffectedInvariants}.Validate()
}

func ExceptionPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "exceptions.json")
}

func LoadGovernanceExceptions(path string) ([]ExceptionV1, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []ExceptionV1
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("decode exceptions: %w", err)
	}
	seen := map[string]bool{}
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return nil, err
		}
		if seen[record.ID] {
			return nil, fmt.Errorf("exception id %q already exists", record.ID)
		}
		seen[record.ID] = true
	}
	return records, nil
}
