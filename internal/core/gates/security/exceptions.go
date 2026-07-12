package security

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Exception is one immutable, governed suppression record. Revocation is a new
// record with Action="revoke" and the same Finding; prior records remain audit
// history in the append-only JSONL ledger.
type Exception struct {
	Finding             string `json:"finding"`
	Action              string `json:"action"`
	Reason              string `json:"reason"`
	Ticket              string `json:"ticket"`
	Owner               string `json:"owner"`
	Scope               string `json:"scope"`
	Revision            string `json:"revision"`
	Environment         string `json:"environment"`
	IssuedAt            string `json:"issued_at"`
	ExpiresAt           string `json:"expires_at"`
	CompensatingControl string `json:"compensating_control"`
	Approver            string `json:"approver"`
}

type ExceptionSet struct {
	active  map[string]struct{}
	Records []Exception
	Digest  string
}

func (s ExceptionSet) Allows(finding string) bool { _, ok := s.active[finding]; return ok }

func ValidateException(e Exception) error {
	fields := map[string]string{"finding": e.Finding, "action": e.Action, "reason": e.Reason, "ticket": e.Ticket, "owner": e.Owner, "scope": e.Scope, "revision": e.Revision, "environment": e.Environment, "issued_at": e.IssuedAt, "expires_at": e.ExpiresAt, "compensating_control": e.CompensatingControl, "approver": e.Approver}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("exception missing %s", name)
		}
	}
	if e.Action != "suppress" && e.Action != "revoke" {
		return fmt.Errorf("exception action must be suppress or revoke")
	}
	issued, err := time.Parse(time.RFC3339, e.IssuedAt)
	if err != nil {
		return fmt.Errorf("exception issued_at: %w", err)
	}
	expires, err := time.Parse(time.RFC3339, e.ExpiresAt)
	if err != nil {
		return fmt.Errorf("exception expires_at: %w", err)
	}
	if !expires.After(issued) {
		return fmt.Errorf("exception expiry must follow issue")
	}
	return nil
}

func ExceptionDigest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func LoadExceptions(root, revision, environment string, now time.Time) (ExceptionSet, []Finding) {
	path := filepath.Join(root, ".specd", "security", "exceptions.jsonl")
	raw, err := os.ReadFile(path)
	set := ExceptionSet{active: map[string]struct{}{}}
	if os.IsNotExist(err) {
		return set, nil
	}
	if err != nil {
		return set, []Finding{{Scanner: "exceptions", Rule: "load", Severity: "error", Excerpt: err.Error()}}
	}
	set.Digest = ExceptionDigest(raw)
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for line := 1; scanner.Scan(); line++ {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var e Exception
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return ExceptionSet{active: map[string]struct{}{}}, []Finding{{Scanner: "exceptions", Rule: "parse", Severity: "error", Excerpt: fmt.Sprintf("line %d: %v", line, err)}}
		}
		if err := ValidateException(e); err != nil {
			return ExceptionSet{active: map[string]struct{}{}}, []Finding{{Scanner: "exceptions", Rule: "schema", Severity: "error", Excerpt: fmt.Sprintf("line %d: %v", line, err)}}
		}
		set.Records = append(set.Records, e)
		expires, _ := time.Parse(time.RFC3339, e.ExpiresAt)
		if e.Revision != revision || e.Environment != environment || !now.Before(expires) {
			continue
		}
		if e.Action == "revoke" {
			delete(set.active, e.Finding)
		} else {
			set.active[e.Finding] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return ExceptionSet{active: map[string]struct{}{}}, []Finding{{Scanner: "exceptions", Rule: "read", Severity: "error", Excerpt: err.Error()}}
	}
	return set, nil
}

func AppendException(root string, e Exception) error {
	if err := ValidateException(e); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".specd", "security", "exceptions.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
