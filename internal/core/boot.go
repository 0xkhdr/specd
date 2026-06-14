package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
)

// BootDetectorVersion is recorded in boot.json so freshness checks can tell
// which detector ruleset produced a given result. Bump on any change to the
// detection ruleset so older boot.json files are flagged stale.
const BootDetectorVersion = "1.1.0"

// Detection is a single detector's verdict for one ecosystem. A detector
// returns nil when its ecosystem is not present in the repo.
type Detection struct {
	Stack       string   // canonical stack id, e.g. "python", "nodejs", "rust", "go"
	ProjectName string   // name parsed from a manifest, if any
	Frameworks  []string // detected frameworks within this stack
	Verify      string   // inferred verify command
	VerifyFrom  string   // human-readable source of the verify inference
	Sources     []string // files that triggered this detection
	Priority    int      // filled from Detector.Priority(); higher wins ties
}

// Detector is a standalone, AI-free ecosystem detector. Detect must be pure
// static file analysis: no exec, no network.
type Detector interface {
	Name() string
	Priority() int
	Detect(root string) *Detection
}

// bootDetectors is the ordered registry. Priority (not slice order) decides
// which stack supplies the primary verify command; on equal priority the stack
// name (ascending) breaks the tie, so the winner is fully deterministic for a
// polyglot repo. Current priorities: python 40, java 35, go/rust 30,
// ruby/php/elixir 25, nodejs 20. Example: a Go+Rust repo both score 30, so "go"
// (alphabetically first) supplies the primary verify and "rust" becomes an
// alternative. Slice order here is irrelevant to the outcome — AnalyzeBoot sorts.
var bootDetectors = []Detector{
	pythonDetector{},
	javaDetector{},
	goDetector{},
	rustDetector{},
	nodeDetector{},
	rubyDetector{},
	phpDetector{},
	elixirDetector{},
}

// BootResult is the merged, deterministic output of all detectors. Field order
// is normalized (sorted) so the same repo state always yields identical JSON.
type BootResult struct {
	ProjectName     string              `json:"projectName"`
	Stacks          []string            `json:"stacks"`
	Frameworks      map[string][]string `json:"frameworks"`
	Verify          string              `json:"verifyCommand"`
	VerifyFrom      string              `json:"verifyCommandSource"`
	VerifyAlts      []string            `json:"verifyAlternatives"`
	Build           []string            `json:"buildTools"`
	CI              []string            `json:"ciProviders"`
	Layout          string              `json:"layout"`
	Sources         []string            `json:"sources"`
	Conflicts       []string            `json:"conflicts"`
	GeneratedAt     string              `json:"generatedAt"`
	DetectorVersion string              `json:"detectorVersion"`
}

// AnalyzeBoot runs every detector against root and merges the verdicts. It is
// pure with respect to the filesystem — GeneratedAt is left blank for the
// caller to stamp, keeping AnalyzeBoot deterministic and unit-testable.
func AnalyzeBoot(root string) BootResult {
	var dets []*Detection
	for _, d := range bootDetectors {
		if det := d.Detect(root); det != nil {
			det.Priority = d.Priority()
			dets = append(dets, det)
		}
	}

	// Order by descending priority (then stack name) so the primary verify
	// command is chosen deterministically.
	sort.SliceStable(dets, func(i, j int) bool {
		if dets[i].Priority != dets[j].Priority {
			return dets[i].Priority > dets[j].Priority
		}
		return dets[i].Stack < dets[j].Stack
	})

	res := BootResult{
		Stacks:          []string{},
		Frameworks:      map[string][]string{},
		VerifyAlts:      []string{},
		Build:           []string{},
		CI:              []string{},
		Sources:         []string{},
		Conflicts:       []string{},
		DetectorVersion: BootDetectorVersion,
	}

	srcSet := map[string]bool{}
	for _, d := range dets {
		res.Stacks = append(res.Stacks, d.Stack)
		if len(d.Frameworks) > 0 {
			fw := append([]string{}, d.Frameworks...)
			sort.Strings(fw)
			res.Frameworks[d.Stack] = fw
		}
		for _, s := range d.Sources {
			if !srcSet[s] {
				srcSet[s] = true
				res.Sources = append(res.Sources, s)
			}
		}
		if d.Verify != "" {
			if res.Verify == "" {
				res.Verify = d.Verify
				res.VerifyFrom = d.VerifyFrom
			} else {
				res.VerifyAlts = append(res.VerifyAlts, d.Verify)
			}
		}
		if res.ProjectName == "" && d.ProjectName != "" {
			res.ProjectName = d.ProjectName
		}
	}

	if res.ProjectName == "" {
		res.ProjectName = baseName(root)
	}
	res.Conflicts = bootConflicts(dets)

	// Build / CI / layout — file-existence signals, independent of stack.
	build, buildSrc := detectBuildTools(root)
	res.Build = build
	ci, ciSrc := detectCI(root)
	res.CI = ci
	res.Layout = detectLayout(root)
	for _, s := range append(buildSrc, ciSrc...) {
		if !srcSet[s] {
			srcSet[s] = true
			res.Sources = append(res.Sources, s)
		}
	}

	// Normalize for stable output.
	sort.Strings(res.Stacks)
	sort.Strings(res.Sources)
	return res
}

