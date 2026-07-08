package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultMaxObserveBytes caps a single inbound error payload. Production
// observability payloads are small; a large body is hostile input and rejected
// before parsing (V9 §5).
const DefaultMaxObserveBytes = 256 * 1024

// StackFrame is one correlation hint from a production error: a source file and
// (optionally) line and symbol. File is matched against task `files:` contracts.
type StackFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

// ErrorPayload is the schema-validated inbound production error (V9/P5.2). It is
// hostile input: unknown fields, absolute/traversing frame paths, and missing
// required fields are rejected. Fields mirror the common Sentry-export shape.
type ErrorPayload struct {
	Service     string       `json:"service,omitempty"`
	Environment string       `json:"environment,omitempty"`
	Severity    string       `json:"severity"`
	Message     string       `json:"message"`
	Fingerprint string       `json:"fingerprint,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Frames      []StackFrame `json:"frames,omitempty"`
}

// ParseErrorPayload strictly decodes and validates an error payload.
func ParseErrorPayload(data []byte) (ErrorPayload, error) {
	var p ErrorPayload
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return ErrorPayload{}, GateError(fmt.Sprintf("error payload: %v", err))
	}
	if err := ValidateErrorPayload(p); err != nil {
		return ErrorPayload{}, err
	}
	return p, nil
}

// ValidateErrorPayload enforces the payload invariants: a message is required,
// severity must be recognized, and every frame path must be repo-relative with
// no traversal (correlation hints are matched against task contracts, so a path
// escaping the repo is rejected outright).
func ValidateErrorPayload(p ErrorPayload) error {
	if strings.TrimSpace(p.Message) == "" {
		return GateError("error payload: message is required")
	}
	if severityImpact(p.Severity) == "" {
		return GateError(fmt.Sprintf("error payload: unknown severity %q (allowed: info, warning, error, fatal, critical)", p.Severity))
	}
	for _, f := range p.Frames {
		if err := validateFramePath(f.File); err != nil {
			return err
		}
		if f.Line < 0 {
			return GateError(fmt.Sprintf("error payload: frame %q has negative line", f.File))
		}
	}
	return nil
}

// validateFramePath rejects absolute paths and any `..` traversal segment.
func validateFramePath(p string) error {
	if p == "" {
		return nil
	}
	if filepath.IsAbs(p) {
		return GateError(fmt.Sprintf("error payload: absolute frame path %q rejected", p))
	}
	clean := filepath.ToSlash(filepath.Clean(p))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return GateError(fmt.Sprintf("error payload: frame path %q escapes the repo", p))
	}
	return nil
}

// severityImpact maps a payload severity to a specd midreq impact, or "" when
// the severity is unrecognized. Deterministic total function.
func severityImpact(sev string) string {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical", "fatal":
		return "critical"
	case "error":
		return "high"
	case "warning", "warn":
		return "medium"
	case "info", "debug":
		return "low"
	}
	return ""
}

// SeverityImpact is the exported mapping used by callers to gate high/critical
// entries (V9/P5.2).
func SeverityImpact(sev string) string { return severityImpact(sev) }

// Correlation is the deterministic result of matching a payload to a spec: the
// attributed spec, the tasks whose file contracts matched, the matched frame
// files, the mapped impact, and the confidence with its supporting facts.
type Correlation struct {
	Spec         string   `json:"spec"`
	Tasks        []string `json:"tasks,omitempty"`
	MatchedFiles []string `json:"matchedFiles,omitempty"`
	Impact       string   `json:"impact"`
	Confidence   string   `json:"confidence"`
	Facts        []string `json:"facts"`
}

// CorrelatePayload attributes an error payload to a spec, deterministically.
// Attribution order: an explicit forceSpec; else the spec whose task `files:`
// contracts match the most frame paths (ties broken by slug); else the spec with
// the most recent recorded deploy to the payload's environment. When nothing
// correlates it returns an error asking for --spec, so every accepted error can
// still become an evidenced midreq entry (success metric).
func CorrelatePayload(root string, p ErrorPayload, forceSpec string) (Correlation, error) {
	impact := severityImpact(p.Severity)
	if forceSpec != "" {
		if err := RequireSpec(root, forceSpec); err != nil {
			return Correlation{}, err
		}
		tasks, files := matchSpecFrames(root, forceSpec, p.Frames)
		return buildCorrelation(root, forceSpec, tasks, files, p, impact, true), nil
	}

	best := ""
	var bestTasks, bestFiles []string
	for _, slug := range ListSpecs(root) {
		tasks, files := matchSpecFrames(root, slug, p.Frames)
		if len(files) > len(bestFiles) {
			best, bestTasks, bestFiles = slug, tasks, files
		}
	}
	if best != "" {
		return buildCorrelation(root, best, bestTasks, bestFiles, p, impact, false), nil
	}

	// No file match — fall back to the most-recent deploy to this environment.
	if slug := recentDeploySpec(root, p.Environment); slug != "" {
		return buildCorrelation(root, slug, nil, nil, p, impact, false), nil
	}
	return Correlation{}, GateError("observe: no spec correlated to this payload — re-run with --spec <slug> to attribute it")
}

// matchSpecFrames returns the tasks whose files contract matches any frame path
// and the sorted set of matched frame files, for one spec.
func matchSpecFrames(root, slug string, frames []StackFrame) (tasks, files []string) {
	raw := ReadArtifact(root, slug, "tasks.md")
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	doc, err := ParseTasks(*raw)
	if err != nil {
		return nil, nil
	}
	fileSet := map[string]bool{}
	taskSet := map[string]bool{}
	for _, t := range doc.Tasks {
		patterns := parseFilesContract(t.Meta["files"])
		if len(patterns) == 0 || containsStr(patterns, "*") {
			continue
		}
		for _, f := range frames {
			if f.File == "" {
				continue
			}
			clean := filepath.ToSlash(filepath.Clean(f.File))
			if matchesAnyGlob(clean, patterns) {
				fileSet[clean] = true
				taskSet[t.ID] = true
			}
		}
	}
	return sortedSetKeys(taskSet), sortedSetKeys(fileSet)
}

// recentDeploySpec returns the spec whose recorded deploy targeted env, breaking
// ties by most recent time then slug. "" when none.
func recentDeploySpec(root, env string) string {
	best, bestTime := "", ""
	for _, slug := range ListSpecs(root) {
		st, err := LoadState(root, slug)
		if err != nil || st == nil || st.Deploy == nil {
			continue
		}
		if env != "" && st.Deploy.Env != env {
			continue
		}
		if st.Deploy.Time > bestTime || (st.Deploy.Time == bestTime && slug < best) {
			best, bestTime = slug, st.Deploy.Time
		}
	}
	return best
}

// buildCorrelation assembles the Correlation with its confidence facts. High
// confidence requires both a file-contract match and a recorded deploy to the
// payload environment; a file match alone is medium; deploy-only or forced-with-
// no-match is low.
func buildCorrelation(root, slug string, tasks, files []string, p ErrorPayload, impact string, forced bool) Correlation {
	c := Correlation{Spec: slug, Tasks: tasks, MatchedFiles: files, Impact: impact}
	st, _ := LoadState(root, slug)
	deployMatch := st != nil && st.Deploy != nil && (p.Environment == "" || st.Deploy.Env == p.Environment)

	switch {
	case len(files) > 0 && deployMatch:
		c.Confidence = "high"
		c.Facts = append(c.Facts, fmt.Sprintf("%d frame file(s) match task contract(s) %s", len(files), strings.Join(tasks, ",")))
		c.Facts = append(c.Facts, fmt.Sprintf("spec has a recorded deploy to env %q", st.Deploy.Env))
	case len(files) > 0:
		c.Confidence = "medium"
		c.Facts = append(c.Facts, fmt.Sprintf("%d frame file(s) match task contract(s) %s", len(files), strings.Join(tasks, ",")))
	case deployMatch:
		c.Confidence = "low"
		c.Facts = append(c.Facts, fmt.Sprintf("no frame matched a task contract; attributed by recent deploy to env %q", st.Deploy.Env))
	default:
		c.Confidence = "low"
		c.Facts = append(c.Facts, "operator-forced attribution (--spec); no frame or deploy correlation")
	}
	if forced {
		c.Facts = append(c.Facts, "attribution pinned by --spec")
	}
	return c
}

// RenderObserveMidreq renders the deterministic mid-requirements.md entry body
// for a correlated production error. The turn header is prepended by the caller
// (which owns the state lock and turn counter), mirroring `specd midreq`.
func RenderObserveMidreq(p ErrorPayload, c Correlation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**Source:** production error (observe)\n")
	if p.Service != "" {
		fmt.Fprintf(&b, "**Service:** %s\n", p.Service)
	}
	if p.Environment != "" {
		fmt.Fprintf(&b, "**Environment:** %s\n", p.Environment)
	}
	fmt.Fprintf(&b, "**Severity:** %s → impact %s\n", p.Severity, c.Impact)
	fmt.Fprintf(&b, "**Message (verbatim):** %q\n", p.Message)
	if p.Fingerprint != "" {
		fmt.Fprintf(&b, "**Fingerprint:** %s\n", p.Fingerprint)
	}
	if len(c.MatchedFiles) > 0 {
		fmt.Fprintf(&b, "**Matched files:** %s\n", strings.Join(c.MatchedFiles, ", "))
	}
	fmt.Fprintf(&b, "**Correlation confidence:** %s\n", c.Confidence)
	for _, f := range c.Facts {
		fmt.Fprintf(&b, "  - %s\n", f)
	}
	return b.String()
}

func sortedSetKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
