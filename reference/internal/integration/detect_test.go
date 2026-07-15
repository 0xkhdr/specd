package integration

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectUsesExecutableAndProjectMarkerEvidence(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".host", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	detector := Detector{
		LookPath: func(name string) (string, error) {
			if name == "agent" {
				return "/fixture/bin/agent", nil
			}
			return "", errors.New("not found")
		},
		Stat: os.Stat,
	}
	got := detector.Detect(root, "host", DetectionProbe{
		Executable:    "agent",
		ProjectConfig: ".host/mcp.json",
		Scopes:        []Scope{ScopeGlobal, ScopeProject},
		Method:        "native-cli",
	})
	if !got.Detected || got.Confidence != ConfidenceHigh {
		t.Fatalf("Detect() = %#v", got)
	}
	if got.Executable != "/fixture/bin/agent" || got.ProjectConfig != config {
		t.Fatalf("Detect() evidence = %#v", got)
	}
	if want := []Scope{ScopeGlobal, ScopeProject}; !reflect.DeepEqual(got.Scopes, want) {
		t.Fatalf("Scopes = %v, want %v", got.Scopes, want)
	}
}

func TestSelectHostsAmbiguousNonInteractiveDoesNotSelect(t *testing.T) {
	detections := []Detection{
		{Host: "codex", Detected: true},
		{Host: "claude-code", Detected: true},
		{Host: "cursor", Detected: false},
	}
	got, err := SelectHosts("auto", false, detections)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Ambiguous || len(got.Selected) != 0 {
		t.Fatalf("SelectHosts() = %#v", got)
	}
	if want := []string{"claude-code", "codex"}; !reflect.DeepEqual(got.Suggestions, want) {
		t.Fatalf("Suggestions = %v, want %v", got.Suggestions, want)
	}
}

func TestSelectHostsExplicitAndUnsupported(t *testing.T) {
	detections := []Detection{{Host: "codex"}}
	got, err := SelectHosts("codex", false, detections)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Selected, []string{"codex"}) {
		t.Fatalf("Selected = %v", got.Selected)
	}
	if _, err := SelectHosts("unknown", false, detections); err == nil {
		t.Fatal("unsupported explicit host accepted")
	}
}
