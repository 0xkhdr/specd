package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

type failSecondWriter struct{ calls int }

func (w *failSecondWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == 2 {
		return 0, errors.New("second write failed")
	}
	return len(p), nil
}

func TestDeadlineContextFallbacks(t *testing.T) {
	for _, deadline := range []string{"not-time", time.Now().Add(-time.Second).Format(time.RFC3339Nano)} {
		ctx, cancel := deadlineContext(context.Background(), deadline)
		select {
		case <-ctx.Done():
			t.Fatalf("deadline %q canceled immediately", deadline)
		default:
		}
		cancel()
		if ctx.Err() == nil {
			t.Fatalf("deadline %q did not cancel", deadline)
		}
	}
}

func TestLineWriterWriteError(t *testing.T) {
	w := newLineWriter("[x] ", failingWriter{})
	if n, err := w.Write([]byte("boom\n")); err == nil || n != 0 {
		t.Fatalf("Write = (%d, %v), want write failure", n, err)
	}

	second := &failSecondWriter{}
	w = newLineWriter("[x] ", second)
	if n, err := w.Write([]byte("boom\n")); err == nil || n != 0 {
		t.Fatalf("Write second = (%d, %v), want write failure", n, err)
	}
}

func TestLineWriterFlushEmpty(t *testing.T) {
	w := newLineWriter("[x] ", failingWriter{})
	w.Flush()
}

func TestShellRunnerCommandFailure(t *testing.T) {
	res, err := ShellRunner{}.Run(context.Background(), Mission{
		Command:  "exit 7",
		WorkerID: "fail",
		TaskID:   "Tfail",
		Deadline: time.Now().Add(5 * time.Second).Format(time.RFC3339Nano),
	})
	if err == nil || res.ExitErr == nil || res.TimedOut {
		t.Fatalf("expected non-timeout exit error, res=%+v err=%v", res, err)
	}
}
