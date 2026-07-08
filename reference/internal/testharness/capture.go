package testharness

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// CaptureStderr runs fn with os.Stderr redirected to a pipe and returns
// everything fn wrote to it. It is the shared replacement for the per-file
// captureStderr helpers, so tests that assert diagnostic / R4 lines emitted to
// stderr do not each re-implement the redirect dance.
//
// The original os.Stderr is always restored before CaptureStderr returns.
func CaptureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stderr, fn)
}

// CaptureStdout is the os.Stdout counterpart of CaptureStderr.
func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stdout, fn)
}

// capture redirects *target to a pipe for the duration of fn and returns what
// was written. target is restored even if fn panics.
func captureStream(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	orig := *target
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("CaptureStderr: os.Pipe: %v", err)
	}
	*target = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	defer func() {
		*target = orig
	}()
	fn()
	_ = w.Close()
	return <-done
}
