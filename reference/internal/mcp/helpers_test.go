package mcp

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// helpers_test.go is the single home for test helpers that need unexported
// package types (toolDef). Shared by every _test.go in package mcp; do not
// re-declare these per file.

// td builds a minimal toolDef carrying just the name the tool filters key on.
func td(name string) toolDef { return toolDef{Name: name} }

// names projects a toolDef slice to its ordered names for slice comparisons.
func names(tools []toolDef) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Name
	}
	return out
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote, so
// tests can assert R4/diagnostic lines the filters emit. Package mcp (internal)
// cannot import testharness without an import cycle (testharness → cmd → mcp),
// so this mirrors testharness.CaptureStderr locally; external mcp_test files use
// the shared helper. See HELPERS.md.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stderr = orig
	return <-done
}
