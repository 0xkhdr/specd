//go:build !windows

package worker

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestShellRunnerPersistsSpecScopedMission(t *testing.T) {
	root := t.TempDir()
	m := Mission{
		Command:  "true",
		Root:     root,
		Spec:     "auth-flow",
		TaskID:   "T1",
		Attempt:  2,
		WorkerID: "w1",
	}
	r := ShellRunner{Stdout: io.Discard, Stderr: io.Discard}
	if _, err := r.Run(context.Background(), m); err != nil {
		t.Fatalf("Run: %v", err)
	}

	paths, err := core.NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := paths.MissionPath("auth-flow", "T1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if want != filepath.Join(root, ".specd", "runtime", "missions", "auth-flow-T1-2.json") {
		t.Fatalf("unexpected mission path %q", want)
	}
	raw, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("durable mission not persisted: %v", err)
	}
	var got Mission
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("mission file is not valid JSON: %v", err)
	}
	if got.TaskID != "T1" || got.Spec != "auth-flow" {
		t.Fatalf("persisted mission payload wrong: %+v", got)
	}
}

func TestShellRunnerDeterministicOverwrite(t *testing.T) {
	root := t.TempDir()
	r := ShellRunner{Stdout: io.Discard, Stderr: io.Discard}
	m := Mission{Command: "true", Root: root, Spec: "s1", TaskID: "T3", Attempt: 1, WorkerID: "w"}
	if _, err := r.Run(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	missions := filepath.Join(root, ".specd", "runtime", "missions")
	entries, err := os.ReadDir(missions)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("re-issued attempt duplicated mission files: %d entries", len(entries))
	}
}

func TestShellRunnerNoRootKeepsLegacyContract(t *testing.T) {
	// Without Root, no durable record is written; the temp-file transport remains.
	r := ShellRunner{Stdout: io.Discard, Stderr: io.Discard}
	if _, err := r.Run(context.Background(), Mission{Command: "true", WorkerID: "w"}); err != nil {
		t.Fatalf("legacy mission run failed: %v", err)
	}
}
