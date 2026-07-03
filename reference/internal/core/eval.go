package core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/runner"
)

const EvalSchemaVersion = 1

type EvalRubric struct {
	SchemaVersion int         `json:"schemaVersion,omitempty"`
	Suite         string      `json:"suite,omitempty"`
	MinScore      float64     `json:"minScore,omitempty"`
	Checks        []EvalCheck `json:"checks"`
}

type EvalCheck struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Weight     float64 `json:"weight,omitempty"`
	Path       string  `json:"path,omitempty"`
	Pattern    string  `json:"pattern,omitempty"`
	Command    string  `json:"command,omitempty"`
	TimeoutMs  int     `json:"timeoutMs,omitempty"`
	Predicate  string  `json:"predicate,omitempty"`
	Max        int     `json:"max,omitempty"`
	Min        int     `json:"min,omitempty"`
	Required   bool    `json:"required,omitempty"`
	WorkingDir string  `json:"workingDir,omitempty"`
	// Sandbox selects the isolation backend for command checks: "none"
	// (default, host shell), "bwrap", or "container". Reuses the shared verify
	// runner so eval command checks inherit the same env-scrub and fail-closed
	// isolation as verify (invariant 8: single shared sandboxed exec path).
	Sandbox string `json:"sandbox,omitempty"`
}

type EvalReport struct {
	SchemaVersion int               `json:"schemaVersion"`
	Suite         string            `json:"suite"`
	Score         float64           `json:"score"`
	MinScore      float64           `json:"minScore"`
	Passed        bool              `json:"passed"`
	RubricDigest  string            `json:"rubricDigest"`
	GeneratedAt   string            `json:"generatedAt"`
	Checks        []EvalCheckResult `json:"checks"`
	Failures      []string          `json:"failures,omitempty"`
	ReportPath    string            `json:"reportPath,omitempty"`
}

type EvalCheckResult struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Weight     float64 `json:"weight"`
	Score      float64 `json:"score"`
	Passed     bool    `json:"passed"`
	Message    string  `json:"message,omitempty"`
	Digest     string  `json:"digest,omitempty"`
	DurationMs int64   `json:"durationMs,omitempty"`
}

// Prototype lifecycle statuses. A prototype spec is created `pending`, skips
// the design/tasks planning gates, and can never reach `complete`; `promote`
// transitions it to `promoted`, after which the normal ratchet applies.
const (
	PrototypePending  = "pending"
	PrototypePromoted = "promoted"
)

type PrototypeState struct {
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt,omitempty"`
	PromotedAt    string `json:"promotedAt,omitempty"`
	EvalReport    string `json:"evalReport,omitempty"`
	EvalDigest    string `json:"evalDigest,omitempty"`
	Evidence      string `json:"evidence,omitempty"`
	PromotedScore string `json:"promotedScore,omitempty"`
}

func DefaultEvalRubricPath(root, slug string) string {
	return ArtifactPath(root, slug, "eval-rubric.json")
}

func LoadEvalRubric(path string) (*EvalRubric, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, "", GateError("eval rubric contains NUL byte: " + path)
	}
	sum := sha256.Sum256(data)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	var rubric EvalRubric
	if err := json.Unmarshal(data, &rubric); err != nil {
		return nil, "", GateError(fmt.Sprintf("invalid eval rubric %s: %v", path, err))
	}
	if err := ValidateEvalRubric(&rubric); err != nil {
		return nil, "", err
	}
	return &rubric, digest, nil
}