// bootConflicts flags ambiguities a human should resolve, e.g. two test
// frameworks claimed within one stack.
func bootConflicts(dets []*Detection) []string {
	out := []string{}
	testFW := map[string][]string{
		"nodejs": {"jest", "vitest"},
		"python": {"pytest"},
	}
	for _, d := range dets {
		runners, ok := testFW[d.Stack]
		if !ok {
			continue
		}
		var found []string
		for _, r := range runners {
			if contains(d.Frameworks, r) {
				found = append(found, r)
			}
		}
		if len(found) > 1 {
			out = append(out, d.Stack+": multiple test frameworks detected ("+joinComma(found)+") — pick one for verify")
		}
	}
	sort.Strings(out)
	return out
}

// BootFreshness is the verdict of the boot-freshness gate.
type BootFreshness struct {
	Stale  bool     `json:"stale"`
	Issues []string `json:"issues"`
}

// CheckBootFreshness validates that .specd/boot.json still reflects the repo:
// every recorded source must still exist, the detector version must match, and
// a fresh re-analysis must equal the stored result (timestamp aside). Returns a
// NotFoundError when boot.json is absent.
func CheckBootFreshness(root string) (BootFreshness, error) {
	raw := ReadOrNull(filepath.Join(SpecdDir(root), "boot.json"))
	if raw == nil {
		return BootFreshness{}, NotFoundError("no .specd/boot.json — run `specd boot` first.")
	}
	var stored BootResult
	if err := json.Unmarshal([]byte(*raw), &stored); err != nil {
		return BootFreshness{}, GateError("boot.json is not valid JSON: " + err.Error())
	}

	issues := []string{}
	for _, s := range stored.Sources {
		if !FileExists(filepath.Join(root, s)) {
			issues = append(issues, "recorded source no longer exists: "+s)
		}
	}
	if stored.DetectorVersion != BootDetectorVersion {
		issues = append(issues, fmt.Sprintf("detector version drift: boot.json=%s, current=%s — re-run `specd boot --force`", stored.DetectorVersion, BootDetectorVersion))
	}

	current := AnalyzeBoot(root)
	current.GeneratedAt = ""
	stored.GeneratedAt = ""
	if !reflect.DeepEqual(stored, current) {
		if !equalStrs(stored.Stacks, current.Stacks) {
			issues = append(issues, fmt.Sprintf("stacks drift: boot.json=%v, current=%v", stored.Stacks, current.Stacks))
		}
		if !equalStrs(stored.Sources, current.Sources) {
			issues = append(issues, fmt.Sprintf("sources drift: boot.json=%v, current=%v", stored.Sources, current.Sources))
		}
		if len(issues) == 0 {
			issues = append(issues, "detection drift: boot.json differs from current analysis — re-run `specd boot --force`")
		}
	}
	return BootFreshness{Stale: len(issues) > 0, Issues: issues}, nil
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
