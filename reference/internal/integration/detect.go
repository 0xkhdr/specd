package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DetectionProbe describes passive evidence for one host.
type DetectionProbe struct {
	Executable    string
	ProjectConfig string
	Scopes        []Scope
	Method        string
}

// Detector permits hermetic PATH and filesystem tests.
type Detector struct {
	LookPath func(string) (string, error)
	Stat     func(string) (os.FileInfo, error)
}

func DefaultDetector() Detector {
	return Detector{LookPath: exec.LookPath, Stat: os.Stat}
}

func (d Detector) Detect(root, host string, probe DetectionProbe) Detection {
	result := Detection{
		Host:       host,
		Scopes:     append([]Scope(nil), probe.Scopes...),
		Method:     probe.Method,
		Confidence: ConfidenceNone,
	}
	if d.LookPath == nil {
		d.LookPath = exec.LookPath
	}
	if d.Stat == nil {
		d.Stat = os.Stat
	}

	var evidence []string
	if probe.Executable != "" {
		if executable, err := d.LookPath(probe.Executable); err == nil {
			result.Executable = executable
			evidence = append(evidence, "executable "+probe.Executable+" found")
		}
	}
	if probe.ProjectConfig != "" {
		config := filepath.Join(root, filepath.FromSlash(probe.ProjectConfig))
		if _, err := d.Stat(config); err == nil {
			result.ProjectConfig = config
			evidence = append(evidence, "project config "+probe.ProjectConfig+" found")
		}
	}

	result.Detected = len(evidence) > 0
	switch len(evidence) {
	case 0:
		result.Reason = "no executable or project configuration found"
	case 1:
		result.Confidence = ConfidenceMedium
		result.Reason = evidence[0]
	default:
		result.Confidence = ConfidenceHigh
		result.Reason = strings.Join(evidence, "; ")
	}
	return normalizeDetection(result)
}

func DetectAll(registry *Registry, root string) []Detection {
	results := make([]Detection, 0, len(registry.adapters))
	for _, adapter := range registry.Adapters() {
		result := normalizeDetection(adapter.Detect(root))
		if result.Host == "" {
			result.Host = adapter.Name()
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Host < results[j].Host })
	return results
}

// Selection is a non-mutating resolution of an --agent choice.
type Selection struct {
	Selected    []string `json:"selected"`
	Suggestions []string `json:"suggestions"`
	Ambiguous   bool     `json:"ambiguous"`
	Reason      string   `json:"reason"`
}

func SelectHosts(selection string, interactive bool, detections []Detection) (Selection, error) {
	result := Selection{Selected: []string{}, Suggestions: []string{}}
	detected := make([]string, 0, len(detections))
	known := make(map[string]bool, len(detections))
	for _, detection := range detections {
		known[detection.Host] = true
		if detection.Detected {
			detected = append(detected, detection.Host)
		}
	}
	sort.Strings(detected)

	switch selection {
	case "", "auto":
		switch len(detected) {
		case 0:
			result.Reason = "no supported coding-agent host detected"
		case 1:
			result.Selected = append(result.Selected, detected[0])
			result.Reason = "one supported coding-agent host detected"
		default:
			result.Suggestions = append(result.Suggestions, detected...)
			result.Ambiguous = true
			if interactive {
				result.Reason = "multiple hosts detected; user selection required"
			} else {
				result.Reason = "multiple hosts detected in non-interactive mode; no host selected"
			}
		}
	case "all":
		result.Selected = append(result.Selected, detected...)
		result.Reason = "all detected hosts selected"
	case "none":
		result.Reason = "host integration disabled"
	default:
		if !known[selection] {
			return result, fmt.Errorf("unsupported host %q", selection)
		}
		result.Selected = append(result.Selected, selection)
		result.Reason = "explicit host selected"
	}
	return result, nil
}
