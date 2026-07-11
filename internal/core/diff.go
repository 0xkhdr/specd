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
	return DerivedDiff{Baseline: baseline, Paths: diff.Paths, Digest: Digest([]byte(strings.Join(diff.Paths, "\n")))}, nil
}
