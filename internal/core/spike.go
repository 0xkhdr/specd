package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const spikeRecordPrefix = "spike:"

// Spike is a bounded exploratory-learning record (spec 01 R7.3). It captures
// what a prototype learned WITHOUT authorizing anything: a spike never completes
// a task and never approves architecture. Those transitions demand a passing
// verify record (CompleteTask) and a human design approval respectively, and a
// spike is a distinct record kind that neither path reads — so it cannot be a
// bypass. The bound (Question + Scope + a future Expiry) is what keeps a spike
// from silently becoming an open-ended parallel track that substitutes for real
// intent: an unbounded "spike" is just unreviewed work.
type Spike struct {
	Question  string `json:"question"`
	Scope     string `json:"scope"`
	Expiry    string `json:"expiry"`
	OutputRef string `json:"output_ref,omitempty"`
	Timestamp string `json:"timestamp"`
	GitHead   string `json:"git_head"`
	Actor     string `json:"actor"`
}

// StampSpike fills the provenance triple, mirroring StampRecord/StampAmendment.
func StampSpike(s Spike, gitHead string) Spike {
	s.Timestamp = Clock().Format(time.RFC3339)
	s.GitHead = gitHead
	s.Actor = recordActor()
	return s
}

// Validate enforces the R7.3 bound: question, scope, and a parseable expiry are
// all required, and the expiry must fall strictly after the record's own
// timestamp so the exploration window is finite. It deliberately does NOT
// compare against wall-clock now — an already-elapsed spike is expired, not
// invalid, and must still decode so history stays replayable (see Expired).
func (s Spike) Validate() error {
	if strings.TrimSpace(s.Question) == "" {
		return errors.New("spike question is required")
	}
	if strings.TrimSpace(s.Scope) == "" {
		return errors.New("spike scope is required")
	}
	if strings.TrimSpace(s.Expiry) == "" {
		return errors.New("spike expiry is required")
	}
	expiry, err := time.Parse(time.RFC3339, s.Expiry)
	if err != nil {
		return fmt.Errorf("spike expiry %q is not an RFC3339 time: %w", s.Expiry, err)
	}
	if s.Timestamp != "" {
		recorded, err := time.Parse(time.RFC3339, s.Timestamp)
		if err != nil {
			return fmt.Errorf("spike timestamp %q is not an RFC3339 time: %w", s.Timestamp, err)
		}
		if !expiry.After(recorded) {
			return fmt.Errorf("spike expiry %q must be after recorded time %q: a spike must be bounded", s.Expiry, s.Timestamp)
		}
	}
	return nil
}

// Expired reports whether the spike's exploration window has closed as of now.
// A malformed expiry is treated as expired (fail-closed): a spike whose bound
// cannot be read cannot be trusted as live. Reports use this to surface stale
// spikes; it never invalidates the stored record.
func (s Spike) Expired(now time.Time) bool {
	expiry, err := time.Parse(time.RFC3339, s.Expiry)
	if err != nil {
		return true
	}
	return !now.Before(expiry)
}

// AppendSpike stores a validated spike under a never-reused sequential key,
// mirroring AppendAmendment. It touches no lifecycle status, task status, or
// approval record — recording a spike authorizes nothing.
func (s *State) AppendSpike(spike Spike) error {
	if err := spike.Validate(); err != nil {
		return err
	}
	if s.Records == nil {
		s.Records = map[string]json.RawMessage{}
	}
	key := fmt.Sprintf("%s%d", spikeRecordPrefix, s.countSpikeKeys())
	for {
		if _, exists := s.Records[key]; !exists {
			break
		}
		key = fmt.Sprintf("%s%d", spikeRecordPrefix, s.countSpikeKeys()+1)
	}
	raw, err := json.Marshal(spike)
	if err != nil {
		return err
	}
	s.Records[key] = raw
	return nil
}

// Spikes returns the recorded spikes in stable key order.
func (s State) Spikes() ([]Spike, error) {
	keys := s.spikeKeys()
	sort.Strings(keys)
	out := make([]Spike, 0, len(keys))
	for _, key := range keys {
		var spike Spike
		if err := json.Unmarshal(s.Records[key], &spike); err != nil {
			return nil, fmt.Errorf("decode %s: %w", key, err)
		}
		if err := spike.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s: %w", key, err)
		}
		out = append(out, spike)
	}
	return out, nil
}

func (s State) spikeKeys() []string {
	keys := make([]string, 0)
	for key := range s.Records {
		if strings.HasPrefix(key, spikeRecordPrefix) {
			keys = append(keys, key)
		}
	}
	return keys
}

func (s State) countSpikeKeys() int { return len(s.spikeKeys()) }
