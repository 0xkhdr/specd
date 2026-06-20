package testharness_test

import (
	"fmt"
	"os"
	"testing"

	th "github.com/0xkhdr/specd/internal/testharness"
)

func TestCaptureStderr(t *testing.T) {
	// Act
	got := th.CaptureStderr(t, func() { fmt.Fprint(os.Stderr, "boom") })

	// Assert
	if got != "boom" {
		t.Errorf("CaptureStderr = %q, want %q", got, "boom")
	}
}

func TestCaptureStdout(t *testing.T) {
	got := th.CaptureStdout(t, func() { fmt.Fprint(os.Stdout, "hello") })
	if got != "hello" {
		t.Errorf("CaptureStdout = %q, want %q", got, "hello")
	}
}

func TestCaptureRestoresStream(t *testing.T) {
	// Arrange
	orig := os.Stderr

	// Act
	_ = th.CaptureStderr(t, func() {})

	// Assert: the global is put back so later output is not swallowed.
	if os.Stderr != orig {
		t.Error("CaptureStderr did not restore os.Stderr")
	}
}

func TestCaptureEmptyWhenNothingWritten(t *testing.T) {
	if got := th.CaptureStdout(t, func() {}); got != "" {
		t.Errorf("expected empty capture, got %q", got)
	}
}
