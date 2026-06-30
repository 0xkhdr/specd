//go:build windows

package worker

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/0xkhdr/specd/internal/obs"
)

const windowsUnsupportedMessage = "orchestration requires a POSIX shell (sh); not supported on Windows — run under WSL"

// ShellRunner fails fast on Windows. specd keeps the rest of the spec workflow
// portable, but Brain/Pinky worker orchestration depends on POSIX process-group
// semantics for production-safe deadline kills.
type ShellRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (r ShellRunner) Run(_ context.Context, _ Mission) (Result, error) {
	if endSpan := obs.StartSpan("worker.run"); endSpan != nil {
		defer endSpan()
	}

	err := errors.New(windowsUnsupportedMessage)
	return Result{ExitErr: err, Duration: 0 * time.Second}, err
}