func ValidateEvalRubric(r *EvalRubric) error {
	if r.SchemaVersion != 0 && r.SchemaVersion != EvalSchemaVersion {
		return GateError(fmt.Sprintf("eval rubric schemaVersion %d unsupported", r.SchemaVersion))
	}
	if r.Suite == "" {
		r.Suite = "default"
	}
	if !validEvalID(r.Suite) {
		return GateError("eval suite must match ^[a-z0-9][a-z0-9-]*$")
	}
	if r.MinScore < 0 || r.MinScore > 1 {
		return GateError("eval minScore must be between 0 and 1")
	}
	if len(r.Checks) == 0 {
		return GateError("eval rubric must contain at least one check")
	}
	seen := map[string]bool{}
	for i := range r.Checks {
		c := &r.Checks[i]
		if !validEvalID(c.ID) {
			return GateError(fmt.Sprintf("eval check %d id must match ^[a-z0-9][a-z0-9-]*$", i+1))
		}
		if seen[c.ID] {
			return GateError(fmt.Sprintf("duplicate eval check id %q", c.ID))
		}
		seen[c.ID] = true
		if c.Weight < 0 {
			return GateError(fmt.Sprintf("eval check %q weight must be non-negative", c.ID))
		}
		if c.Weight == 0 {
			c.Weight = 1
		}
		if hasNUL(c.ID, c.Kind, c.Path, c.Pattern, c.Command, c.Predicate, c.WorkingDir) {
			return GateError(fmt.Sprintf("eval check %q contains NUL byte", c.ID))
		}
		switch c.Kind {
		case "regex":
			if c.Path == "" || c.Pattern == "" {
				return GateError(fmt.Sprintf("eval check %q regex requires path and pattern", c.ID))
			}
			if _, err := regexp.Compile(c.Pattern); err != nil {
				return GateError(fmt.Sprintf("eval check %q invalid regex: %v", c.ID, err))
			}
			if err := validateRelPath(c.Path); err != nil {
				return GateError(fmt.Sprintf("eval check %q invalid path: %v", c.ID, err))
			}
		case "command":
			if strings.TrimSpace(c.Command) == "" {
				return GateError(fmt.Sprintf("eval check %q command is required", c.ID))
			}
			if c.TimeoutMs < 0 {
				return GateError(fmt.Sprintf("eval check %q timeoutMs must be non-negative", c.ID))
			}
			if c.WorkingDir != "" {
				if err := validateRelPath(c.WorkingDir); err != nil {
					return GateError(fmt.Sprintf("eval check %q invalid workingDir: %v", c.ID, err))
				}
			}
			switch c.Sandbox {
			case "", "none", "bwrap", "container":
			default:
				return GateError(fmt.Sprintf("eval check %q sandbox must be none, bwrap, or container", c.ID))
			}
		case "trajectory":
			switch c.Predicate {
			case "exists", "max-events", "min-events", "pattern":
			default:
				return GateError(fmt.Sprintf("eval check %q unknown trajectory predicate %q", c.ID, c.Predicate))
			}
			if c.Predicate == "pattern" {
				if c.Pattern == "" {
					return GateError(fmt.Sprintf("eval check %q pattern predicate requires pattern", c.ID))
				}
				if _, err := regexp.Compile(c.Pattern); err != nil {
					return GateError(fmt.Sprintf("eval check %q invalid regex: %v", c.ID, err))
				}
			}
		default:
			return GateError(fmt.Sprintf("eval check %q unknown kind %q", c.ID, c.Kind))
		}
	}
	return nil
}

