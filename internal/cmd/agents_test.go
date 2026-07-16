package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestAgentsDoctorJSONHealthyEnvelope(t *testing.T) {
	root := t.TempDir()
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return Run(root, "agents", []string{"doctor"}, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ProtocolVersion string               `json:"protocol_version"`
		Healthy         bool                 `json:"healthy"`
		Findings        []core.DriverFinding `json:"findings"`
		NextAction      string               `json:"next_action"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("doctor JSON invalid: %v\n%s", err, out)
	}
	if got.ProtocolVersion != core.DriverProtocolVersion || !got.Healthy || got.Findings == nil || len(got.Findings) != 0 || got.NextAction == "" {
		t.Fatalf("doctor JSON ambiguous: %+v", got)
	}
}

func TestAgentsDoctorJSONDefectiveEnvelope(t *testing.T) {
	root := t.TempDir()
	out, err := captureStdout(t, func() error {
		return Run(root, "agents", []string{"doctor"}, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ProtocolVersion string               `json:"protocol_version"`
		Healthy         bool                 `json:"healthy"`
		Findings        []core.DriverFinding `json:"findings"`
		NextAction      string               `json:"next_action"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("doctor JSON invalid: %v\n%s", err, out)
	}
	if got.ProtocolVersion != core.DriverProtocolVersion || got.Healthy || len(got.Findings) == 0 || got.NextAction == "" {
		t.Fatalf("defective doctor JSON ambiguous: %+v", got)
	}
}
