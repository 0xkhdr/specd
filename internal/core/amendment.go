package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

const amendmentRecordPrefix = "amendment:"

// Amendment is an append-only change-impact record. IDs are stable contract
// addresses (requirements, design units, or tasks); digests pin the source
// bytes before and after the change.
type Amendment struct {
	ChangeID         string            `json:"change_id"`
	AffectedIDs      []string          `json:"affected_ids"`
	Rationale        string            `json:"rationale"`
	BeforeDigests    map[string]string `json:"before_digests,omitempty"`
	AfterDigests     map[string]string `json:"after_digests,omitempty"`
	RequiredRechecks []string          `json:"required_rechecks"`
	RecordedRevision int64             `json:"recorded_revision,omitempty"`
	Timestamp        string            `json:"timestamp"`
	GitHead          string            `json:"git_head"`
	Actor            string            `json:"actor"`
}

func StampAmendment(a Amendment, gitHead string) Amendment {
	a.Timestamp = Clock().Format(time.RFC3339)
	a.GitHead = gitHead
	a.Actor = recordActor()
	return a
}

func (a Amendment) Validate() error {
	if a.ChangeID == "" {
		return errors.New("amendment change_id is required")
	}
	if a.Rationale == "" {
		return errors.New("amendment rationale is required")
	}
	if len(a.AffectedIDs) == 0 {
		return errors.New("amendment affected_ids is required")
	}
	seen := map[string]bool{}
	for _, id := range a.AffectedIDs {
		if id == "" || seen[id] {
			return fmt.Errorf("amendment affected_ids contains invalid or duplicate id %q", id)
		}
		seen[id] = true
	}
	if len(a.RequiredRechecks) == 0 {
		return errors.New("amendment required_rechecks is required")
	}
	return nil
}

func (s *State) AppendAmendment(a Amendment) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if s.Records == nil {
		s.Records = map[string]json.RawMessage{}
	}
	if a.RecordedRevision == 0 {
		a.RecordedRevision = s.Revision
	}
	keys := make([]string, 0)
	for key := range s.Records {
		if len(key) >= len(amendmentRecordPrefix) && key[:len(amendmentRecordPrefix)] == amendmentRecordPrefix {
			keys = append(keys, key)
		}
	}
	// Sequence is derived from existing entries and never reuses a key.
	sort.Strings(keys)
	key := fmt.Sprintf("%s%d", amendmentRecordPrefix, len(keys))
	for {
		if _, exists := s.Records[key]; !exists {
			break
		}
		key = fmt.Sprintf("%s%d", amendmentRecordPrefix, len(keys)+1)
		keys = append(keys, key)
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	s.Records[key] = raw
	return nil
}

func (s State) Amendments() ([]Amendment, error) {
	keys := make([]string, 0)
	for key := range s.Records {
		if len(key) >= len(amendmentRecordPrefix) && key[:len(amendmentRecordPrefix)] == amendmentRecordPrefix {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := make([]Amendment, 0, len(keys))
	for _, key := range keys {
		var amendment Amendment
		if err := json.Unmarshal(s.Records[key], &amendment); err != nil {
			return nil, fmt.Errorf("decode %s: %w", key, err)
		}
		if err := amendment.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s: %w", key, err)
		}
		out = append(out, amendment)
	}
	return out, nil
}