func RunEval(root, slug string, rubric *EvalRubric, rubricDigest string) (*EvalReport, error) {
	results := make([]EvalCheckResult, 0, len(rubric.Checks))
	failures := []string{}
	var earned, total float64
	for _, check := range rubric.Checks {
		result := runEvalCheck(root, slug, check)
		results = append(results, result)
		total += result.Weight
		earned += result.Weight * result.Score
		if !result.Passed {
			failures = append(failures, result.ID)
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	score := 0.0
	if total > 0 {
		score = earned / total
	}
	score = roundScore(score)
	passed := score >= rubric.MinScore
	for _, result := range results {
		if result.Weight > 0 && result.Score == 0 && result.Passed == false && checkRequired(rubric, result.ID) {
			passed = false
			break
		}
	}
	return &EvalReport{
		SchemaVersion: EvalSchemaVersion,
		Suite:         rubric.Suite,
		Score:         score,
		MinScore:      rubric.MinScore,
		Passed:        passed,
		RubricDigest:  rubricDigest,
		GeneratedAt:   Clock().UTC().Format(time.RFC3339Nano),
		Checks:        results,
		Failures:      failures,
	}, nil
}

func SaveEvalReport(root, slug string, report *EvalReport) (string, error) {
	dir := filepath.Join(SpecDir(root, slug), "evals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	seq := 1
	for {
		path := filepath.Join(dir, fmt.Sprintf("%s-%03d.json", report.Suite, seq))
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			report.ReportPath = filepath.ToSlash(filepath.Join(".specd", "specs", slug, "evals", filepath.Base(path)))
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return "", err
			}
			if err := AtomicWrite(path, string(data)+"\n"); err != nil {
				return "", err
			}
			return path, nil
		}
		seq++
	}
}

func MarkPrototypePromoted(state *State, report *EvalReport, evidence string) error {
	if state.Prototype == nil || state.Prototype.Status == "" {
		return GateError("spec is not marked as prototype")
	}
	if !report.Passed {
		return GateError("cannot promote prototype without passing eval")
	}
	if strings.TrimSpace(evidence) == "" {
		return GateError("promote requires --evidence")
	}
	if strings.ContainsRune(evidence, 0) {
		return GateError("promote evidence contains NUL byte")
	}
	state.Prototype.Status = PrototypePromoted
	state.Prototype.PromotedAt = Clock().UTC().Format(time.RFC3339Nano)
	state.Prototype.EvalReport = report.ReportPath
	state.Prototype.EvalDigest = report.RubricDigest
	state.Prototype.Evidence = evidence
	state.Prototype.PromotedScore = strconv.FormatFloat(report.Score, 'f', 3, 64)
	return nil
}

func runEvalCheck(root, slug string, check EvalCheck) EvalCheckResult {
	start := time.Now()
	result := EvalCheckResult{ID: check.ID, Kind: check.Kind, Weight: check.Weight}
	switch check.Kind {
	case "regex":
		result.Passed, result.Message, result.Digest = evalRegexCheck(root, slug, check)
	case "command":
		result.Passed, result.Message, result.Digest = evalCommandCheck(root, check)
	case "trajectory":
		result.Passed, result.Message, result.Digest = evalTrajectoryCheck(root, slug, check)
	}
	if result.Passed {
		result.Score = 1
	}
	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

func evalRegexCheck(root, slug string, check EvalCheck) (bool, string, string) {
	path := filepath.Join(SpecDir(root, slug), check.Path)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err.Error(), ""
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return false, "file contains NUL byte", digestBytes(data)
	}
	re := regexp.MustCompile(check.Pattern)
	if re.Match(data) {
		return true, "matched", digestBytes(data)
	}
	return false, "pattern not found", digestBytes(data)
}

// evalCommandCheck runs a rubric command check through the shared verify runner
// so it inherits the same env scrub (ScrubbedEnv) and fail-closed sandbox as
// verify and custom gates — the single shared sandboxed exec path required by
// invariant 8. Exit code 0 = pass; the combined stdout/stderr tail is recorded
// as the evidence digest.
func evalCommandCheck(root string, check EvalCheck) (bool, string, string) {
	shell := strings.TrimSpace(os.Getenv("SPECD_VERIFY_SHELL"))
	if shell == "" {
		shell = "sh"
	}
	cwd := root
	if check.WorkingDir != "" {
		cwd = filepath.Join(root, check.WorkingDir)
	}
	r, err := runner.SelectRunner(check.Sandbox)
	if err != nil {
		return false, fmt.Sprintf("sandbox unavailable: %v", err), ""
	}
	timeout := 60 * time.Second
	if check.TimeoutMs > 0 {
		timeout = time.Duration(check.TimeoutMs) * time.Millisecond
	}
	res := r.Run(context.Background(), runner.RunSpec{
		Root:    cwd,
		Shell:   shell,
		Command: check.Command,
		Env:     ScrubbedEnv(),
		Timeout: timeout,
	})
	out := res.Stdout + res.Stderr
	digest := digestBytes([]byte(out))
	if res.TimedOut {
		return false, fmt.Sprintf("command timed out after %s", timeout), digest
	}
	tail := tailString(out, 2048)
	if res.ExitCode != 0 {
		if tail == "" {
			tail = fmt.Sprintf("exit %d", res.ExitCode)
		}
		return false, tail, digest
	}
	if tail == "" {
		tail = "command passed"
	}
	return true, tail, digest
}

func evalTrajectoryCheck(root, slug string, check EvalCheck) (bool, string, string) {
	events, err := ReadTrajectoryFile(TrajectoryPath(root, slug))
	if err != nil {
		return false, err.Error(), ""
	}
	data, _ := json.Marshal(events)
	count := len(events)
	switch check.Predicate {
	case "exists":
		return count > 0, fmt.Sprintf("%d events", count), digestBytes(data)
	case "max-events":
		return count <= check.Max, fmt.Sprintf("%d events <= %d", count, check.Max), digestBytes(data)
	case "min-events":
		return count >= check.Min, fmt.Sprintf("%d events >= %d", count, check.Min), digestBytes(data)
	case "pattern":
		re := regexp.MustCompile(check.Pattern)
		return re.Match(data), "trajectory pattern", digestBytes(data)
	default:
		return false, "unknown predicate", digestBytes(data)
	}
}

func checkRequired(r *EvalRubric, id string) bool {
	for _, c := range r.Checks {
		if c.ID == id {
			return c.Required
		}
	}
	return false
}

func validEvalID(s string) bool {
	return regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(s)
}

func validateRelPath(path string) error {
	if filepath.IsAbs(path) || strings.Contains(path, "\x00") {
		return fmt.Errorf("must be a relative path")
	}
	clean := filepath.Clean(path)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("must stay under the spec directory")
	}
	return nil
}

