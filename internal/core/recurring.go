package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const RecurringSchemaV1 = 1

var recurringID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Recurring definitions are untrusted commands executed only by external CI.
// This allowlist constrains contract shape; it is not a sandbox guarantee.
var recurringExecutable = map[string]bool{"go": true, "specd": true}

type RecurringCheckV1 struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Command       string `json:"command"`
	Cadence       string `json:"cadence,omitempty"`
	Trigger       string `json:"trigger,omitempty"`
}

func (c RecurringCheckV1) Validate() error {
	if c.SchemaVersion != 0 && c.SchemaVersion != RecurringSchemaV1 {
		return fmt.Errorf("unsupported recurring check schema_version %d", c.SchemaVersion)
	}
	if !recurringID.MatchString(c.ID) {
		return fmt.Errorf("invalid recurring check id %q", c.ID)
	}
	command := strings.TrimSpace(c.Command)
	if command == "" || len(command) > 4096 || strings.ContainsAny(command, "\x00\r\n") {
		return errors.New("recurring command must be a bounded single line")
	}
	fields := strings.Fields(command)
	first := fields[0]
	if strings.ContainsAny(command, ";&|`$<>") || strings.Contains(first, "/") || !recurringExecutable[first] {
		return fmt.Errorf("recurring executable %q is not in definition allowlist", first)
	}
	if len(fields) < 2 || (first == "go" && fields[1] != "test") || (first == "specd" && fields[1] != "check" && fields[1] != "status" && fields[1] != "drift" && fields[1] != "report") {
		return fmt.Errorf("recurring command %q is not an allowed read-only invocation", command)
	}
	if strings.TrimSpace(c.Cadence) == "" && strings.TrimSpace(c.Trigger) == "" {
		return errors.New("recurring check requires cadence or trigger metadata")
	}
	if len(c.Cadence) > 128 || len(c.Trigger) > 128 || strings.ContainsAny(c.Cadence+c.Trigger, "\x00\r\n") {
		return errors.New("recurring schedule metadata is invalid")
	}
	return nil
}

type RecurringVerdict string

const (
	RecurringPass RecurringVerdict = "pass"
	RecurringFail RecurringVerdict = "fail"
)

type RecurringResultV1 struct {
	SchemaVersion int              `json:"schema_version"`
	CheckID       string           `json:"check_id"`
	GitHead       string           `json:"git_head"`
	ReleaseID     string           `json:"release_id"`
	ConfigID      string           `json:"config_id"`
	Verdict       RecurringVerdict `json:"verdict"`
	ObservedAt    string           `json:"observed_at"`
}

func (r RecurringResultV1) Validate() error {
	if r.SchemaVersion != 0 && r.SchemaVersion != RecurringSchemaV1 {
		return fmt.Errorf("unsupported recurring result schema_version %d", r.SchemaVersion)
	}
	if !recurringID.MatchString(r.CheckID) {
		return fmt.Errorf("invalid recurring check id %q", r.CheckID)
	}
	if !HeadPinned(r.GitHead) {
		return errors.New("recurring result requires a resolvable git HEAD")
	}
	if !recurringID.MatchString(r.ReleaseID) || !recurringID.MatchString(r.ConfigID) {
		return errors.New("recurring result requires bounded release and config identities")
	}
	if r.Verdict != RecurringPass && r.Verdict != RecurringFail {
		return fmt.Errorf("invalid recurring verdict %q", r.Verdict)
	}
	if _, err := time.Parse(time.RFC3339, r.ObservedAt); err != nil {
		return fmt.Errorf("invalid observed_at: %w", err)
	}
	return nil
}

func RecurringResultsPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "recurring-results.jsonl")
}

func RecordRecurringResult(root, slug string, result RecurringResultV1) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if err := result.Validate(); err != nil {
		return err
	}
	if err := ResolveGitCommit(root, result.GitHead); err != nil {
		return fmt.Errorf("recurring result git_head: %w", err)
	}
	result.SchemaVersion = RecurringSchemaV1
	line, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, AppendFile(RecurringResultsPath(root, slug), string(line)+"\n")
	})
	return err
}

func ResolveGitCommit(root, revision string) error {
	if !HeadPinned(revision) {
		return errors.New("commit is not pinned")
	}
	cmd := exec.Command("git", "-C", root, "cat-file", "-e", revision+"^{commit}")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot resolve commit %q", revision)
	}
	return nil
}

func LoadRecurringResults(path string) ([]RecurringResultV1, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []RecurringResultV1
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		var r RecurringResultV1
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("decode recurring result: %w", err)
		}
		if err := r.Validate(); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, scanner.Err()
}

type RecurringSuccessorPlan struct {
	Provenance ProvenanceV1 `json:"provenance"`
	Link       ProgramLink  `json:"link"`
}

func PlanRecurringSuccessor(source, successor string, result RecurringResultV1, enabled bool) (*RecurringSuccessorPlan, error) {
	if !enabled {
		return nil, nil
	}
	if err := ValidateSlug(source); err != nil {
		return nil, err
	}
	if err := ValidateSlug(successor); err != nil {
		return nil, err
	}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	if result.Verdict != RecurringFail {
		return nil, errors.New("successor may only be planned for a failing recurring result")
	}
	ref := fmt.Sprintf("recurring:%s@%s", result.CheckID, result.GitHead)
	return &RecurringSuccessorPlan{Provenance: ProvenanceV1{SchemaVersion: ProvenanceSchemaV1, SourceType: SourcePolicy, SourceRef: ref, AffectedSpecs: []string{source}, PriorLinks: []ProvenanceLink{{From: successor, To: source, Kind: LinkKindMaintains, Reason: "recurring invariant failure"}}}, Link: ProgramLink{From: successor, To: source, Kind: LinkKindMaintains, Reason: "recurring invariant failure"}}, nil
}
