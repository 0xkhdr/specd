package verify

import (
	"context"
	"strings"
	"testing"
)

// TestVerifyRunSelectorCouplingRefusesEmptyRun pins spec R2.3: a recorded `go
// test` run whose selector matched no test in every package it reached is
// rewritten to a non-passing exit code, and the selector is named in stderr.
func TestVerifyRunSelectorCouplingRefusesEmptyRun(t *testing.T) {
	// A command that both looks like a `go test -run` selector and prints go's
	// no-tests summary line; the `#` comments the marker out of execution.
	cmd := `printf 'ok  \tpkg\t0.001s [no tests to run]\n' # go test -run TestNope`
	res, err := Run(context.Background(), Options{Command: cmd})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.ExitCode != NoTestsExitCode {
		t.Fatalf("empty run not refused: exit=%d want %d", res.ExitCode, NoTestsExitCode)
	}
	if !strings.Contains(res.Stderr, "TestNope") {
		t.Fatalf("refusal did not name the selector: %q", res.Stderr)
	}
}

// TestVerifyRunSelectorCouplingParsesOutput pins the pure detector: a single
// no-test package is refused (naming the selector), a multi-package command
// stays valid when any package runs, and non-selector / non-go-test commands
// are never refused.
func TestVerifyRunSelectorCouplingParsesOutput(t *testing.T) {
	cases := []struct {
		name      string
		command   string
		output    string
		wantEmpty bool
		wantSel   string
	}{
		{"single_no_tests", "go test . -run TestFoo", "ok  \tpkg/a\t0.001s [no tests to run]\n", true, "TestFoo"},
		{"run_equals_form", "go test . -run=TestFoo", "ok  \tpkg/a\t0.001s [no tests to run]\n", true, "TestFoo"},
		{"multi_one_executed", "go test ./... -run TestFoo", "ok  \tpkg/a\t0.010s\nok  \tpkg/b\t0.001s [no tests to run]\n", false, ""},
		{"not_a_selector", "go test ./...", "ok  \tpkg/a\t0.001s [no tests to run]\n", false, ""},
		{"not_go_test", "make check", "ok  \tpkg/a\t0.001s [no tests to run]\n", false, ""},
		{"no_packages_reported", "go test . -run TestFoo", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel, empty := runSelectorMatchedNothing(tc.command, tc.output)
			if empty != tc.wantEmpty {
				t.Fatalf("empty=%v want %v (%q)", empty, tc.wantEmpty, tc.output)
			}
			if empty && sel != tc.wantSel {
				t.Fatalf("selector=%q want %q", sel, tc.wantSel)
			}
		})
	}
}