func tailString(s string, limit int) string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) == 0 {
		if len(s) > limit {
			return s[len(s)-limit:]
		}
		return s
	}
	joined := strings.Join(lines, "\n")
	if len(joined) > limit {
		return joined[len(joined)-limit:]
	}
	return joined
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hasNUL(values ...string) bool {
	for _, v := range values {
		if strings.ContainsRune(v, 0) {
			return true
		}
	}
	return false
}

func roundScore(v float64) float64 {
	return float64(int(v*1000000+0.5)) / 1000000
}

// EvalRubricSkeleton compiles requirements prose into a rubric skeleton with one
// stub check per acceptance criterion. The transform is deterministic and
// interpretation-free: criterion IDs fully determine the check IDs and count.
// Every stub is a regex check with a placeholder pattern the author replaces —
// minScore stays 0 so a freshly-compiled skeleton never blocks by accident.
func EvalRubricSkeleton(requirements string) *EvalRubric {
	crits := ExtractCriteria(requirements)
	rubric := &EvalRubric{SchemaVersion: EvalSchemaVersion, Suite: "default", MinScore: 0}
	for _, c := range crits {
		rubric.Checks = append(rubric.Checks, EvalCheck{
			ID:      "crit-" + strings.ReplaceAll(c.ID, ".", "-"),
			Kind:    "regex",
			Weight:  1,
			Path:    "spec.md",
			Pattern: "REPLACE-ME",
		})
	}
	return rubric
}

// MarshalEvalRubric renders a rubric as canonical indented JSON with a trailing
// newline, matching the byte discipline of the rest of specd's artifacts.
func MarshalEvalRubric(r *EvalRubric) (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

// EvalTrendRun is one recorded eval run in chronological (seq) order, with the
// score delta from the previous run of the same suite.
type EvalTrendRun struct {
	Suite  string  `json:"suite"`
	Seq    int     `json:"seq"`
	Score  float64 `json:"score"`
	Delta  float64 `json:"delta"`
	Passed bool    `json:"passed"`
}

// EvalFailureCluster counts how often a given check ID failed across the run
// history. Clustering is by exact check ID — a deterministic key, no prose
// interpretation.
type EvalFailureCluster struct {
	CheckID string `json:"checkID"`
	Count   int    `json:"count"`
}

// EvalTrendReport is the deterministic rollup of a spec's eval result history.
type EvalTrendReport struct {
	Slug     string               `json:"slug"`
	Runs     []EvalTrendRun       `json:"runs"`
	Clusters []EvalFailureCluster `json:"clusters,omitempty"`
}

// EvalTrend reads the result-file history for a spec and returns score deltas
// plus failure clustering. When suite is non-empty only that suite's runs are
// considered. Output is a pure function of the on-disk result files.
func EvalTrend(root, slug, suite string) (*EvalTrendReport, error) {
	dir := filepath.Join(SpecDir(root, slug), "evals")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return &EvalTrendReport{Slug: slug}, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	report := &EvalTrendReport{Slug: slug}
	prev := map[string]float64{}
	seenPrev := map[string]bool{}
	failCounts := map[string]int{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var r EvalReport
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, GateError(fmt.Sprintf("eval trend: invalid result file %s: %v", name, err))
		}
		if suite != "" && r.Suite != suite {
			continue
		}
		delta := 0.0
		if seenPrev[r.Suite] {
			delta = roundScore(r.Score - prev[r.Suite])
		}
		prev[r.Suite] = r.Score
		seenPrev[r.Suite] = true
		report.Runs = append(report.Runs, EvalTrendRun{
			Suite:  r.Suite,
			Seq:    seqFromEvalReportName(name),
			Score:  r.Score,
			Delta:  delta,
			Passed: r.Passed,
		})
		for _, id := range r.Failures {
			failCounts[id]++
		}
	}
	for id, n := range failCounts {
		report.Clusters = append(report.Clusters, EvalFailureCluster{CheckID: id, Count: n})
	}
	sort.SliceStable(report.Clusters, func(i, j int) bool {
		if report.Clusters[i].Count != report.Clusters[j].Count {
			return report.Clusters[i].Count > report.Clusters[j].Count
		}
		return report.Clusters[i].CheckID < report.Clusters[j].CheckID
	})
	return report, nil
}

func seqFromEvalReportName(name string) int {
	base := strings.TrimSuffix(name, ".json")
	i := strings.LastIndex(base, "-")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(base[i+1:])
	if err != nil {
		return 0
	}
	return n
}
