package core

import (
	"strings"

	corescope "github.com/0xkhdr/specd/internal/core/scope"
)

type DerivedDiff struct {
	Baseline string   `json:"baseline"`
	Paths    []string `json:"paths"`
	Digest   string   `json:"digest"`
}

func DeriveDiff(root, baseline string) (DerivedDiff, error) {
	diff, err := corescope.Derive(root, baseline)
	if err != nil {
		return DerivedDiff{}, err
	}
	paths := diff.Paths[:0]
	for _, path := range diff.Paths {
		if path == ".specd" || strings.HasPrefix(path, ".specd/") {
			continue
		}
		paths = append(paths, path)
	}
	return DerivedDiff{Baseline: baseline, Paths: paths, Digest: Digest([]byte(strings.Join(paths, "\n")))}, nil
}
