package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/adapter"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// runOTelExport projects the spec's local observable-event traces
// (evals/traces/<run>.jsonl) into OpenTelemetry-compatible spans as stable JSON
// Lines (spec 10 R10.2). Each trace file is one run: it is parsed with the
// forbidden-field guard, then mapped through the external adapter, so raw
// source/prompt data is absent by construction and correlation is preserved. A
// spec with no recorded traces contributes nothing — graceful degradation,
// mirroring the other report projections. Two exports over the same tree are
// byte-identical: files are visited in sorted order and the export writes nothing.
func runOTelExport(root, slug string) (string, error) {
	dir := filepath.Dir(core.EvalTracePath(root, slug, "_"))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		events, err := orchestration.ParseTrace(raw)
		if err != nil {
			return "", err
		}
		spans, err := adapter.ExportOTel(events)
		if err != nil {
			return "", err
		}
		for _, span := range spans {
			if err := enc.Encode(span); err != nil {
				return "", err
			}
		}
	}
	return b.String(), nil
}
