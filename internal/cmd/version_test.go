package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return Run(t.TempDir(), "version", nil, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "specd ") {
		t.Fatalf("unexpected version output: %q", out)
	}
}

func TestRunVersionJSON(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return Run(t.TempDir(), "version", nil, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version string `json:"version"`
		Go      string `json:"go"`
		OS      string `json:"os"`
		Arch    string `json:"arch"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version == "" || got.Go == "" || got.OS == "" || got.Arch == "" {
		t.Fatalf("missing version fields: %+v", got)
	}
}
